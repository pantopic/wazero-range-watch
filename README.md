# Wazero Range Watch

A [wazero](https://pkg.go.dev/github.com/tetratelabs/wazero) host module, ABI and guest SDK providing [byteinterval](https://github.com/logbn/byteinterval) support for WASI modules.

## Host Module

[![Go Reference](https://godoc.org/github.com/pantopic/wazero-range-watch/host?status.svg)](https://godoc.org/github.com/pantopic/wazero-range-watch/host)
[![Go Report Card](https://goreportcard.com/badge/github.com/pantopic/wazero-range-watch/host)](https://goreportcard.com/report/github.com/pantopic/wazero-range-watch/host)
[![Go Coverage](https://github.com/pantopic/wazero-range-watch/wiki/host/coverage.svg)](https://raw.githack.com/wiki/pantopic/wazero-range-watch/host/coverage.html)

First register the host module with the runtime

```go
import (
    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

    "github.com/pantopic/wazero-range-watch/host"
)

func main() {
    ctx := context.Background()
    r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig())
    wasi_snapshot_preview1.MustInstantiate(ctx, r)

    module := wazero_range_watch.New()
    module.Register(ctx, r)

    // ...
}
```

## Guest SDK (Go)

[![Go Reference](https://godoc.org/github.com/pantopic/wazero-range-watch/range-watch-go?status.svg)](https://godoc.org/github.com/pantopic/wazero-range-watch/range-watch-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/pantopic/wazero-range-watch/range-watch-go)](https://goreportcard.com/report/github.com/pantopic/wazero-range-watch/range-watch-go)

Then you can import the guest SDK into your WASI module to emit range watch events from WASI.

```go
package main

import (
    "github.com/pantopic/wazero-range-watch/range-watch-go"
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
```

This is useful for emitting [watch events](https://etcd.io/docs/v3.3/learning/api/#watch-api) from WASM state machines.

## Roadmap

This project is in alpha. Breaking API changes should be expected until Beta.

- `v0.0.x` - Alpha
  - [ ] Stabilize API
- `v0.x.x` - Beta
  - [ ] Finalize API
  - [ ] Test in production
- `v1.x.x` - General Availability
  - [ ] Proven long term stability in production
