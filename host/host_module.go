package wazero_range_watch

import (
	"context"
	"encoding/binary"
	"log"
	"sync"

	"github.com/logbn/byteinterval"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Name is the name of this host module.
const Name = "pantopic/wazero-range-watch"

var (
	DefaultCtxKeyMeta = `wazero_range_watch_meta`
	DefaultCtxKey     = `wazero_range_watch`
)

type meta struct {
	ptrRev    uint32
	ptrBufCap uint32
	ptrBufLen uint32
	ptrBuf    uint32
}

type hostModule struct {
	sync.RWMutex

	module     api.Module
	ctxKeyMeta string
	ctxKey     string
}

type Option func(*hostModule)

func New(opts ...Option) *hostModule {
	p := &hostModule{
		ctxKeyMeta: DefaultCtxKeyMeta,
		ctxKey:     DefaultCtxKey,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *hostModule) Name() string {
	return Name
}
func (p *hostModule) Stop() {}

// Register instantiates the host module, making it available to all module instances in this runtime
func (p *hostModule) Register(ctx context.Context, r wazero.Runtime) (err error) {
	builder := r.NewHostModuleBuilder(Name)
	register := func(name string, fn func(ctx context.Context, m api.Module, stack []uint64)) {
		builder = builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(fn), nil, nil).Export(name)
	}
	for name, fn := range map[string]any{
		"Flush": func(ctx context.Context, watches *byteinterval.Tree[chan uint64], keys [][]byte, rev uint64) {
			for _, w := range watches.FindAny(keys...) {
				w <- rev
			}
		},
	} {
		switch fn := fn.(type) {
		case func(ctx context.Context, watches *byteinterval.Tree[chan uint64], keys [][]byte, rev uint64):
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				meta := get[*meta](ctx, p.ctxKeyMeta)
				fn(ctx, p.watches(ctx), keys(m, meta), rev(m, meta))
			})
		default:
			log.Panicf("Method signature implementation missing: %#v", fn)
		}
	}
	p.module, err = builder.Instantiate(ctx)
	return
}

// InitContext retrieves the meta page from the wasm module
func (p *hostModule) InitContext(ctx context.Context, m api.Module) (context.Context, *meta, error) {
	stack, err := m.ExportedFunction(`__range_watch`).Call(ctx)
	if err != nil {
		return ctx, nil, err
	}
	meta := &meta{}
	ptr := uint32(stack[0])
	for i, v := range []*uint32{
		&meta.ptrRev,
		&meta.ptrBufCap,
		&meta.ptrBufLen,
		&meta.ptrBuf,
	} {
		*v = readUint32(m, ptr+uint32(4*i))
	}
	return context.WithValue(ctx, p.ctxKeyMeta, meta), meta, nil
}

func (p *hostModule) watches(ctx context.Context) *byteinterval.Tree[chan uint64] {
	return get[*byteinterval.Tree[chan uint64]](ctx, p.ctxKey)
}

func get[T any](ctx context.Context, key string) T {
	v := ctx.Value(key)
	if v == nil {
		log.Panicf("Context item missing %s", key)
	}
	return v.(T)
}

func rev(m api.Module, meta *meta) uint64 {
	return readUint64(m, meta.ptrRev)
}

func readUint32(m api.Module, ptr uint32) (val uint32) {
	val, ok := m.Memory().ReadUint32Le(ptr)
	if !ok {
		log.Panicf("Memory.Read(%d) out of range", ptr)
	}
	return
}

func keys(m api.Module, meta *meta) (keys [][]byte) {
	buf := read(m, meta.ptrBuf, meta.ptrBufLen, meta.ptrBufCap)
	for len(buf) > 0 {
		keyLen := binary.BigEndian.Uint16(buf)
		buf = buf[2:]
		if len(buf) < int(keyLen) {
			log.Panicf("Buffer too short for key length %d", keyLen)
		}
		key := buf[:keyLen]
		buf = buf[keyLen:]
		keys = append(keys, key)
	}
	return
}

func read(m api.Module, ptrData, ptrLen, ptrMax uint32) (buf []byte) {
	buf, ok := m.Memory().Read(ptrData, readUint32(m, ptrMax))
	if !ok {
		log.Panicf("Memory.Read(%d, %d) out of range", ptrData, ptrLen)
	}
	return buf[:readUint32(m, ptrLen)]
}

func readUint64(m api.Module, ptr uint32) (val uint64) {
	val, ok := m.Memory().ReadUint64Le(ptr)
	if !ok {
		log.Panicf("Memory.Read(%d) out of range", ptr)
	}
	return
}

func writeUint32(m api.Module, ptr uint32, val uint32) {
	if ok := m.Memory().WriteUint32Le(ptr, val); !ok {
		log.Panicf("Memory.Read(%d) out of range", ptr)
	}
}
