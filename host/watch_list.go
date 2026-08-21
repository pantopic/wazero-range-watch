package wazero_range_watch

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/logbn/byteinterval"
)

type alert struct {
	keys [][]byte
	val  uint64
}

type watchMsg struct {
	w   *watch
	val uint64
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

func newWatchList(ctx context.Context) *watchList {
	list := &watchList{
		alertChan: make(chan []alert, 1e3),
		groupID:   new(atomic.Uint64),
		groups:    make(map[uint64]*watchGroup),
		tree:      byteinterval.New[*watch](),
	}
	go func() {
		var m = make(map[*watchGroup][]watchMsg)
		for {
			select {
			case batch := <-list.alertChan:
				for _, a := range batch {
					for _, w := range list.tree.FindAny(a.keys...) {
						m[w.group] = append(m[w.group], watchMsg{w, a.val})
					}
				}
				for wg, msgs := range m {
					select {
					case wg.out <- msgs:
					default:
						slog.Error(`wait group full`)
						wg.closeAll()
					}
				}
				clear(m)
			case <-ctx.Done():
				for _, wg := range list.groups {
					wg.closeAll()
				}
				return
			}
		}
	}()
	return list
}

func (list *watchList) queue(val uint64, keys [][]byte) {
	a := alert{val: val}
	for _, k := range keys {
		a.keys = append(a.keys, append(make([]byte, 0, len(k)), k...))
	}
	list.alerts = append(list.alerts, a)
}

func (list *watchList) flush() {
	if len(list.alerts) == 0 {
		return
	}
	list.alertChan <- list.alerts
	list.alerts = []alert{}
}

func (list *watchList) clear() {
	list.alerts = list.alerts[:0]
}
