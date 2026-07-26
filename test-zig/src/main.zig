const std = @import("std");
const range_watch = @import("range_watch");

// Plain _start (default entry disabled in build.zig): registers the receive
// callback and returns without proc_exit so the module instance stays open.
export fn _start() void {
    range_watch.receive(recv) catch {};
}

fn recv(id: []const u8, val: u64) void {
    var tmp: [64]u8 = undefined;
    const line = std.fmt.bufPrint(&tmp, "{d},{s}\n", .{ val, id }) catch return;
    const iovs = [_]std.os.wasi.ciovec_t{.{ .base = line.ptr, .len = line.len }};
    var nwritten: usize = undefined;
    _ = std.os.wasi.fd_write(1, &iovs, iovs.len, &nwritten);
}

export fn test_emit(val: u32) void {
    range_watch.emit(val, &.{
        "test-100",
        "test-200",
        "test-300",
    });
}

export fn test_create(from: u32, to: u32) void {
    var idb: [32]u8 = undefined;
    var fb: [32]u8 = undefined;
    var tb: [32]u8 = undefined;
    const id = watchID(&idb, from, to);
    range_watch.open(id, key(&fb, from), key(&tb, to)) catch {};
    range_watch.start(id) catch {};
}

export fn test_reserve(from: u32, to: u32) void {
    var idb: [32]u8 = undefined;
    range_watch.reserve(watchID(&idb, from, to)) catch {};
}

export fn test_open(from: u32, to: u32) void {
    var idb: [32]u8 = undefined;
    var fb: [32]u8 = undefined;
    var tb: [32]u8 = undefined;
    const id = watchID(&idb, from, to);
    range_watch.open(id, key(&fb, from), key(&tb, to)) catch {};
}

export fn test_start(from: u32, to: u32) void {
    var idb: [32]u8 = undefined;
    range_watch.start(watchID(&idb, from, to)) catch {};
}

export fn test_emit_2(val: u32) void {
    var kb: [32]u8 = undefined;
    range_watch.emit(val, &.{key(&kb, val)});
}

export fn test_stop(from: u32, to: u32) void {
    var idb: [32]u8 = undefined;
    range_watch.stop(watchID(&idb, from, to)) catch {};
}

// ie. "100-200"
fn watchID(buf: []u8, from: u32, to: u32) []const u8 {
    return std.fmt.bufPrint(buf, "{d}-{d}", .{ from, to }) catch unreachable;
}

// ie. "test-100"
fn key(buf: []u8, n: u32) []const u8 {
    return std.fmt.bufPrint(buf, "test-{d}", .{n}) catch unreachable;
}
