wasm:
	@cd test && tinygo build -buildmode=wasi-legacy -target=wasi -opt=z -gc=leaking -scheduler=none -o ../host/test.wasm module.go

wasm-zig:
	@cd test-zig && zig build --release=small
	@cp test-zig/zig-out/bin/test-zig.wasm host/test-zig.wasm

test:
	@cd host && go test . -v

cover:
	@mkdir -p _dist
	@cd host && go test . -coverprofile=../_dist/coverage.out -v
	@go tool cover -html=_dist/coverage.out -o _dist/coverage.html

cloc:
	@cloc . --exclude-dir=_example,_dist,internal,cmd --exclude-ext=pb.go

.PHONY: all test clean
