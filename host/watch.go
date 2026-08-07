package wazero_range_watch

import (
	"sync"

	"github.com/logbn/byteinterval"
)

type watch struct {
	sync.RWMutex

	group  *watchGroup
	id     []byte
	intv   *byteinterval.Interval[*watch]
	out    chan watchMsg
	synced bool
}

func (w *watch) send(val uint64) {
	w.RLock()
	defer w.RUnlock()
	w.out <- watchMsg{w.id, val}
}

func (w *watch) sync() {
	w.Lock()
	defer w.Unlock()
	if w.synced {
		return
	}
	out := w.out
	w.out = w.group.out
	close(out)
	w.synced = true
}

type watchMsg struct {
	id  []byte
	val uint64
}
