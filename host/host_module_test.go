package wazero_range_watch

import (
	"bytes"
	"context"
	_ "embed"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/pantopic/wazero-pool"
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

	ctx, err = hostModule.InitContext(ctx, mod)
	if err != nil {
		t.Fatalf(`%v`, err)
	}

	pool, err := wazeropool.New(ctx, r, testwasm, wazeropool.WithModuleConfig(cfg))
	if err != nil {
		panic(err)
	}
	ctx = wazeropool.ContextSet(ctx, pool)

	ctx = ContextCopy(ctx, ctx)

	call := func(cmd string, params ...uint64) {
		if _, err := mod.ExportedFunction(cmd).Call(ctx, params...); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
	}
	read := func() (val int, id string) {
		line, err := out.ReadString('\n')
		if line == "" {
			return -1, ""
		}
		parts := strings.Split(line[:len(line)-1], ",")
		if val, err = strconv.Atoi(parts[0]); err != nil {
			panic(err)
		}
		id = parts[1]
		return
	}
	expect := func(val int, ids ...string) {
		var m = make(map[string]bool)
		for _, id := range ids {
			m[id] = true
		}
		// time sleep required because we're relying on stdout which may return EOF rather than blocking
		// replace stdout with pipe to solve race condition, removing time sleep dependency
		time.Sleep(10 * time.Millisecond)
		for {
			if len(m) == 0 {
				break
			}
			v, id := read()
			if v != val {
				t.Fatalf("Value %d does not match %d for %s", v, val, id)
			}
			_, ok := m[id]
			if !ok {
				t.Fatalf("ID %s incorrect for %d", id, val)
			}
			delete(m, id)
		}
	}
	t.Run("create", func(t *testing.T) {
		call("test_create", 100, 200)
	})
	t.Run("emit", func(t *testing.T) {
		call("test_emit", 1)
		expect(1, "100-200")
		call("test_create", 200, 300)
		call("test_emit", 2)
		expect(2, "100-200", "200-300")
		call("test_create", 500, 600)
		call("test_emit", 3)
		expect(3, "100-200", "200-300")
		call("test_create", 250, 400)
		call("test_emit", 4)
		expect(4, "100-200", "200-300", "250-400")
	})
	t.Run("delete", func(t *testing.T) {
		call("test_stop", 100, 200)
		call("test_emit", 5)
		expect(5, "200-300", "250-400")
	})
	t.Run("reserve", func(t *testing.T) {
		call("test_reserve", 1000, 2000)
	})
	t.Run("open_start", func(t *testing.T) {
		call("test_open", 1000, 2000)
		call("test_emit_2", 1400)
		call("test_emit_2", 1500)
		call("test_emit_2", 1600)
		call("test_emit_2", 1700)
		call("test_start", 1000, 2000, 1500)
		call("test_emit_2", 1800)
		call("test_emit_2", 1900)
		expect(1600, "1000-2000")
		expect(1700, "1000-2000")
		expect(1800, "1000-2000")
		expect(1900, "1000-2000")
	})
	hostModule.Stop()
}
