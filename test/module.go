package main

import (
	"strconv"

	"github.com/pantopic/wazero-range-watch/sdk-go"
)

func init() {
	range_watch.Receive(recv)
}

func main() {}

func recv(notices []range_watch.Notice) {
	for _, notice := range notices {
		s := strconv.Itoa(int(notice.Val))
		for _, id := range notice.IDs {
			s += " " + string(id)
		}
		println(s)
	}
}

//export test_group_start
func test_group_start() {
	range_watch.GroupStart()
}

//export test_group_stop
func test_group_stop() {
	range_watch.GroupStop()
}

//export test_emit
func test_emit(val uint32) {
	range_watch.Queue(uint64(val), [][]byte{
		[]byte(`test-100`),
		[]byte(`test-200`),
		[]byte(`test-300`),
	})
	range_watch.Flush()
}

//export test_queue
func test_queue(val uint32) {
	range_watch.Queue(uint64(val), [][]byte{
		[]byte(`test-100`),
		[]byte(`test-200`),
		[]byte(`test-300`),
	})
}

//export test_flush
func test_flush() {
	range_watch.Flush()
}

//export test_clear
func test_clear() {
	range_watch.Clear()
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
	range_watch.Queue(uint64(val), [][]byte{
		[]byte(`test-` + strconv.Itoa(int(val))),
	})
	range_watch.Flush()
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
var _ = test_clear
var _ = test_create
var _ = test_emit
var _ = test_emit_2
var _ = test_flush
var _ = test_group_start
var _ = test_group_stop
var _ = test_open
var _ = test_queue
var _ = test_reserve
var _ = test_start
var _ = test_stop
