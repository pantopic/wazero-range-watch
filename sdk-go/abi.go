package range_watch

import (
	"encoding/binary"
	"unsafe"
)

type Notice struct {
	Val uint64
	IDs [][]byte
}

var (
	_bufCap uint32 = 16 << 10 // 16KB
	_bufLen uint32
	_buf           = make([]byte, int(_bufCap))
	_errCap uint32 = 1 << 10 // 1KB
	_errLen uint32
	_val    uint64
	_err    = make([]byte, int(_errCap))
	meta    = make([]uint32, 7)

	recv func(items []Notice)
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

var notices []Notice

//export __range_watch_recv
func __range_watch_recv() {
	notices = notices[:0]
	var i uint32 = 2
	count := binary.BigEndian.Uint16(_buf)
	for range count {
		var n Notice
		n.Val = binary.BigEndian.Uint64(_buf[i:])
		i += 8
		idCount := binary.BigEndian.Uint16(_buf[i:])
		i += 2
		for range idCount {
			idLen := uint32(binary.BigEndian.Uint16(_buf[i:]))
			i += 2
			id := _buf[i : i+idLen]
			i += idLen
			n.IDs = append(n.IDs, id)
		}
		notices = append(notices, n)
	}
	recv(notices)
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

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_queue
func _queue()

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_flush
func _flush()

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_clear
func _clear()

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_group_start
func _group_start()

//go:wasm-module pantopic/wazero-range-watch
//export __range_watch_group_stop
func _group_stop()

// Fix for lint rule `unusedfunc`
var _ = __range_watch
var _ = __range_watch_recv
