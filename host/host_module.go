package wazero_range_watch

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"log/slog"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/pantopic/wazero-pool"
)

// Name is the name of this host module.
const Name = "pantopic/wazero-range-watch"

var (
	ctxKeyMeta      = Name + `/meta`
	ctxKeyWatchList = Name + `/watch_list`
)

func ContextCopy(dst, src context.Context) context.Context {
	dst = context.WithValue(dst, ctxKeyMeta, get[*meta](src, ctxKeyMeta))
	dst = context.WithValue(dst, ctxKeyWatchList, newWatchList(dst))
	return dst
}

type meta struct {
	ptrBuf    uint32
	ptrBufCap uint32
	ptrBufLen uint32
	ptrErr    uint32
	ptrErrCap uint32
	ptrErrLen uint32
	ptrVal    uint32
}

type hostModule struct {
	sync.RWMutex
	sync.WaitGroup

	module api.Module
}

type Option func(*hostModule)

func New(opts ...Option) *hostModule {
	p := &hostModule{}
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
		"__range_watch_flush": func(ctx context.Context, list *watchList, keys [][]byte, val uint64) {
			for _, w := range list.tree.FindAny(keys...) {
				w <- val
			}
		},
		"__range_watch_create": func(ctx context.Context, list *watchList, id, from, to []byte) {
			watch, err := list.new(ctx, id, from, to)
			if err != nil {
				return
			}
			watch.Go(func() {
				for {
					select {
					case val := <-watch.out:
						meta := get[*meta](ctx, ctxKeyMeta)
						wazeropool.Context(ctx).Run(func(mod api.Module) {
							setData(mod, meta, id)
							setVal(mod, meta, val)
							setErr(mod, meta, nil)
							if _, err = mod.ExportedFunction("__range_watch_recv").Call(ctx); err != nil {
								slog.Error("Error calling watch receive notice", "watchID", id, "err", err.Error())
								return
							}
							if err = getErr(mod, meta); err != nil {
								slog.Error("Error receiving watch notice", "watchID", id, "err", err.Error())
								watch.close()
								return
							}
						})
					case <-watch.ctx.Done():
						return
					}
				}
			})
		},
		"__range_watch_delete": func(ctx context.Context, list *watchList, id []byte) (err error) {
			watch, err := list.find(id)
			if err == nil {
				watch.close()
			}
			return
		},
	} {
		switch fn := fn.(type) {
		case func(ctx context.Context, watches *watchList, keys [][]byte, val uint64):
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				meta := get[*meta](ctx, ctxKeyMeta)
				fn(ctx, getWatchList(ctx), keys(m, meta), val(m, meta))
			})
		case func(ctx context.Context, watches *watchList, id, from, to []byte):
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				meta := get[*meta](ctx, ctxKeyMeta)
				k := keys(m, meta)
				if len(k) != 3 {
					panic(`expected 3 args`)
				}
				fn(ctx, getWatchList(ctx),
					append([]byte{}, k[0]...),
					append([]byte{}, k[1]...),
					append([]byte{}, k[2]...))
			})
		case func(ctx context.Context, watches *watchList, id []byte) error:
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				meta := get[*meta](ctx, ctxKeyMeta)
				err := fn(ctx, getWatchList(ctx), getData(m, meta))
				setErr(m, meta, err)
			})
		default:
			log.Panicf("Method signature implementation missing: %#v", fn)
		}
	}
	p.module, err = builder.Instantiate(ctx)
	return
}

// InitContext retrieves the meta page from the wasm module
func (p *hostModule) InitContext(ctx context.Context, m api.Module) (context.Context, error) {
	stack, err := m.ExportedFunction(`__range_watch`).Call(ctx)
	if err != nil {
		return ctx, err
	}
	meta := &meta{}
	ptr := uint32(stack[0])
	for i, v := range []*uint32{
		&meta.ptrBuf,
		&meta.ptrBufCap,
		&meta.ptrBufLen,
		&meta.ptrErr,
		&meta.ptrErrCap,
		&meta.ptrErrLen,
		&meta.ptrVal,
	} {
		*v = readUint32(m, ptr+uint32(4*i))
	}
	return context.WithValue(ctx, ctxKeyMeta, meta), nil
}

func getWatchList(ctx context.Context) *watchList {
	return get[*watchList](ctx, ctxKeyWatchList)
}

func dataBuf(m api.Module, meta *meta) []byte {
	return read(m, meta.ptrBuf, 0, meta.ptrBufCap)
}

func setVal(m api.Module, meta *meta, val uint64) {
	writeUint64(m, meta.ptrVal, val)
}

func getVal(m api.Module, meta *meta) (val uint64) {
	return readUint64(m, meta.ptrVal)
}

func setData(m api.Module, meta *meta, b []byte) {
	copy(dataBuf(m, meta)[:len(b)], b)
	writeUint32(m, meta.ptrBufLen, uint32(len(b)))
}

func getData(m api.Module, meta *meta) []byte {
	return read(m, meta.ptrBuf, meta.ptrBufLen, meta.ptrBufCap)
}

func errBuf(m api.Module, meta *meta) []byte {
	return read(m, meta.ptrErr, 0, meta.ptrErrCap)
}

func getErr(m api.Module, meta *meta) (err error) {
	if b := read(m, meta.ptrErr, meta.ptrErrLen, meta.ptrErrCap); len(b) > 0 {
		err = errors.New(string(b))
	}
	return
}

func setErr(m api.Module, meta *meta, err error) {
	var msg string
	if err != nil {
		msg = err.Error()
		copy(errBuf(m, meta)[:len(msg)], msg)
	}
	writeUint32(m, meta.ptrErrLen, uint32(len(msg)))
}

func get[T any](ctx context.Context, key string) T {
	v := ctx.Value(key)
	if v == nil {
		log.Panicf("Context item missing %s", key)
	}
	return v.(T)
}

func val(m api.Module, meta *meta) uint64 {
	return readUint64(m, meta.ptrVal)
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
		keys = append(keys, buf[:keyLen])
		buf = buf[keyLen:]
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

func writeUint64(m api.Module, ptr uint32, val uint64) {
	if ok := m.Memory().WriteUint64Le(ptr, val); !ok {
		log.Panicf("Memory.Read(%d) out of range", ptr)
	}
}
