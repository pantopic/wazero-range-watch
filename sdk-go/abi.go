package range_watch

import (
	"encoding/binary"
	"unsafe"
)

var (
	bufCap uint32 = 16 << 10 // 16KB
	bufLen uint32
	buf           = make([]byte, int(bufCap))
	errCap uint32 = 1 << 10 // 1KB
	errLen uint32
	err    = make([]byte, int(errCap))
	val    uint64
	meta   = make([]uint32, 7)

	recv func(id []byte, val uint64)
)

//export __range_watch
func __range_watch() (res uint32) {
	for i, p := range []unsafe.Pointer{
		unsafe.Pointer(&buf[0]),
		unsafe.Pointer(&bufCap),
		unsafe.Pointer(&bufLen),
		unsafe.Pointer(&err[0]),
		unsafe.Pointer(&errCap),
		unsafe.Pointer(&errLen),
		unsafe.Pointer(&val),
	} {
		meta[i] = uint32(uintptr(p))
	}
	return uint32(uintptr(unsafe.Pointer(&meta[0])))
}

//export __range_watch_recv
func __range_watch_recv() {
	recv(getData(), getVal())
}

func getData() []byte {
	return buf[:bufLen]
}

func setData(b []byte) {
	bufLen = uint32(len(b))
	copy(buf[:len(b)], b)
}

func getVal() uint64 {
	return val
}

func setVal(v uint64) {
	val = v
}

func getErr() (e error) {
	if errLen > 0 {
		e = strErr(string(err[:errLen]))
	}
	return
}

func appendKey(k []byte) bool {
	if bufLen+2+uint32(len(k)) > bufCap {
		return false
	}
	binary.BigEndian.PutUint16(buf[bufLen:], uint16(len(k)))
	bufLen += 2
	copy(buf[bufLen:], k)
	bufLen += uint32(len(k))
	return true
}

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_flush
func _flush()

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_open
func _open()

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_start
func _start()

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_stop
func _stop()

// Fix for lint rule `unusedfunc`
var _ = __range_watch
var _ = __range_watch_recv
