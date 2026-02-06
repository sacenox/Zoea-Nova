# Go Compilation Without CGO

Zoea Nova is built as a pure Go application with `CGO_ENABLED=0`. This document explains the benefits and implications of this architectural decision.

## Why Pure Go?

### Compilation Speed

CGO significantly slows down Go compilation:

- **CGO enabled**: Sequential compilation, C compiler overhead, 2-10x slower builds
- **Pure Go**: Parallel compilation, fast incremental builds, near-instant for small changes

```bash
# Benchmark the difference
time CGO_ENABLED=1 go build ./cmd/zoea  # Slower
time CGO_ENABLED=0 go build ./cmd/zoea  # Faster (default)
time go build ./cmd/zoea                # Same as CGO_ENABLED=0
```

### Build Simplicity

Pure Go eliminates external dependencies:

- No C compiler toolchain required (gcc/clang)
- Single static binary output
- Reproducible builds across environments
- Trivial cross-compilation:

```bash
# Cross-compile for Linux from any platform
GOOS=linux GOARCH=amd64 go build ./cmd/zoea

# Cross-compile for macOS ARM
GOOS=darwin GOARCH=arm64 go build ./cmd/zoea

# Cross-compile for Windows
GOOS=windows GOARCH=amd64 go build ./cmd/zoea
```

### Runtime Performance

- No overhead crossing Go/C boundary
- Better garbage collector coordination
- Simpler memory management

## Pure Go Dependencies

Zoea Nova's stack is entirely CGO-free:

| Component | Package | Notes |
|-----------|---------|-------|
| SQLite | `modernc.org/sqlite` | Pure Go SQLite implementation |
| HTTP/JSON | Standard library | Native Go |
| TUI | Bubble Tea ecosystem | Pure Go |
| LLM clients | `go-openai` | Pure Go |
| Logging | `zerolog` | Pure Go |
| Configuration | `github.com/BurntSushi/toml` | Pure Go |

## When CGO Is Required

CGO is only necessary when:

1. **Linking against C libraries** - Proprietary SDKs with only C bindings
2. **Calling C functions directly** - Hardware drivers, legacy code
3. **Using CGO-dependent packages** - Some older database drivers (e.g., `mattn/go-sqlite3`)

## Historical Context

Previous versions of Zoea Nova may have used BAML (BoundaryML's AI function framework), which required CGO. Removing BAML eliminated this requirement and significantly improved build times.

## Verification

Check if a package requires CGO:

```bash
# List all dependencies
go list -deps ./cmd/zoea

# Check for CGO usage
go list -f '{{if .CgoFiles}}{{.ImportPath}}: {{.CgoFiles}}{{end}}' ./...

# Verify pure Go build
CGO_ENABLED=0 go build ./cmd/zoea  # Should succeed
```

If the build fails with `CGO_ENABLED=0`, a dependency requires CGO.

## Build Configuration

The `Makefile` explicitly disables CGO:

```makefile
build:
	CGO_ENABLED=0 go build -o bin/zoea ./cmd/zoea
```

This ensures:
- Consistent builds across environments
- No accidental CGO dependencies
- Fast compilation times
- Easy cross-compilation

## References

- [Go CGO Documentation](https://pkg.go.dev/cmd/cgo)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) - Pure Go SQLite
- [Cross-compilation Guide](https://go.dev/doc/install/source#environment)
