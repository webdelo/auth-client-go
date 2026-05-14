# Project Map

## Directory Structure

```
auth-client-go/
├── lib/                    # Main library code
│   ├── lib.go             # Main library file
│   ├── lib_test.go        # Unit tests
│   ├── internal/          # Internal packages
│   └── examples/          # Runnable examples
│       └── basic/
├── dev/
│   └── tools/             # Development tools
├── .github/
│   └── workflows/         # CI configuration
├── .memory_bank/          # Knowledge base
├── .claude/               # Claude Code configuration
└── [configuration files]
```

## Key Files

- `lib/lib.go` - Main entry point
- `Makefile` - Build and development commands
- `.golangci.yaml` - Linter configuration
- `scratch.yaml` - Scratch CLI configuration
