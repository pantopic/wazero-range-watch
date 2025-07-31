package rangewatch

func Emit(val uint64, keys [][]byte) {
	rev = val
	bufLen = 0
	for _, k := range keys {
		appendKey(k)
	}
	if bufLen > 0 {
		flush()
	}
}
