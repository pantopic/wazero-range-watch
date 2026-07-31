package wazero_range_watch

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/pantopic/wazero-pool"
)

//go:embed test\.wasm
var testwasmGo []byte

//go:embed test-zig\.wasm
var testwasmZig []byte

func TestModule(t *testing.T) {
	for _, tc := range []struct {
		name string
		wasm []byte
	}{
		{`go`, testwasmGo},
		{`zig`, testwasmZig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testModule(t, tc.wasm)
		})
	}
}

func testModule(t *testing.T, testwasm []byte) {
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

	ctx = hostModule.ContextCopy(ctx, ctx)

	call := func(cmd string, params ...uint64) {
		if _, err := mod.ExportedFunction(cmd).Call(ctx, params...); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
	}
	read := func() (id string, vals []int) {
		line, err := out.ReadString('\n')
		if err == io.EOF {
			return "", []int{-1}
		}
		if err != nil {
			panic(err)
		}
		parts := strings.Split(line[:len(line)-1], " ")
		if len(parts) < 2 {
			t.Fatalf("Invalid output line: %s", line)
		}
		for _, s := range parts[1:] {
			if s == "{" || s == "}" {
				continue
			}
			s = strings.Trim(s, ",")
			val, err := strconv.Atoi(s)
			if err != nil {
				panic(err)
			}
			vals = append(vals, val)
		}
		id = parts[0]
		return
	}
	expect := func(vals []int, ids ...string) {
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
			id, found := read()
			var vm = map[int]bool{}
			for _, v := range found {
				vm[v] = true
			}
			for _, v := range vals {
				if _, ok := vm[v]; !ok {
					t.Errorf("Value %d missing for %s (%v)\n%s", v, id, vals, string(debug.Stack()))
				}
			}
			if len(id) > 0 {
				_, ok := m[id]
				if !ok {
					t.Errorf("ID %s incorrect for %v", id, vals)
					break
				}
				delete(m, id)
			} else {
				break
			}
		}
	}
	t.Run("create", func(t *testing.T) {
		call("test_create", 100, 200)
	})
	t.Run("emit", func(t *testing.T) {
		call("test_emit", 1)
		expect([]int{1}, "100-200")
		call("test_create", 200, 300)
		call("test_emit", 2)
		expect([]int{2}, "100-200", "200-300")
		call("test_create", 500, 600)
		call("test_emit", 3)
		expect([]int{3}, "100-200", "200-300")
		call("test_create", 250, 400)
		call("test_emit", 4)
		expect([]int{4}, "100-200", "200-300", "250-400")
	})
	ctx = hostModule.ContextCopy(ctx, ctx)
	t.Run("delete", func(t *testing.T) {
		call("test_stop", 100, 200)
		call("test_emit", 5)
		expect([]int{5}, "200-300", "250-400")
	})
	t.Run("reserve", func(t *testing.T) {
		call("test_reserve", 1000, 2000)
	})
	ctx = hostModule.ContextCopy(ctx, ctx)
	t.Run("open_start", func(t *testing.T) {
		call("test_open", 1000, 2000)
		call("test_emit_2", 1500)
		call("test_emit_2", 1600)
		call("test_emit_2", 1700)
		call("test_start", 1000, 2000)
		call("test_emit_2", 1800)
		expect([]int{1500, 1600, 1700, 1800}, "1000-2000")
		expect([]int{-1}, "1000-2000")
		call("test_emit_2", 1900)
		expect([]int{1900}, "1000-2000")
	})
	hostModule.Stop()
}
