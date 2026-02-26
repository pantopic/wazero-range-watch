package wazero_range_watch

import (
	"context"
	"encoding/base64"
	"sync"

	"github.com/logbn/byteinterval"
)

type watchList struct {
	sync.RWMutex

	items map[string]*watch
	tree  *byteinterval.Tree[chan uint64]
}

var watchListPool = sync.Pool{
	New: func() any {
		return &watchList{
			items: make(map[string]*watch),
			tree:  byteinterval.New[chan uint64](),
		}
	},
}

// One watch list per connection
func newWatchList(ctx context.Context) *watchList {
	w := watchListPool.Get().(*watchList)
	go func() {
		<-ctx.Done()
		w.release()
	}()
	return w
}

func (list *watchList) release() {
	if list == nil {
		return
	}
	list.Lock()
	for _, w := range list.items {
		w._close()
	}
	list.Unlock()
	watchListPool.Put(list)
}

// Multiple watches per connection
func (list *watchList) open(ctx context.Context, id, from, to []byte) (w *watch, err error) {
	k := base64.URLEncoding.EncodeToString(id)
	list.Lock()
	defer list.Unlock()
	w, ok := list.items[k]
	if ok {
		return w, ErrWatchExists
	}
	out := make(chan uint64, 1e3)
	w = &watch{
		id:   k,
		out:  out,
		list: list,
		intv: list.tree.Insert(from, to, out),
	}
	w.ready.Add(1)
	w.ctx, w.cancel = context.WithCancel(ctx)
	list.items[k] = w
	return
}

func (list *watchList) find(id []byte) (w *watch, err error) {
	k := base64.URLEncoding.EncodeToString(id)
	list.RLock()
	defer list.RUnlock()
	w, ok := list.items[k]
	if !ok {
		err = ErrWatchNotFound
	}
	return
}

func (w *watch) close() {
	w.list.Lock()
	defer w.list.Unlock()
	w._close()
}

func (w *watch) _close() {
	w.intv.Remove()
	w.cancel()
	w.Wait()
	delete(w.list.items, w.id)
}

type watch struct {
	sync.WaitGroup

	cancel context.CancelFunc
	ctx    context.Context
	id     string
	intv   *byteinterval.Interval[chan uint64]
	list   *watchList
	out    chan uint64
	ready  sync.WaitGroup
	after  uint64
}
