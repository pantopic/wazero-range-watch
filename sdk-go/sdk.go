package range_watch

// Receive registers a callback to receive watch notices
func Receive(fn func(items []Notice)) (err error) {
	if recv != nil {
		return ErrWatchReceiveAlreadyRegistered
	}
	recv = fn
	return
}

// Queue queues a value to be broadcast to watchers of a set of keys
func Queue(v uint64, keys [][]byte) {
	_val = v
	_bufLen = 0
	for _, k := range keys {
		if !appendKey(k) {
			_queue()
			_bufLen = 0
			appendKey(k)
		}
	}
	if _bufLen > 0 {
		_queue()
	}
}

// Flush broadcasts queued alerts to watchers sequentially and asynchronously
func Flush() error {
	_flush()
	return getErr()
}

// Clear empties the broadcast queue
func Clear() error {
	_clear()
	return getErr()
}

// Reserve locks the range watch id for future opening
func Reserve(id []byte) error {
	setData(id)
	_reserve()
	return getErr()
}

// Open starts receiving values into a buffer
func Open(id, from, to []byte) error {
	_bufLen = 0
	appendKey(id)
	appendKey(from)
	appendKey(to)
	_open()
	return getErr()
}

// Start begins the processing of values in the buffer
func Start(id []byte) error {
	setData(id)
	_start()
	return getErr()
}

// Stop closes the range watch
func Stop(id []byte) error {
	setData(id)
	_stop()
	return getErr()
}

// GroupStart starts the watch group
func GroupStart() {
	_group_start()
}

// GroupStop stops the watch group
func GroupStop() {
	_group_stop()
}
