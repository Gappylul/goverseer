# Goverseer

[![Go Reference](https://pkg.go.dev/badge/github.com/gappylul/goverseer.svg)](https://pkg.go.dev/github.com/gappylul/goverseer)
[![Go Report Card](https://goreportcard.com/badge/github.com/gappylul/goverseer)](https://goreportcard.com/report/github.com/gappylul/goverseer)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Production-ready process supervision for Go applications inspired by Erlang/OTP.

## Features

- **Erlang-style supervision trees** with restart strategies
- **Restart intensity limits** to prevent crash loops
- **Multiple backoff policies** (exponential, linear, constant, jitter)
- **Dynamic child management** at runtime
- **Panic recovery** with stack traces
- **Graceful shutdown** with configurable timeouts
- **Event system** for logging and metrics
- **Hierarchical supervisors** for complex applications
- **Thread-safe** using actor model
- **Zero external dependencies**

## Installation

```bash
go get github.com/gappylul/goverseer
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/gappylul/goverseer"
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
        panic(err)
    }
    
    // Wait for shutdown signal...
    sup.Wait()
}
```

## Restart Strategies

- **OneForOne**: Restart only the failed child
- **OneForAll**: Stop and restart all children
- **RestForOne**: Restart failed child and all children started after it
- **SimpleOneForOne**: Dynamic worker pool pattern

## Restart Types

- **Permanent**: Always restart (use for critical services)
- **Transient**: Restart only on error/panic (use for tasks that can complete)
- **Temporary**: Never restart (use for one-off tasks)