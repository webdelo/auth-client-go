---
prefix: FM
name: Factory Method
packages: []
triggers:
  - factory
  - factory method
  - dispatch by type
  - object creation
  - фабрика
  - фабричный метод
description: "Factory Method pattern: interface + dispatch (switch/map) for runtime object creation without specifying concrete types"
---

# Factory Method Pattern in Go

## FM-001: Description

The Factory Method pattern provides an interface for creating objects without specifying their concrete types. In Go, this is implemented using interfaces and dispatch logic (switch/map), not class inheritance as in Java/C++.

A factory encapsulates object creation logic, allowing the caller to request a product by key (e.g., exchange name + section) and receive an interface back. This decouples the caller from concrete implementations.

## FM-002: When to Use

- Multiple implementations of the same interface, selected at runtime
- Object creation requires validation or configuration
- New implementations may be added without modifying existing code

## FM-003: Go-Idiomatic Implementation

### FM-003-A: Define the Product Interface

The product interface lives in the **producer** package (where implementations reside). The consumer references it.

```go
// TradeParser reads raw trades one at a time from a CSV source.
type TradeParser interface {
    Next(ctx context.Context) (entity.RawTrade, error)
    Close() error
}
```

### FM-003-B: Implement Concrete Products

```go
type BinanceSpotTradeParser struct { /* ... */ }

func (p *BinanceSpotTradeParser) Next(ctx context.Context) (entity.RawTrade, error) { /* ... */ }
func (p *BinanceSpotTradeParser) Close() error { return nil }
```

### FM-003-C: Factory with Dispatch

```go
type parserKey struct {
    Market  primitives.MarketCode
    Section primitives.Section
}

type ParserFactory struct {
    log ctxlogger.Logger
}

func NewParserFactory(log ctxlogger.Logger) *ParserFactory {
    return &ParserFactory{log: log}
}

// Create returns a parser for the given reader and symbol.
// No context.Context — factory creates objects, not runtime operations.
func (f *ParserFactory) Create(r io.Reader, symbol primitives.Symbol) (TradeParser, error) {
    key := parserKey{Market: symbol.Market.Code, Section: symbol.Market.Section}

    switch key {
    case parserKey{Market: MarketBinance, Section: primitives.SectionSpot}:
        return NewBinanceSpotTradeParser(r, symbol, f.log)
    default:
        return nil, fmt.Errorf("no parser for exchange %q section %q",
            symbol.Market.Code, symbol.Market.Section)
    }
}
```

## FM-004: Key Rules

1. **Factory returns interface**, not concrete type — callers depend on behavior, not implementation
2. **No `context.Context`** in factory methods — context is for runtime I/O, not object creation
3. **No `//nolint`** annotations (ES-0171)
4. **Use constants** for dispatch keys (`MarketBinance`, not `"binance"`)
5. **Value parameters** for domain objects like Symbol (ES-0170)

## FM-005: Anti-Patterns

| Anti-Pattern | Problem | Fix |
|-------------|---------|-----|
| Return concrete type | Tight coupling to implementation | Return interface |
| Accept `context.Context` | Factory creates objects, not I/O | Remove context |
| String literals as keys | Magic strings, typos | Use typed constants |
| Single-implementation factory | Over-engineering | Use simple constructor |
| `panic` on unknown key | Crashes runtime | Return error |

## FM-006: Reference

- [Refactoring Guru — Factory Method in Go](https://refactoring.guru/design-patterns/factory-method/go/example)
- [Effective Go — Interfaces](https://go.dev/doc/effective_go#interfaces)
