package range_watch

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

func Receive(fn receiveFunc) (err error) {
	if recv != nil {
		return ErrWatchReceiveAlreadyRegistered
	}
	recv = fn
	return
}

func Create(id, from, to []byte) error {
	bufLen = 0
	appendKey(id)
	appendKey(from)
	appendKey(to)
	_create()
	return getErr()
}

func Delete(id []byte) error {
	setData(id)
	_delete()
	return getErr()
}
