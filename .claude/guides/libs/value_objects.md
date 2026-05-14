---
prefix: FVO
name: Financial Value Objects
packages:
  - github.com/webdelo/financial-values/assetquantity
  - github.com/webdelo/financial-values/percent
  - github.com/webdelo/financial-values/quantity
triggers:
  - value object
  - financial-values
  - AssetQuantity
  - Percent
  - Quantity
  - Coins
  - Units
  - TickSize
  - BaseSize
description: "Financial arithmetic: AssetQuantity (price/volume with Asset precision), Percent (fractions), Quantity (dimensionless); rounding via TickSize/BaseSize"
---

# Value Objects: financial-values

Rules for working with `github.com/webdelo/financial-values`.

---

## FVO-001: Type Hierarchy and Selection

| Task | Type | Package |
|------|------|---------|
| Price, volume, cost | `AssetQuantity` | `assetquantity` |
| Percent, fraction | `Percent` | `percent` |
| Dimensionless multiplier/divisor | `Quantity` | `quantity` |
| Asset metadata | `Asset` | `assetquantity` |

**Rule:** Never store prices/volumes as `float64`, `int64`, `string`, or `Coins`/`Units`
in the domain layer. Use `AssetQuantity` exclusively.

---

## FVO-002: AssetQuantity

### FVO-003: Constructors

```go
// From DB (coins = minimum units)
aq := assetquantity.FromCoins(row.PriceCoins, symbol.TickSize.Asset())

// From number
aq := assetquantity.FromUnits(1.50, usdAsset)

// From string
aq, err := assetquantity.FromUnitsInString("51.37", btcAsset)

// Zero value
aq := assetquantity.ZeroValue(asset)

// Nil / zero checks
if aq.IsNil() { ... }   // not set at all
if aq.IsZero() { ... }  // set, but equals 0
```

### FVO-004: Arithmetic — error vs panic

```go
// Use error-version in business logic:
result, err := price.Add(fee)
result, err := price.Sub(discount)
ratio, err  := price.Divide(basePrice)      // → Quantity (dimensionless)
pct, err    := price.PercentFrom(basePrice) // → Percent

// Use WithPanic only in tests or when asset mismatch is contractually impossible:
result := price.AddWithPanic(fee)
```

### FVO-005: Applying a Percent

```go
// Calculate share
share := price.Share(percent.FromUnit(15.5))   // 15.5% of price

// Recover original from share
full := partial.DivideOnPercent(percent.FromUnit(15.5))
```

### FVO-006: Comparison

```go
// Error-version (recommended in business logic):
ok, err := currentPrice.More(stopLoss)

// Panic-version (in loops with guaranteed single asset):
if currentPrice.MoreWithPanic(stopLoss) { ... }

// Min / max / sum:
highest, err := assetquantity.Max(price1, price2, price3)
total, err   := assetquantity.Sum(fees...)
```

### FVO-007: Rounding (tick size)

```go
roundedPrice := rawPrice.RoundTo(symbol.TickSize.Quantity())
floorPrice   := rawPrice.RoundDownTo(symbol.TickSize.Quantity())
```

### FVO-008: Sequence Builder (chained operations)

```go
result, err := assetquantity.NewSequence(initialPrice).
    Add(fee).
    Sub(discount).
    Multiply(quantity).
    Calculate()
```

---

## FVO-009: Percent

### FVO-010: Constructors

```go
p := percent.FromUnit(34.5)              // 34.5%
p := percent.FromFloat(0.345)            // 34.5%
p, err := percent.FromUnitInString("34.5")
```

### FVO-011: Arithmetic

```go
total   := p1.Add(p2)
avg     := percent.Average(p1, p2, p3)
doubled := p.MultiplyFloat(2.0)
half    := p.DivideOnParts(2.0)
```

### FVO-012: Serialization

JSON stores in units (34.5% → `34.5`). In DB store as `NUMERIC`/`DECIMAL`, read via:

```go
p, err := percent.FromUnitInString(row.SLPercent)
```

---

## FVO-013: Quantity

Used as a **dimensionless multiplier** (result of dividing two AssetQuantity values,
or a coefficient for multiplication):

```go
// Obtain from division AssetQuantity / AssetQuantity
ratio, err := price.Divide(basePrice)  // → Quantity

// Multiply AssetQuantity by coefficient
scaled := price.Multiply(ratio)
```

**Do not use Quantity** for storing prices, volumes, or percentages.

---

## FVO-014: Anti-patterns

```go
// FORBIDDEN: store financial data as float64
type BadStopLoss struct { Price float64 }

// FORBIDDEN: convert via .Units() for arithmetic
result := price.Units() + fee.Units()

// FORBIDDEN: use Coins directly as a financial value
func calcFee(coins int64) int64 { return coins * 5 / 100 }

// CORRECT:
type StopLoss struct { Price assetquantity.AssetQuantity }
result, err := price.Add(fee)
fee, err     := price.Share(percent.FromUnit(5))
```
