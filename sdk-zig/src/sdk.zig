const std = @import("std");
const abi = @import("abi.zig");

pub const Error = error{
    WatchReceiveAlreadyRegistered,
    Host,
};

/// Registers a callback to receive watch notices
pub fn receive(f: *const fn (id: []const u8, vals: []u64) void) Error!void {
    if (abi._recv != null) return Error.WatchReceiveAlreadyRegistered;
    abi._recv = f;
}

/// Queues alerts for broadcast to watchers of a set of keys
pub fn queue(v: u64, keys: []const []const u8) void {
    abi._val = v;
    abi._buf_len = 0;
    for (keys) |k| {
        if (!abi.appendKey(k)) {
            abi.__range_watch_queue();
            abi._buf_len = 0;
            _ = abi.appendKey(k);
        }
    }
    if (abi._buf_len > 0) {
        abi.__range_watch_queue();
    }
}

/// Broadcasts queued alerts
pub fn flush() Error!void {
    abi.__range_watch_flush();
    return abi.getErr();
}

/// Clears alert queue
pub fn clear() Error!void {
    abi.__range_watch_flush();
    return abi.getErr();
}

/// Locks the range watch id for future opening
pub fn reserve(id: []const u8) Error!void {
    abi.setData(id);
    abi.__range_watch_reserve();
    return abi.getErr();
}

/// Starts receiving values into a buffer
pub fn open(id: []const u8, from: []const u8, to: []const u8) Error!void {
    abi._buf_len = 0;
    _ = abi.appendKey(id);
    _ = abi.appendKey(from);
    _ = abi.appendKey(to);
    abi.__range_watch_open();
    return abi.getErr();
}

/// Begins the processing of values in the buffer
pub fn start(id: []const u8) Error!void {
    abi.setData(id);
    abi.__range_watch_start();
    return abi.getErr();
}

/// Closes the range watch
pub fn stop(id: []const u8) Error!void {
    abi.setData(id);
    abi.__range_watch_stop();
    return abi.getErr();
}

/// Returns the message of the most recent host error (valid until the next host call)
pub fn lastError() []const u8 {
    return abi._err[0..abi._err_len];
}

/// Starts the watch group
pub fn groupStart() void {
    abi.__range_watch_group_start();
}

/// Stops the watch group
pub fn groupStop() void {
    abi.__range_watch_group_stop();
}
