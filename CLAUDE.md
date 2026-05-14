# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**auth-client-go** is a Go library that [TODO: add description].

**Project Type:** Library

## Build & Development Commands

```bash
# Build
go build ./lib/...

# Run tests
go test ./lib/...              # All tests
go test -race ./lib/...        # With race detector

# Benchmarks
go test -bench=. -benchmem ./lib/...

# Code quality
go fmt ./...
go vet ./...
golangci-lint run ./... --timeout=5m
```

## Project Structure

```
auth-client-go/
├── lib/                    # Main library code
│   ├── lib.go             # Main library file
│   ├── lib_test.go        # Unit tests
│   ├── internal/          # Internal packages (not exported)
│   └── examples/          # Runnable examples
│       └── basic/
│           └── main.go
├── dev/
│   └── tools/             # Development tools
├── .github/
│   └── workflows/
│       └── ci.yaml        # CI pipeline
├── .memory_bank/          # Knowledge base for Claude Code
├── .claude/               # Claude Code configuration
├── .golangci.yaml         # Linter configuration
├── go.mod
├── Makefile
├── README.md
├── CHANGELOG.md
└── scratch.yaml           # Scratch CLI configuration
```

## Go Coding Standards

### Naming
- Package names: `snake_case` (e.g., `my_package`)
- Constructors: Always `New()`, never `NewTypeName()`
- Options: Use `With*()` prefix for optional configurations

### Error Handling
- Wrap errors: `fmt.Errorf("operation_name: %w", err)`
- "Not found" conditions return empty result + nil error, not NotFoundError
- No empty lines before `if err != nil`

### Other Rules
- `context.Context` always first argument
- Max 120 chars per line
- Functions >40 lines require review, >80 lines are bad
- Use pointer receivers consistently across all methods of a type
- Interfaces defined where consumed, not produced

## Linting (CRITICAL)

**Do NOT modify `.golangci.yaml`!**

- When receiving linter warnings, **ALWAYS fix the code**, not the linter configuration
- Do not add exclusions, disable linters, or change thresholds
- `//nolint` directives are allowed ONLY with justification and in exceptional cases

## Agent Usage

See [.claude/AGENTS.md](.claude/AGENTS.md) for agent usage rules and automation guidelines.
