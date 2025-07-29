package rangewatch

import (
	"encoding/binary"
	"unsafe"
)

var (
	rev    uint64
	bufCap uint32 = 16 << 10 // 16KB
	bufLen uint32
	buf    = make([]byte, int(bufCap))
	meta   = make([]uint32, 3)
)

//export __rangewatch
func __rangewatch() (res uint32) {
	meta[0] = uint32(uintptr(unsafe.Pointer(&rev)))
	meta[1] = uint32(uintptr(unsafe.Pointer(&bufCap)))
	meta[2] = uint32(uintptr(unsafe.Pointer(&bufLen)))
	meta[3] = uint32(uintptr(unsafe.Pointer(&buf[0])))
	return uint32(uintptr(unsafe.Pointer(&meta[0])))
}

func appendKey(k []byte) {
	if bufLen+uint32(len(k))+2 > bufCap {
		flush()
		bufLen = 0
		buf = buf[:0]
	}
	binary.BigEndian.PutUint16(buf[:bufLen], uint16(len(k)))
	bufLen += 2
	copy(buf[bufLen:], k)
	bufLen += uint32(len(k))
}

//go:wasm-module rangewatch
//export Flush
func flush()

// Fix for lint rule `unusedfunc`
var _ = __rangewatch
