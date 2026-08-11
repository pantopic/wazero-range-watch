package wazero_range_watch

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/logbn/byteinterval"
)

type alert struct {
	keys [][]byte
	val  uint64
}

type watchList struct {
	sync.RWMutex

	alertChan  chan []alert
	alertMutex sync.Mutex
	alerts     []alert
	groupID    *atomic.Uint64
	groups     map[uint64]*watchGroup
	tree       *byteinterval.Tree[*watch]
}

var watchListPool = sync.Pool{
	New: func() any {
		return &watchList{
			alertChan: make(chan []alert, 1e3),
			groupID:   new(atomic.Uint64),
			groups:    make(map[uint64]*watchGroup),
			tree:      byteinterval.New[*watch](),
		}
	},
}

func newWatchList(ctx context.Context) *watchList {
	list := watchListPool.Get().(*watchList)
	go func() {
		var m = make(map[*watchGroup][]watchMsg)
		for {
			select {
			case batch := <-list.alertChan:
				for _, a := range batch {
					for _, w := range list.tree.FindAny(a.keys...) {
						m[w.group] = append(m[w.group], watchMsg{w.id, a.val})
					}
				}
				for wg, msgs := range m {
					wg.out <- msgs
				}
				clear(m)
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
	defer list.Unlock()
	for _, wg := range list.groups {
		wg.closeAll()
	}
	list.clear()
outer:
	for {
		// drain alert chan
		select {
		case <-list.alertChan:
		default:
			break outer
		}
	}
	watchListPool.Put(list)
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
