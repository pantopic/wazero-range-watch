package rangewatch

func Emit(keys [][]byte) {
	bufLen = 0
	buf = buf[:0]
	for _, k := range keys {
		appendKey(k)
	}
	if bufLen > 0 {
		flush()
	}
}
