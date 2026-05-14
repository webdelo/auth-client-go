# auth-client-go

[![Go Reference](https://pkg.go.dev/badge/github.com/webdelo/auth-client-go.svg)](https://pkg.go.dev/github.com/webdelo/auth-client-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/webdelo/auth-client-go)](https://goreportcard.com/report/github.com/webdelo/auth-client-go)

A Go library for [TODO: add description].

## Installation

```bash
go get github.com/webdelo/auth-client-go
```

## Quick Start

```go
package main

import (
    "fmt"

    "github.com/webdelo/auth-client-go/lib"
)

func main() {
    fmt.Println("Version:", lib.Version)
}
```

## Features

- [TODO: List key features]

## Examples

See the [examples](lib/examples/) directory for runnable examples:

- [basic](lib/examples/basic/) - Basic usage example

## Development

### Prerequisites

- Go 1.24 or later
- golangci-lint v1.64.8

### Commands

```bash
make help            # Show all available commands
make test            # Run unit tests
make test-race       # Run tests with race detector
make bench           # Run benchmarks
make coverage        # Generate coverage report
make lint            # Run linter
make build-examples  # Build example programs
```

## License

[TODO: Add license]
