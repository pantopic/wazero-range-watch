//! Zig SDK for the pantopic/wazero-range-watch host module.
//! Mirrors the ABI of sdk-go: the guest exports `__range_watch` (meta page)
//! and `__range_watch_recv`, and imports the host functions below.

const std = @import("std");

pub const Error = error{
    WatchReceiveAlreadyRegistered,
    Host,
};

const buf_cap_bytes = 16 << 10; // 16KB
const err_cap_bytes = 1 << 10; // 1KB

var _buf: [buf_cap_bytes]u8 = undefined;
var _buf_cap: u32 = buf_cap_bytes;
var _buf_len: u32 = 0;
var _err: [err_cap_bytes]u8 = undefined;
var _err_cap: u32 = err_cap_bytes;
var _err_len: u32 = 0;
var _val: u64 = 0;
var _meta: [7]u32 = undefined;

var _recv: ?*const fn (id: []const u8, val: u64) void = null;

export fn __range_watch() u32 {
    _meta = .{
        @intFromPtr(&_buf),
        @intFromPtr(&_buf_cap),
        @intFromPtr(&_buf_len),
        @intFromPtr(&_err),
        @intFromPtr(&_err_cap),
        @intFromPtr(&_err_len),
        @intFromPtr(&_val),
    };
    return @intFromPtr(&_meta);
}

export fn __range_watch_recv() void {
    if (_recv) |f| f(_buf[0.._buf_len], _val);
}

extern "pantopic/wazero-range-watch" fn __range_watch_flush() void;
extern "pantopic/wazero-range-watch" fn __range_watch_reserve() void;
extern "pantopic/wazero-range-watch" fn __range_watch_open() void;
extern "pantopic/wazero-range-watch" fn __range_watch_start() void;
extern "pantopic/wazero-range-watch" fn __range_watch_stop() void;

/// Registers a callback to receive watch notices
pub fn receive(f: *const fn (id: []const u8, val: u64) void) Error!void {
    if (_recv != null) return Error.WatchReceiveAlreadyRegistered;
    _recv = f;
}

/// Broadcasts a value to watchers of a set of keys
pub fn emit(v: u64, keys: []const []const u8) void {
    _val = v;
    _buf_len = 0;
    for (keys) |k| {
        if (!appendKey(k)) {
            __range_watch_flush();
            _buf_len = 0;
            _ = appendKey(k);
        }
    }
    if (_buf_len > 0) {
        __range_watch_flush();
    }
}

/// Locks the range watch id for future opening
pub fn reserve(id: []const u8) Error!void {
    setData(id);
    __range_watch_reserve();
    return getErr();
}

/// Starts receiving values into a buffer
pub fn open(id: []const u8, from: []const u8, to: []const u8) Error!void {
    _buf_len = 0;
    _ = appendKey(id);
    _ = appendKey(from);
    _ = appendKey(to);
    __range_watch_open();
    return getErr();
}

/// Begins the processing of values in the buffer
pub fn start(id: []const u8) Error!void {
    setData(id);
    __range_watch_start();
    return getErr();
}

/// Closes the range watch
pub fn stop(id: []const u8) Error!void {
    setData(id);
    __range_watch_stop();
    return getErr();
}

/// Returns the message of the most recent host error (valid until the next host call)
pub fn lastError() []const u8 {
    return _err[0.._err_len];
}

fn setData(b: []const u8) void {
    _buf_len = @intCast(b.len);
    @memcpy(_buf[0..b.len], b);
}

fn getErr() Error!void {
    if (_err_len > 0) return Error.Host;
}

fn appendKey(k: []const u8) bool {
    if (_buf_len + 2 + k.len > _buf_cap) {
        return false;
    }
    std.mem.writeInt(u16, _buf[_buf_len..][0..2], @intCast(k.len), .big);
    _buf_len += 2;
    @memcpy(_buf[_buf_len..][0..k.len], k);
    _buf_len += @intCast(k.len);
    return true;
}
