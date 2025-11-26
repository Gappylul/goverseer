# Goverseer

[![Go Reference](https://pkg.go.dev/badge/github.com/Gappylul/goverseer.svg)](https://pkg.go.dev/github.com/Gappylul/goverseer)
[![Go Report Card](https://goreportcard.com/badge/github.com/Gappylul/goverseer)](https://goreportcard.com/report/github.com/Gappylul/goverseer)
[![Tests](https://github.com/Gappylul/goverseer/actions/workflows/test.yml/badge.svg)](https://github.com/Gappylul/goverseer/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/Gappylul/goverseer/branch/main/graph/badge.svg)](https://codecov.io/gh/Gappylul/goverseer)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Production-ready process supervision for Go applications inspired by Erlang/OTP.

## Features

✨ **Erlang-style supervision trees** with multiple restart strategies  
🔄 **Restart intensity limits** to prevent crash loops  
⏱️ **Multiple backoff policies** (exponential, linear, constant, jitter)  
🔌 **Dynamic child management** at runtime  
🛡️ **Panic recovery** with full stack traces  
🎯 **Graceful shutdown** with configurable timeouts  
📊 **Event system** for logging and metrics integration  
🌲 **Hierarchical supervisors** for complex applications  
🔒 **Thread-safe** using actor model pattern  
📦 **Zero external dependencies** - pure Go stdlib

## Installation
```bash
go get github.com/Gappylul/goverseer
```

## Quick Start
```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/Gappylul/goverseer"
)

func worker(ctx context.Context) error {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            fmt.Println("Working...")
        }
    }
}

func main() {
    sup := goverseer.New(
        goverseer.OneForOne,
        goverseer.WithName("main-supervisor"),
        goverseer.WithIntensity(5, time.Minute),
        goverseer.WithChildren(
            goverseer.ChildSpec{
                Name:    "worker-1",
                Start:   worker,
                Restart: goverseer.Permanent,
            },
        ),
    )
    
    if err := sup.Start(); err != nil {
        log.Fatal(err)
    }
    
    log.Println("Supervisor started")
    
    if err := sup.Wait(); err != nil {
        log.Fatal(err)
    }
}
```

## Documentation

- [GoDoc](https://pkg.go.dev/github.com/Gappylul/goverseer) - Full API documentation
- [Examples](./examples) - Working code examples

## Examples

All examples are in the [examples/](./examples) directory:

- **[basic](./examples/basic)** - Simple worker supervision
- **[restart_types](./examples/restart_types)** - Permanent, Transient, Temporary
- **[error_handling](./examples/error_handling)** - Retries and panic recovery
- **[hierarchical](./examples/hierarchical)** - Multi-level supervision trees
- **[worker_pool](./examples/worker_pool)** - Dynamic worker pool with scaling
- **[web_server](./examples/web_server)** - HTTP server with supervision

## Testing
```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run benchmarks
make bench

# Run linter
make lint
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Inspired by Erlang/OTP's supervisor behavior and similar projects in other languages.