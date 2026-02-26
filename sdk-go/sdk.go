package range_watch

// Receive registers a callback to receive watch notices
func Receive(fn func(id []byte, val uint64)) (err error) {
	if recv != nil {
		return ErrWatchReceiveAlreadyRegistered
	}
	recv = fn
	return
}

// Emit broadcasts a value to watchers of a set of keys
func Emit(v uint64, keys [][]byte) {
	val = v
	bufLen = 0
	for _, k := range keys {
		if !appendKey(k) {
			_flush()
			bufLen = 0
			appendKey(k)
		}
	}
	if bufLen > 0 {
		_flush()
	}
}

// Open starts receiving values into a buffer
func Open(id, from, to []byte) error {
	bufLen = 0
	appendKey(id)
	appendKey(from)
	appendKey(to)
	_open()
	return getErr()
}

// Start begins the processing of values in the buffer after supplied minimum
func Start(id []byte, after uint64) error {
	setData(id)
	setVal(after)
	_start()
	return getErr()
}

// Stop closes the range watch
func Stop(id []byte) error {
	setData(id)
	_stop()
	return getErr()
}
