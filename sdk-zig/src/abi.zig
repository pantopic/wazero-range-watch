//! Zig SDK for the pantopic/wazero-range-watch host module.
//! Mirrors the ABI of sdk-go: the guest exports `__range_watch` (meta page)
//! and `__range_watch_recv`, and imports the host functions below.

const std = @import("std");
const sdk = @import("sdk.zig");

const _buf_cap_default: u32 = 16 << 10; // 16KB
const _err_cap_default: u32 = 1 << 10; // 1KB
const _vals_cap_default: u32 = 1 << 10; // 1,024

pub var _buf_cap: u32 = _buf_cap_default;
pub var _buf_len: u32 = 0;
pub var _buf: [_buf_cap_default]u8 = undefined;
pub var _err_cap: u32 = _err_cap_default;
pub var _err_len: u32 = 0;
pub var _err: [_err_cap_default]u8 = undefined;
pub var _val: u64 = 0;
pub var _vals_cap: u32 = _vals_cap_default;
pub var _vals_len: u32 = 0;
pub var _vals: [_vals_cap_default]u64 = undefined;
pub var _meta: [10]u32 = undefined;

pub var _recv: ?*const fn (id: []const u8, vals: []u64) void = null;

export fn __range_watch() u32 {
    _meta = .{
        @intFromPtr(&_buf),
        @intFromPtr(&_buf_cap),
        @intFromPtr(&_buf_len),
        @intFromPtr(&_err),
        @intFromPtr(&_err_cap),
        @intFromPtr(&_err_len),
        @intFromPtr(&_val),
        @intFromPtr(&_vals),
        @intFromPtr(&_vals_cap),
        @intFromPtr(&_vals_len),
    };
    return @intFromPtr(&_meta);
}

export fn __range_watch_recv() void {
    if (_recv) |f| f(_buf[0.._buf_len], _vals[0.._vals_len]);
}

pub extern "pantopic/wazero-range-watch" fn __range_watch_queue() void;
pub extern "pantopic/wazero-range-watch" fn __range_watch_flush() void;
pub extern "pantopic/wazero-range-watch" fn __range_watch_reset() void;
pub extern "pantopic/wazero-range-watch" fn __range_watch_reserve() void;
pub extern "pantopic/wazero-range-watch" fn __range_watch_open() void;
pub extern "pantopic/wazero-range-watch" fn __range_watch_start() void;
pub extern "pantopic/wazero-range-watch" fn __range_watch_stop() void;
pub extern "pantopic/wazero-range-watch" fn __range_watch_group_start() void;
pub extern "pantopic/wazero-range-watch" fn __range_watch_group_stop() void;

pub fn setData(b: []const u8) void {
    _buf_len = @intCast(b.len);
    @memcpy(_buf[0..b.len], b);
}

pub fn getErr() sdk.Error!void {
    if (_err_len > 0) return sdk.Error.Host;
}

pub fn appendKey(k: []const u8) bool {
    if (_buf_len + 2 + k.len > _buf_cap) {
        return false;
    }
    std.mem.writeInt(u16, _buf[_buf_len..][0..2], @intCast(k.len), .big);
    _buf_len += 2;
    @memcpy(_buf[_buf_len..][0..k.len], k);
    _buf_len += @intCast(k.len);
    return true;
}
