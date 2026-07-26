const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.resolveTargetQuery(.{
        .cpu_arch = .wasm32,
        .os_tag = .wasi,
    });
    const optimize = b.standardOptimizeOption(.{
        .preferred_optimize_mode = .ReleaseSmall,
    });
    const exe = b.addExecutable(.{
        .name = "test-zig",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "range_watch", .module = b.dependency("range_watch", .{}).module("range_watch") },
            },
        }),
    });
    // The module exports its own plain _start (see src/main.zig) instead of
    // the WASI entry point so instantiation does not proc_exit.
    exe.entry = .disabled;
    exe.rdynamic = true;
    // Keep initial memory under the host test's 64 page (4MB) limit
    exe.stack_size = 256 << 10;
    b.installArtifact(exe);
}
