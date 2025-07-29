package wazero_range_watch

import (
	"bytes"
	"context"
	_ "embed"
	"testing"

	"github.com/logbn/byteinterval"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed test\.wasm
var testwasm []byte

func TestModule(t *testing.T) {
	var (
		ctx = context.Background()
		out = &bytes.Buffer{}
	)
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithMemoryLimitPages(64)) // 4 MB
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	hostModule := New()
	hostModule.Register(ctx, r)

	compiled, err := r.CompileModule(ctx, testwasm)
	if err != nil {
		panic(err)
	}
	cfg := wazero.NewModuleConfig().WithStdout(out)
	mod, err := r.InstantiateModule(ctx, compiled, cfg)
	if err != nil {
		t.Errorf(`%v`, err)
		return
	}

	var meta *meta
	ctx, meta, err = hostModule.InitContext(ctx, mod)
	if err != nil {
		t.Fatalf(`%v`, err)
	}
	if v := readUint32(mod, meta.ptrBufCap); v != 16<<10 {
		t.Fatalf("incorrect buffer cap: %#v %d", meta, v)
	}

	// create byte interval
	watches := byteinterval.New[chan uint64]()
	ctx = context.WithValue(ctx, hostModule.ctxKey, watches)

	call := func(cmd string, params ...uint64) {
		if _, err := mod.ExportedFunction(cmd).Call(ctx, params...); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
	}
	res := make(chan uint64, 10)
	expect := func(n int) {
		var found int
	loop:
		for {
			select {
			case <-res:
				found++
			default:
				break loop
			}
		}
		if found != n {
			t.Fatalf("expected %d, got %d", n, found)
		}

	}
	t.Run("base", func(t *testing.T) {
		i1 := watches.Insert([]byte(`test-100`), []byte(`test-200`), res)
		call("test")
		expect(1)
		watches.Insert([]byte(`test-200`), []byte(`test-300`), res)
		call("test")
		expect(2)
		watches.Insert([]byte(`test-500`), []byte(`test-600`), res)
		call("test")
		expect(2)
		watches.Insert([]byte(`test-250`), []byte(`test-350`), res)
		call("test")
		expect(3)
		i1.Remove()
		call("test")
		expect(2)
	})
	hostModule.Stop()
}
