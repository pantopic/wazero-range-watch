package main

import (
	"strconv"

	"github.com/pantopic/wazero-range-watch/sdk-go"
)

func main() {
	if err := range_watch.Receive(recv); err != nil {
		panic(err)
	}
}

func recv(id []byte, val uint64) {
	println(strconv.Itoa(int(val)) + `,` + string(id))
}

//export test_emit
func test_emit(val uint32) {
	range_watch.Emit(uint64(val), [][]byte{
		[]byte(`test-100`),
		[]byte(`test-200`),
		[]byte(`test-300`),
	})
}

//export test_create
func test_create(from, to uint32) {
	id := watchID(from, to)
	range_watch.Open(id,
		[]byte(`test-`+strconv.Itoa(int(from))),
		[]byte(`test-`+strconv.Itoa(int(to))),
	)
	range_watch.Start(id)
}

//export test_reserve
func test_reserve(from, to uint32) {
	range_watch.Reserve(watchID(from, to))
}

//export test_open
func test_open(from, to uint32) {
	id := watchID(from, to)
	range_watch.Open(id,
		[]byte(`test-`+strconv.Itoa(int(from))),
		[]byte(`test-`+strconv.Itoa(int(to))),
	)
}

//export test_start
func test_start(from, to uint32) {
	id := watchID(from, to)
	range_watch.Start(id)
}

//export test_emit_2
func test_emit_2(val uint32) {
	range_watch.Emit(uint64(val), [][]byte{
		[]byte(`test-` + strconv.Itoa(int(val))),
	})
}

//export test_stop
func test_stop(from, to uint32) {
	id := watchID(from, to)
	range_watch.Stop(id)
}

func watchID(from, to uint32) []byte {
	// ie. "100-200"
	return []byte(strconv.Itoa(int(from)) + `-` + strconv.Itoa(int(to)))
}

// Fix for lint rule `unusedfunc`
var _ = test_emit
var _ = test_reserve
var _ = test_create
var _ = test_open
var _ = test_start
var _ = test_stop
var _ = test_emit_2
