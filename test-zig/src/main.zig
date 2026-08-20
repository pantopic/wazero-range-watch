const std = @import("std");
const range_watch = @import("range_watch");

var arena_state = std.heap.ArenaAllocator.init(std.heap.wasm_allocator);

export fn _start() void {
    range_watch.init(arena_state.allocator());
    range_watch.receive(recv) catch {};
}

fn recv(notices: []range_watch.Notice) void {
    var tmp: [512]u8 = undefined;
    for (notices) |notice| {
        var offset: usize = 0;
        var written = std.fmt.bufPrint(tmp[offset..], "{d} ", .{notice.val}) catch return;
        offset += written.len;
        for (notice.ids, 0..) |id, index| {
            if (index > 0) {
                tmp[offset] = ' ';
                offset += 1;
            }
            written = std.fmt.bufPrint(tmp[offset..], "{s}", .{id}) catch return;
            offset += written.len;
        }
        tmp[offset] = '\n';
        offset += 1;
        const line = tmp[0..offset];
        const iovs = [_]std.os.wasi.ciovec_t{.{ .base = line.ptr, .len = line.len }};
        var nwritten: usize = undefined;
        _ = std.os.wasi.fd_write(1, &iovs, iovs.len, &nwritten);
    }
}

export fn test_group_start() void {
    range_watch.groupStart();
}

export fn test_group_stop() void {
    range_watch.groupStop();
}

export fn test_emit(val: u32) void {
    range_watch.queue(val, &.{
        "test-100",
        "test-200",
        "test-300",
    });
    range_watch.flush() catch unreachable;
}

export fn test_queue(val: u32) void {
    range_watch.queue(val, &.{
        "test-100",
        "test-200",
        "test-300",
    });
}

export fn test_flush() void {
    range_watch.flush() catch unreachable;
}

export fn test_clear() void {
    range_watch.clear() catch unreachable;
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
    range_watch.queue(val, &.{key(&kb, val)});
    range_watch.flush() catch unreachable;
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
