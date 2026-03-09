package range_watch

import (
	"encoding/binary"
	"unsafe"
)

var (
	_bufCap uint32 = 16 << 10 // 16KB
	_bufLen uint32
	_buf           = make([]byte, int(_bufCap))
	_errCap uint32 = 1 << 10 // 1KB
	_errLen uint32
	_err    = make([]byte, int(_errCap))
	_val    uint64
	meta    = make([]uint32, 7)

	recv func(id []byte, val uint64)
)

//export __range_watch
func __range_watch() (res uint32) {
	for i, p := range []unsafe.Pointer{
		unsafe.Pointer(&_buf[0]),
		unsafe.Pointer(&_bufCap),
		unsafe.Pointer(&_bufLen),
		unsafe.Pointer(&_err[0]),
		unsafe.Pointer(&_errCap),
		unsafe.Pointer(&_errLen),
		unsafe.Pointer(&_val),
	} {
		meta[i] = uint32(uintptr(p))
	}
	return uint32(uintptr(unsafe.Pointer(&meta[0])))
}

//export __range_watch_recv
func __range_watch_recv() {
	recv(_buf[:_bufLen], _val)
}

func setData(b []byte) {
	_bufLen = uint32(len(b))
	copy(_buf[:len(b)], b)
}

func getErr() (e error) {
	if _errLen > 0 {
		e = strErr(string(_err[:_errLen]))
	}
	return
}

func appendKey(k []byte) bool {
	if _bufLen+2+uint32(len(k)) > _bufCap {
		return false
	}
	binary.BigEndian.PutUint16(_buf[_bufLen:], uint16(len(k)))
	_bufLen += 2
	copy(_buf[_bufLen:], k)
	_bufLen += uint32(len(k))
	return true
}

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_flush
func _flush()

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_reserve
func _reserve()

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
