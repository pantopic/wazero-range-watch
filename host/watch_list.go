package wazero_range_watch

import (
	"context"
	"encoding/base64"
	"sync"

	"github.com/logbn/byteinterval"
)

type watchList struct {
	sync.Mutex

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
	defer list.Unlock()
	for _, w := range list.items {
		w._close()
	}
	defer watchListPool.Put(list)
}

func (list *watchList) new(ctx context.Context, id, from, to []byte) (w *watch, err error) {
	list.Lock()
	defer list.Unlock()
	w, ok := list.items[base64.URLEncoding.EncodeToString(id)]
	if ok {
		return w, ErrWatchExists
	}
	out := make(chan uint64)
	w = &watch{
		id:   id,
		out:  out,
		list: list,
		intv: list.tree.Insert(from, to, out),
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	list.items[base64.URLEncoding.EncodeToString(id)] = w
	return
}

func (list *watchList) find(id []byte) (w *watch, err error) {
	w, ok := list.items[base64.URLEncoding.EncodeToString(id)]
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
	delete(w.list.items, base64.URLEncoding.EncodeToString(w.id))
}

type watch struct {
	sync.WaitGroup

	cancel context.CancelFunc
	ctx    context.Context
	id     []byte
	intv   *byteinterval.Interval[chan uint64]
	list   *watchList
	out    chan uint64
}
