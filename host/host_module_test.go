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
	ctx = hostModule.ContextCopy(ctx, ctx)
	call := func(cmd string, params ...uint64) {
		if _, err := mod.ExportedFunction(cmd).Call(ctx, params...); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
	}
	read := func() (val int, ids []string) {
		line, err := out.ReadString('\n')
		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}
		parts := strings.Split(line[:len(line)-1], " ")
		if len(parts) < 2 {
			t.Fatalf("Invalid output line: %s", line)
		}
		val, err = strconv.Atoi(parts[0])
		if err != nil {
			panic(err)
		}
		for _, s := range parts[1:] {
			if s == "{" || s == "}" {
				continue
			}
			s = strings.Trim(s, ",")
			ids = append(ids, s)
		}
		return
	}
	type result struct {
		val int
		ids []string
	}
	expect := func(results []result) {
		// time sleep required because we're relying on stdout which may return EOF rather than blocking
		for _, r := range results {
			var vm = map[string]bool{}
			// replace stdout with pipe to solve race condition, removing time sleep dependency
			time.Sleep(20 * time.Millisecond)
			for len(vm) < len(r.ids) {
				val, found := read()
				if val == 0 {
					break
				}
				if val != r.val {
					t.Errorf("Expected value %d, got %d", r.val, val)
				}
				for _, id := range found {
					vm[id] = true
				}
			}
			for _, id := range r.ids {
				if _, ok := vm[id]; !ok {
					t.Errorf("ID %s missing\n%s", id, string(debug.Stack()))
				}
			}
		}
	}
	t.Run("group_start", func(t *testing.T) {
		call("test_group_start")
	})
	t.Run("create", func(t *testing.T) {
		call("test_create", 100, 200)
	})
	t.Run("emit", func(t *testing.T) {
		call("test_emit", 1)
		expect([]result{{1, []string{"100-200"}}})
		call("test_create", 200, 300)
		call("test_emit", 2)
		expect([]result{{2, []string{"100-200", "200-300"}}})
		call("test_create", 500, 600)
		call("test_emit", 3)
		expect([]result{{3, []string{"100-200", "200-300"}}})
		call("test_create", 250, 400)
		call("test_emit", 4)
		expect([]result{{4, []string{"100-200", "200-300", "250-400"}}})
	})
	ctx = hostModule.ContextCopy(ctx, ctx)
	t.Run("delete", func(t *testing.T) {
		call("test_stop", 100, 200)
		call("test_emit", 5)
		expect([]result{{5, []string{"200-300", "250-400"}}})
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
		expect([]result{
			{1500, []string{"1000-2000"}},
			{1600, []string{"1000-2000"}},
			{1700, []string{"1000-2000"}},
			{1800, []string{"1000-2000"}},
		})
		expect([]result{{0, nil}})
		call("test_emit_2", 1900)
		expect([]result{{1900, []string{"1000-2000"}}})
	})
	t.Run("group_stop", func(t *testing.T) {
		call("test_group_stop")
	})
	hostModule.Stop()
}
