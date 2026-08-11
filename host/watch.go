package wazero_range_watch

import (
	"sync"

	"github.com/logbn/byteinterval"
)

type watch struct {
	sync.WaitGroup
	sync.RWMutex

	group  *watchGroup
	id     []byte
	intv   *byteinterval.Interval[*watch]
	key    string
	out    chan watchMsg
	synced bool
}

func (w *watch) send(val uint64) bool {
	w.RLock()
	defer w.RUnlock()
	if w.synced {
		return false
	}
	w.out <- watchMsg{w.id, val}
	return true
}

func (w *watch) sync() {
	w.Lock()
	defer w.Unlock()
	if w.synced {
		return
	}
	w.group.Lock()
	defer w.group.Unlock()
	delete(w.group.unsynced, w.key)
	close(w.out)
	w.Wait()
	w.synced = true
}

func (w *watch) isSynced() bool {
	w.RLock()
	defer w.RUnlock()
	return w.synced
}

type watchMsg struct {
	id  []byte
	val uint64
}
