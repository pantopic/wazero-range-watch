package wazero_range_watch

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"sync"

	"github.com/tetratelabs/wazero/api"

	"github.com/pantopic/wazero-pool"
)

type watchGroup struct {
	sync.WaitGroup
	sync.RWMutex

	ctx      context.Context
	id       uint64
	list     *watchList
	out      chan []watchMsg
	active   bool
	watches  map[string]*watch
	unsynced map[string]*watch
}

func newWatchGroup(ctx context.Context) (wg *watchGroup) {
	wg = &watchGroup{}
	return
}

func (wg *watchGroup) reserve(ctx context.Context, id []byte) (w *watch, err error) {
	wg.Lock()
	defer wg.Unlock()
	return wg._reserve(ctx, id, false)
}

func (wg *watchGroup) _reserve(ctx context.Context, id []byte, synced bool) (w *watch, err error) {
	var k = wg.key(id)
	var ok bool
	if w, ok = wg.watches[k]; ok {
		return w, ErrWatchExists
	}
	w = &watch{
		group: wg,
		id:    append([]byte{}, id...),
		key:   k,
		out:   make(chan watchMsg, 1<<10),
	}
	wg.watches[k] = w
	if !synced {
		wg.unsynced[k] = w
	}
	return
}

func (wg *watchGroup) open(ctx context.Context, id, from, to []byte, synced bool) (w *watch, err error) {
	wg.Lock()
	defer wg.Unlock()
	if !wg.active {
		return nil, ErrWatchGroupNotActive
	}
	w, err = wg._reserve(ctx, id, synced)
	if err == ErrWatchExists && w.intv != nil {
		return nil, ErrWatchAlreadyOpen
	}
	w.intv = wg.list.tree.Insert(from, to, w)
	if synced {
		w.synced = true
		delete(wg.unsynced, w.key)
	}
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
	close(wg.out)
	return
}

func (wg *watchGroup) key(id []byte) string {
	return hex.EncodeToString(append(binary.BigEndian.AppendUint64([]byte{}, wg.id), id...))
}

func (wg *watchGroup) start(ctx context.Context) {
	wg.Lock()
	defer wg.Unlock()
	wg.list = getWatchList(ctx)
	wg.id = wg.list.groupID.Add(1)
	wg.out = make(chan []watchMsg, 10<<10)
	wg.watches = make(map[string]*watch)
	wg.unsynced = make(map[string]*watch)
	wg.active = true
	wg.list.Lock()
	wg.list.groups[wg.id] = wg
	wg.list.Unlock()
	var meta = get[*meta](ctx, ctxKeyMeta)
	var bufCap uint32
	wazeropool.FromContext(ctx).Run(func(mod api.Module) {
		bufCap = readUint32(mod, meta.ptrBufCap)
	})
	var bufPool = sync.Pool{
		New: func() any { return make([]byte, 0, bufCap) },
	}
	var batches = make(chan []byte)
	wg.Go(func() {
		var val uint64
		for {
			val = 0
			buf := bufPool.Get().([]byte)[:0]
			select {
			case msgs, ok := <-wg.out:
				if !ok {
					close(batches)
					return
				}
				wg.RLock()
			drain:
				for _, msg := range msgs {
					if len(wg.unsynced) > 0 {
						k := wg.key(msg.id)
						if wg.unsynced[k] != nil {
							if wg.unsynced[k].send(msg.val) {
								continue
							}
						}
					}
					if len(buf)+len(msg.id)+8 >= cap(buf) {
						batches <- buf
						buf = bufPool.Get().([]byte)[:0]
						val = 0
					}
					if msg.val != val {
						val = msg.val
						if len(buf) > 0 {
							buf = binary.BigEndian.AppendUint16(buf, 0)
						}
						buf = binary.BigEndian.AppendUint64(buf, val)
					}
					buf = binary.BigEndian.AppendUint16(buf, uint16(len(msg.id)))
					buf = append(buf, msg.id...)
				}
				select {
				case msgs = <-wg.out:
					goto drain
				default:
				}
				wg.RUnlock()
				batches <- buf
			case <-ctx.Done():
				close(batches)
				return
			}
		}
	})
	wg.Go(func() {
		for b := range batches {
			wazeropool.FromContext(ctx).Run(func(mod api.Module) {
				setData(mod, meta, b)
				setErr(mod, meta, nil)
				if _, err := mod.ExportedFunction("__range_watch_recv").Call(ctx); err != nil {
					slog.Error("Error __range_watch_recv A", "err", err.Error())
					return
				}
				if err := getErr(mod, meta); err != nil {
					slog.Error("Error __range_watch_recv B", "err", err.Error())
					wg.closeAll()
					return
				}
			})
			bufPool.Put(b)
		}
	})
}
