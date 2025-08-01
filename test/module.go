package main

import (
	"github.com/pantopic/wazero-range-watch/sdk-go"
)

func main() {}

//export test
func test() {
	rangewatch.Emit(12345, [][]byte{
		[]byte(`test-100`),
		[]byte(`test-200`),
		[]byte(`test-300`),
	})
}

// Fix for lint rule `unusedfunc`
var _ = test
