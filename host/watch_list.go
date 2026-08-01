package wazero_range_watch

import (
	"context"
	"encoding/hex"
	"sync"

	"github.com/logbn/byteinterval"
)

type alert struct {
	keys [][]byte
	val  uint64
}

type watchList struct {
	sync.RWMutex

	items      map[string]*watch
	tree       *byteinterval.Tree[chan uint64]
	alerts     []alert
	alertChan  chan []alert
	alertMutex sync.Mutex
}

var watchListPool = sync.Pool{
	New: func() any {
		return &watchList{
			items:     make(map[string]*watch),
			tree:      byteinterval.New[chan uint64](),
			alertChan: make(chan []alert, 1e3),
		}
	},
}

func newWatchList(ctx context.Context) *watchList {
	list := watchListPool.Get().(*watchList)
	go func() {
		for {
			select {
			case batch := <-list.alertChan:
				for _, a := range batch {
					for _, w := range list.tree.FindAny(a.keys...) {
						// TODO: push into alert pool, start alert worker
						w <- a.val
					}
				}
			case <-ctx.Done():
				list.release()
				return
			}
		}
	}()
	return list
}

func (list *watchList) release() {
	if list == nil {
		return
	}
	list.Lock()
	for _, w := range list.items {
		w._close()
	}
	list.clear()
	for range <-list.alertChan {
		// drain alert chan
	}
	list.Unlock()
	watchListPool.Put(list)
}

func (list *watchList) reserve(ctx context.Context, id []byte) (w *watch, err error) {
	k := hex.EncodeToString(id)
	list.Lock()
	defer list.Unlock()
	w, ok := list.items[k]
	if ok {
		return w, ErrWatchExists
	}
	out := make(chan uint64, 1e3)
	w = &watch{
		id:    k,
		out:   out,
		list:  list,
		ready: &sync.WaitGroup{},
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	list.items[k] = w
	return
}

func (list *watchList) open(ctx context.Context, id, from, to []byte) (w *watch, err error) {
	w, err = list.find(id)
	if err == ErrWatchNotFound {
		w, err = list.reserve(ctx, id)
		if err != nil {
			return
		}
	} else if err != nil {
		return
	} else if w.intv != nil {
		err = ErrWatchAlreadyOpen
		return
	}
	w.intv = list.tree.Insert(from, to, w.out)
	w.ready.Add(1)
	return
}

func (list *watchList) find(id []byte) (w *watch, err error) {
	k := hex.EncodeToString(id)
	list.RLock()
	defer list.RUnlock()
	w, ok := list.items[k]
	if !ok {
		err = ErrWatchNotFound
	}
	return
}

func (list *watchList) queue(val uint64, keys [][]byte) {
	a := alert{val: val}
	for _, k := range keys {
		a.keys = append(a.keys, append(make([]byte, 0, len(k)), k...))
	}
	list.alertMutex.Lock()
	defer list.alertMutex.Unlock()
	list.alerts = append(list.alerts, a)
}

func (list *watchList) flush() {
	if len(list.alerts) == 0 {
		return
	}
	list.alertMutex.Lock()
	alerts := list.alerts
	list.alerts = []alert{}
	list.alertMutex.Unlock()
	list.alertChan <- alerts
}

func (list *watchList) clear() {
	list.alertMutex.Lock()
	list.alerts = list.alerts[:0]
	list.alertMutex.Unlock()
}

func (w *watch) close() {
	w.list.Lock()
	defer w.list.Unlock()
	w._close()
}

func (w *watch) _close() {
	w.intv.Remove()
	w.cancel()
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
	ready  *sync.WaitGroup
	after  uint64
}
