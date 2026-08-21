package wazero_range_watch

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/pantopic/wazero-pool"
)

// Name is the name of this host module.
const Name = "pantopic/wazero-range-watch"

var (
	ctxKeyMeta      = Name + `/meta`
	ctxKeyGroup     = Name + `/group`
	ctxKeyWatchList = Name + `/watch_list`
)

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
	limit  int
}

type Option func(*hostModule)

func New(opts ...Option) (h *hostModule) {
	h = &hostModule{}
	for _, opt := range opts {
		opt(h)
	}
	return
}

func (h *hostModule) Name() string {
	return Name
}
func (h *hostModule) Stop() {}

// Register instantiates the host module, making it available to all module instances in this runtime
func (h *hostModule) Register(ctx context.Context, r wazero.Runtime) (err error) {
	builder := r.NewHostModuleBuilder(Name)
	register := func(name string, fn func(ctx context.Context, m api.Module, stack []uint64)) {
		builder = builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(fn), nil, nil).Export(name)
	}
	for name, fn := range map[string]any{
		"__range_watch_queue": func(ctx context.Context, list *watchList, val uint64, keys [][]byte) {
			list.queue(val, keys)
		},
		"__range_watch_flush": func(ctx context.Context, list *watchList) {
			list.flush()
		},
		"__range_watch_clear": func(ctx context.Context, list *watchList) {
			list.clear()
		},
		"__range_watch_reserve": func(group *watchGroup, id []byte) (err error) {
			_, err = group.reserve(id)
			return
		},
		"__range_watch_open": func(group *watchGroup, id, from, to []byte, synced bool) (err error) {
			_, err = group.open(id, from, to, synced)
			if err != nil {
				return
			}
			return
		},
		"__range_watch_start": func(ctx context.Context, group *watchGroup, id []byte) (err error) {
			watch := group.find(id)
			if watch == nil {
				return ErrWatchNotFound
			}
			if watch.isSynced() {
				return
			}
			meta := get[*meta](ctx, ctxKeyMeta)
			var buf []byte
			var val uint64 = 0
			var count uint16 = 0
			var idCount uint16 = 0
			var idCountIdx int = 0
			var batches = make(chan []byte)
			var bufCap uint32
			var bufPool = sync.Pool{}
			reset := func() {
				buf = bufPool.Get().([]byte)[:0]
				buf = binary.BigEndian.AppendUint16(buf, 0)
				val = 0
				count = 0
				idCount = 0
				idCountIdx = 0
			}
			watch.Go(func() {
				wazeropool.FromContext(ctx).Run(func(mod api.Module) {
					bufCap = readUint32(mod, meta.ptrBufCap)
				})
				bufPool = sync.Pool{
					New: func() any { return make([]byte, 0, bufCap) },
				}
				var closed bool
				for {
					reset()
					select {
					case msg, ok := <-watch.out:
						if !ok {
							close(batches)
							return
						}
					drain:
						for {
							if len(buf)+len(msg.w.id)+12 >= cap(buf) || count == 0xFFFF {
								binary.BigEndian.PutUint16(buf, count)
								binary.BigEndian.PutUint16(buf[idCountIdx:], idCount)
								batches <- buf
								reset()
							}
							if msg.val != val || idCount == 0xFFFF {
								if idCountIdx > 0 {
									binary.BigEndian.PutUint16(buf[idCountIdx:], idCount)
								}
								val = msg.val
								buf = binary.BigEndian.AppendUint64(buf, val)
								idCountIdx = len(buf)
								buf = binary.BigEndian.AppendUint16(buf, 0)
								idCount = 0
								count++
							}
							buf = binary.BigEndian.AppendUint16(buf, uint16(len(msg.w.id)))
							buf = append(buf, msg.w.id...)
							idCount++
							select {
							case msg = <-watch.out:
							default:
								binary.BigEndian.PutUint16(buf, count)
								binary.BigEndian.PutUint16(buf[idCountIdx:], idCount)
								batches <- buf
								break drain
							}
						}
					default:
						if !closed {
							go watch.sync()
							closed = true
						} else {
							time.Sleep(time.Millisecond)
						}
					case <-ctx.Done():
						if !closed {
							go watch.sync()
							closed = true
						}
						return
					}
				}
			})
			watch.Go(func() {
				for b := range batches {
					wazeropool.FromContext(ctx).Run(func(mod api.Module) {
						setData(mod, meta, b)
						setErr(mod, meta, nil)
						if _, err := mod.ExportedFunction("__range_watch_recv").Call(ctx); err != nil {
							slog.Error("Error __range_watch_recv A", "err", err.Error())
							return
						}
						if err := getErr(mod, meta); err != nil {
							slog.Error("Error __range_watch_recv B", "err", err.Error())
							return
						}
					})
					bufPool.Put(b[:0])
				}
			})
			return
		},
		"__range_watch_stop": func(ctx context.Context, wg *watchGroup, id []byte) (err error) {
			return wg.close(id)
		},
		"__range_watch_group_start": func(ctx context.Context, wg *watchGroup) {
			wg.start(ctx)
		},
		"__range_watch_group_stop": func(ctx context.Context, wg *watchGroup) {
			wg.closeAll()
		},
	} {
		switch fn := fn.(type) {
		case func(ctx context.Context, watches *watchList):
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				fn(ctx, getWatchList(ctx))
			})
		case func(ctx context.Context, watches *watchList, val uint64, keys [][]byte):
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				meta := get[*meta](ctx, ctxKeyMeta)
				fn(ctx, getWatchList(ctx), getVal(m, meta), keys(m, meta))
			})
		case func(group *watchGroup, id, from, to []byte, synced bool) error:
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				meta := get[*meta](ctx, ctxKeyMeta)
				k := keys(m, meta)
				if len(k) != 3 {
					panic(`expected 3 args`)
				}
				fn(getWatchGroup(ctx),
					append([]byte{}, k[0]...),
					append([]byte{}, k[1]...),
					append([]byte{}, k[2]...),
					getVal(m, meta) > 0)
				setErr(m, meta, err)
			})
		case func(ctx context.Context, wg *watchGroup, id []byte) error:
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				meta := get[*meta](ctx, ctxKeyMeta)
				err := fn(ctx, getWatchGroup(ctx), getData(m, meta))
				setErr(m, meta, err)
			})
		case func(wg *watchGroup, id []byte) error:
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				meta := get[*meta](ctx, ctxKeyMeta)
				err := fn(getWatchGroup(ctx), getData(m, meta))
				setErr(m, meta, err)
			})
		case func(ctx context.Context, wg *watchGroup):
			register(name, func(ctx context.Context, m api.Module, stack []uint64) {
				fn(ctx, getWatchGroup(ctx))
			})
		default:
			log.Panicf("Method signature implementation missing: %#v", fn)
		}
	}
	h.module, err = builder.Instantiate(ctx)
	return
}

// InitContext retrieves the meta page from the wasm module
func (h *hostModule) InitContext(ctx context.Context, m api.Module) (context.Context, error) {
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

func (h *hostModule) ContextCopy(dst, src context.Context) context.Context {
	v := src.Value(ctxKeyMeta)
	if v == nil {
		return dst
	}
	dst = context.WithValue(dst, ctxKeyMeta, v.(*meta))
	if v := src.Value(ctxKeyWatchList); v != nil {
		dst = context.WithValue(dst, ctxKeyWatchList, v.(*watchList))
		if v := src.Value(ctxKeyGroup); v != nil && v.(*watchGroup).active {
			dst = context.WithValue(dst, ctxKeyGroup, v.(*watchGroup))
		} else {
			dst = context.WithValue(dst, ctxKeyGroup, newWatchGroup())
		}
	} else {
		dst = context.WithValue(dst, ctxKeyWatchList, newWatchList(dst))
	}
	return dst
}

func getWatchList(ctx context.Context) *watchList {
	return get[*watchList](ctx, ctxKeyWatchList)
}

func getWatchGroup(ctx context.Context) *watchGroup {
	return get[*watchGroup](ctx, ctxKeyGroup)
}

func dataBuf(m api.Module, meta *meta) []byte {
	return read(m, meta.ptrBuf, 0, meta.ptrBufCap)
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

func getVal(m api.Module, meta *meta) uint64 {
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
	buf := getData(m, meta)
	for len(buf) > 0 {
		keyLen := binary.BigEndian.Uint16(buf)
		buf = buf[2:]
		keys = append(keys, buf[:keyLen])
		buf = buf[keyLen:]
	}
	return
}

func read(m api.Module, ptrData, ptrLen, ptrCap uint32) (buf []byte) {
	buf, ok := m.Memory().Read(ptrData, readUint32(m, ptrCap))
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
