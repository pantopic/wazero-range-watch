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
	active  bool
	watches map[string]*watch
}

func newWatchGroup(ctx context.Context) (wg *watchGroup) {
	list := getWatchList(ctx)
	wg = &watchGroup{
		list: list,
	}
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
		id:    append([]byte{}, id...),
		out:   make(chan watchMsg, 1e3),
	}
	wg.watches[k] = w
	return
}

func (wg *watchGroup) open(ctx context.Context, id, from, to []byte) (w *watch, err error) {
	wg.Lock()
	defer wg.Unlock()
	if !wg.active {
		return nil, ErrWatchGroupNotActive
	}
	w, err = wg._reserve(ctx, id)
	if err == ErrWatchExists && w.intv != nil {
		return nil, ErrWatchAlreadyOpen
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
	wg.Lock()
	defer wg.Unlock()
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
	wg.Lock()
	defer wg.Unlock()
	for k, w := range wg.watches {
		w.intv.Remove()
		delete(wg.watches, k)
	}
	wg.list.Lock()
	delete(wg.list.groups, wg.id)
	wg.list.Unlock()
	return
}

func (wg *watchGroup) key(id []byte) string {
	return hex.EncodeToString(append(binary.BigEndian.AppendUint64([]byte{}, wg.id), id...))
}

func (wg *watchGroup) start(ctx context.Context) {
	wg.Lock()
	defer wg.Unlock()
	wg.id = wg.list.groupID.Add(1)
	wg.out = make(chan watchMsg, 1e3)
	wg.watches = make(map[string]*watch)
	wg.active = true
	var i int = 0
	var limit int = 1e4
	var batches = make(chan map[string][]watchMsg)
	meta := get[*meta](ctx, ctxKeyMeta)
	wg.list.Lock()
	wg.list.groups[wg.id] = wg
	wg.list.Unlock()
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
				t.Reset(50 * time.Millisecond)
			drain:
				for {
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
			case <-ctx.Done():
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
						if _, err := mod.ExportedFunction("__range_watch_recv").Call(ctx); err != nil {
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
