package wazero_range_watch

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tetratelabs/wazero/api"

	"github.com/pantopic/wazero-pool"
)

type watchGroup struct {
	sync.WaitGroup
	sync.RWMutex

	ctx     context.Context
	id      uint64
	list    *watchList
	out     chan watchMsg
	running bool
	watches map[string]*watch
}

func newWatchGroup(ctx context.Context) (w *watchGroup) {
	list := getWatchList(ctx)
	id := list.groupID.Add(1)
	w = &watchGroup{
		ctx:     ctx,
		id:      id,
		list:    list,
		out:     make(chan watchMsg, 1e3),
		watches: make(map[string]*watch),
	}
	list.Lock()
	list.groups[id] = w
	list.Unlock()
	return
}

func (wg *watchGroup) reserve(ctx context.Context, id []byte) (w *watch, err error) {
	wg.Lock()
	defer wg.Unlock()
	return wg._reserve(ctx, id)
}

func (wg *watchGroup) _reserve(ctx context.Context, id []byte) (w *watch, err error) {
	var k = wg.key(id)
	var ok bool
	if w, ok = wg.watches[k]; ok {
		return w, ErrWatchExists
	}
	w = &watch{
		group: wg,
		ctx:   ctx,
		id:    append([]byte{}, id...),
		out:   make(chan watchMsg, 1e3),
	}
	wg.watches[k] = w
	return
}

func (wg *watchGroup) open(ctx context.Context, id, from, to []byte) (w *watch, err error) {
	wg.Lock()
	defer wg.Unlock()
	w, err = wg._reserve(ctx, id)
	if err == ErrWatchExists && w.intv != nil {
		return nil, ErrWatchAlreadyOpen
	}
	if !wg.running {
		wg.running = true
		wg.start(ctx)
	}
	w.intv = wg.list.tree.Insert(from, to, w)
	return
}

func (wg *watchGroup) find(id []byte) *watch {
	wg.RLock()
	defer wg.RUnlock()
	return wg.watches[wg.key(id)]
}

func (wg *watchGroup) close(id []byte) (err error) {
	wg.list.Lock()
	defer wg.list.Unlock()
	k := wg.key(id)
	w, ok := wg.watches[k]
	if !ok {
		return ErrWatchNotFound
	}
	w.intv.Remove()
	delete(wg.watches, k)
	return
}

func (wg *watchGroup) closeAll() (err error) {
	for k, w := range wg.watches {
		w.intv.Remove()
		delete(wg.watches, k)
	}
	return
}

func (wg *watchGroup) key(id []byte) string {
	return hex.EncodeToString(append(binary.BigEndian.AppendUint64([]byte{}, wg.id), id...))
}

func (wg *watchGroup) start(ctx context.Context) {
	var i int = 0
	var limit int = 1e5
	var batches = make(chan map[string][]watchMsg)
	meta := get[*meta](ctx, ctxKeyMeta)
	wg.Go(func() {
		var valsCap uint32
		wazeropool.FromContext(ctx).Run(func(mod api.Module) {
			valsCap = readUint32(mod, meta.ptrValsCap)
		})
		t := time.NewTicker(50 * time.Millisecond)
		defer t.Stop()
		for {
			i = 0
			m := make(map[string][]watchMsg)
			select {
			case msg := <-wg.out:
			drain:
				for {
					t.Reset(50 * time.Millisecond)
					i++
					k := fmt.Sprintf(`%x`, msg.id)
					if _, ok := m[k]; !ok {
						m[k] = append([]watchMsg{}, msg)
					} else {
						m[k] = append(m[k], msg)
					}
					if len(m[k]) >= int(valsCap) {
						batches <- m
						break drain
					}
					select {
					case msg = <-wg.out:
					case <-t.C:
						batches <- m
						break drain
					}
					if i >= limit {
						batches <- m
						break drain
					}
				}
			case <-wg.ctx.Done():
				return
			}
		}
	})
	wg.Go(func() {
		for m := range batches {
			var g sync.WaitGroup
			for _, msgs := range m {
				g.Go(func() {
					wazeropool.FromContext(ctx).Run(func(mod api.Module) {
						setData(mod, meta, msgs[0].id)
						for i, m := range msgs {
							setVals(mod, meta, uint32(i), m.val)
						}
						setValsLen(mod, meta, uint32(len(msgs)))
						setErr(mod, meta, nil)
						if _, err := mod.ExportedFunction("__range_watch_recv").Call(msgs[0].ctx); err != nil {
							slog.Error("Error __range_watch_recv A", "watchID", msgs[0].id, "err", err.Error())
							return
						}
						if err := getErr(mod, meta); err != nil {
							slog.Error("Error __range_watch_recv B", "watchID", msgs[0].id, "err", err.Error())
							wg.close(msgs[0].id)
							return
						}
					})
				})
			}
			g.Wait()
		}
	})
}
