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
	range_watch.Create(id(from, to),
		[]byte(`test-`+strconv.Itoa(int(from))),
		[]byte(`test-`+strconv.Itoa(int(to))),
	)
}

//export test_delete
func test_delete(from, to uint32) {
	range_watch.Delete(id(from, to))
}

func id(from, to uint32) []byte {
	// ie. "100-200"
	return []byte(strconv.Itoa(int(from)) + `-` + strconv.Itoa(int(to)))
}

// Fix for lint rule `unusedfunc`
var _ = test_emit
var _ = test_create
var _ = test_delete
