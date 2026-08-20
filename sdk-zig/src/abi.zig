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
pub var _meta: [7]u32 = undefined;

pub var _recv: ?*const fn (notices: []Notice) void = null;
pub var _allocator: std.mem.Allocator = undefined;

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

pub const Notice = struct {
    val: u64,
    ids: [][]const u8,
};

export fn __range_watch_recv() void {
    var i: u32 = 0;
    const count = std.mem.readInt(u16, @ptrCast(&_buf[i]), .big);
    const notices = _allocator.alloc(Notice, count) catch unreachable;
    i += 2;
    for (notices) |*n| {
        n.* = Notice{
            .val = std.mem.readInt(u64, @ptrCast(&_buf[i]), .big),
            .ids = _allocator.alloc([]const u8, std.mem.readInt(u16, @ptrCast(&_buf[i + 8]), .big)) catch unreachable,
        };
        i += 10;
        for (n.ids) |*id| {
            const len = std.mem.readInt(u16, @ptrCast(&_buf[i]), .big);
            i += 2;
            id.* = _buf[i .. i + len];
            i += len;
        }
    }
    if (_recv) |f| f(notices);
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
