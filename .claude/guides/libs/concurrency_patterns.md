---
prefix: CNC
name: Concurrency Patterns
packages:
  - sync
  - golang.org/x/sync/errgroup
triggers:
  - worker pool
  - fan-out
  - fan-in
  - pipeline
  - rate limit
  - goroutine pool
  - batch processing
  - semaphore
  - errgroup
  - chunk
description: "Ready-to-use Go concurrency patterns: worker pool, fan-out/fan-in, pipeline, semaphore, errgroup, rate limiter, circuit breaker, graceful shutdown"
---

# Go Concurrency Patterns

**This document contains READY-TO-USE code patterns for copy-paste into your project.**

> **Purpose**: This file provides production-ready implementations of common concurrency patterns. Copy the code directly into your project and adapt as needed.

> **For rules and guidelines**: See [effective_concurrency.md](../guides/golang/effective_concurrency.md) (EC-XXXX numbered rules)

This document describes standard concurrency patterns in Go that **MUST be considered first** when implementing parallel processing.

> **IMPORTANT**: Use these patterns as a foundation. Only if they don't fit a specific task can you implement your own solution with justification.

---

## CNC-001: Worker Pool

**When to use**: Processing a large number of tasks with a fixed number of workers.

### CNC-001-A: Basic Implementation

```go
package main

import (
    "context"
    "sync"
)

// Task represents a unit of work
type Task struct {
    ID   int
    Data any
}

// Result represents the result of processing
type Result struct {
    TaskID int
    Output any
    Err    error
}

// WorkerPool manages a pool of workers
type WorkerPool struct {
    workerCount int
    tasks       chan Task
    results     chan Result
    wg          sync.WaitGroup
}

// New creates a new worker pool
func New(workerCount, taskBufferSize int) *WorkerPool {
    return &WorkerPool{
        workerCount: workerCount,
        tasks:       make(chan Task, taskBufferSize),
        results:     make(chan Result, taskBufferSize),
    }
}

// Start launches workers
func (wp *WorkerPool) Start(ctx context.Context, processor func(Task) Result) {
    for i := 0; i < wp.workerCount; i++ {
        wp.wg.Add(1)
        go wp.worker(ctx, processor)
    }
}

func (wp *WorkerPool) worker(ctx context.Context, processor func(Task) Result) {
    defer wp.wg.Done()

    for {
        select {
        case <-ctx.Done():
            return
        case task, ok := <-wp.tasks:
            if !ok {
                return
            }
            wp.results <- processor(task)
        }
    }
}

// Submit adds a task to the queue
func (wp *WorkerPool) Submit(task Task) {
    wp.tasks <- task
}

// Close closes the pool and waits for completion
func (wp *WorkerPool) Close() {
    close(wp.tasks)
    wp.wg.Wait()
    close(wp.results)
}

// Results returns the results channel
func (wp *WorkerPool) Results() <-chan Result {
    return wp.results
}
```

### CNC-001-B: Usage

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    pool := New(5, 100) // 5 workers, buffer for 100 tasks

    // Task processor
    processor := func(t Task) Result {
        // Processing...
        return Result{TaskID: t.ID, Output: "done"}
    }

    pool.Start(ctx, processor)

    // Send tasks
    go func() {
        for i := 0; i < 50; i++ {
            pool.Submit(Task{ID: i, Data: i * 2})
        }
        pool.Close()
    }()

    // Get results
    for result := range pool.Results() {
        fmt.Printf("Task %d: %v\n", result.TaskID, result.Output)
    }
}
```

---

## CNC-002: Semaphore (Parallelism Limiting)

**When to use**: Limiting the number of concurrently executing operations.

> **IMPORTANT**: Use the standard implementation `golang.org/x/sync/semaphore` in production code.

### CNC-002-A: Standard Implementation (RECOMMENDED)

```go
import (
    "context"
    "golang.org/x/sync/errgroup"
    "golang.org/x/sync/semaphore"
)

// processWithLimit processes items with parallelism limiting
func processWithLimit(ctx context.Context, items []Item, limit int) error {
    sem := semaphore.NewWeighted(int64(limit))
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item // capture

        // Acquire semaphore (blocks if limit is reached)
        if err := sem.Acquire(ctx, 1); err != nil {
            return err
        }

        g.Go(func() error {
            defer sem.Release(1) // Always release the slot
            return processItem(ctx, item)
        })
    }

    return g.Wait()
}
```

### CNC-002-B: TryAcquire (Non-blocking Acquisition)

```go
func tryProcessWithLimit(ctx context.Context, items []Item, limit int) error {
    sem := semaphore.NewWeighted(int64(limit))
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item

        // Try to acquire without blocking
        if !sem.TryAcquire(1) {
            // Slots are busy, can skip or wait
            continue
        }

        g.Go(func() error {
            defer sem.Release(1)
            return processItem(ctx, item)
        })
    }

    return g.Wait()
}
```

### CNC-002-C: Simple Channel-based Implementation (For Educational Purposes)

> **Note**: Use only for learning. In production code, use `golang.org/x/sync/semaphore`.

```go
// Semaphore limits the number of parallel operations
type Semaphore struct {
    sem chan struct{}
}

// NewSemaphore creates a semaphore with the specified limit
func NewSemaphore(limit int) *Semaphore {
    return &Semaphore{
        sem: make(chan struct{}, limit),
    }
}

// Acquire acquires a slot (blocks if limit is reached)
func (s *Semaphore) Acquire() {
    s.sem <- struct{}{}
}

// Release releases a slot
func (s *Semaphore) Release() {
    <-s.sem
}
```

---

## CNC-003: Fan-Out / Fan-In

**When to use**: Parallelizing work and collecting results.

> **Principle of operation**:
> - **Fan-Out**: N workers read from one input channel
> - **Fan-In**: Results from N workers are merged into one output channel
> - WaitGroup is used for proper output channel closure

### CNC-003-A: Fan-Out (Distribution) — With Context

```go
// fanOut launches workers reading from one input channel
// Returns a single output channel with results
func fanOut(ctx context.Context, input <-chan Task, workerCount int, processor func(Task) Result) <-chan Result {
    output := make(chan Result)
    var wg sync.WaitGroup

    // Launch workers
    wg.Add(workerCount)
    for i := 0; i < workerCount; i++ {
        go func() {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done():
                    return
                case task, ok := <-input:
                    if !ok {
                        // Input channel closed
                        return
                    }
                    select {
                    case output <- processor(task):
                    case <-ctx.Done():
                        return
                    }
                }
            }
        }()
    }

    // Close output channel after all workers finish
    go func() {
        wg.Wait()
        close(output)
    }()

    return output
}
```

### CNC-003-B: Fan-Out (Distribution) — Without Context

> **IMPORTANT**: When the input channel is closed, all workers automatically finish.

```go
// fanOutSimple launches workers reading from one input channel
// Returns a single output channel with results
func fanOutSimple(input <-chan Task, workerCount int, processor func(Task) Result) <-chan Result {
    output := make(chan Result)
    var wg sync.WaitGroup

    // Launch workers
    wg.Add(workerCount)
    for i := 0; i < workerCount; i++ {
        go func() {
            defer wg.Done()
            // When input closes, the loop automatically exits
            for task := range input {
                output <- processor(task)
            }
        }()
    }

    // Close output channel after all workers finish
    go func() {
        wg.Wait()
        close(output)
    }()

    return output
}
```

### CNC-003-C: Fan-In (Merging) — With Context

> **Application**: When you need to merge results from multiple independent sources.

```go
// fanIn merges multiple channels into one
func fanIn(ctx context.Context, channels ...<-chan Result) <-chan Result {
    output := make(chan Result)
    var wg sync.WaitGroup

    // Launch a goroutine for each input channel
    wg.Add(len(channels))
    for _, ch := range channels {
        ch := ch // capture
        go func() {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done():
                    return
                case result, ok := <-ch:
                    if !ok {
                        // Channel closed
                        return
                    }
                    select {
                    case output <- result:
                    case <-ctx.Done():
                        return
                    }
                }
            }
        }()
    }

    // Close output channel after all goroutines finish
    go func() {
        wg.Wait()
        close(output)
    }()

    return output
}
```

### CNC-003-D: Fan-In (Merging) — Without Context

> **IMPORTANT**: When all input channels are closed, the output channel closes automatically.

```go
// fanInSimple merges multiple channels into one
func fanInSimple(channels ...<-chan Result) <-chan Result {
    output := make(chan Result)
    var wg sync.WaitGroup

    // Launch a goroutine for each input channel
    wg.Add(len(channels))
    for _, ch := range channels {
        ch := ch // capture
        go func() {
            defer wg.Done()
            // When ch closes, the loop automatically exits
            for result := range ch {
                output <- result
            }
        }()
    }

    // Close output channel after all goroutines finish
    go func() {
        wg.Wait()
        close(output)
    }()

    return output
}
```

### CNC-003-E: Proper Channel Closure Chain

```go
// Example with full closure control
func processPipeline(tasks []Task, workerCount int, processor func(Task) Result) []Result {
    // 1. Create input channel
    input := make(chan Task)

    // 2. Launch producer
    go func() {
        defer close(input) // IMPORTANT: close input after sending all tasks
        for _, task := range tasks {
            input <- task
        }
    }()

    // 3. Fan-Out: launch workers
    // WaitGroup inside fanOutSimple tracks worker completion
    // Output channel closes automatically after all workers finish
    output := fanOutSimple(input, workerCount, processor)

    // 4. Collect results
    var results []Result
    for result := range output { // Loop exits when output closes
        results = append(results, result)
    }

    return results
}
```

### CNC-003-F: Closure Event Sequence

```
1. Producer finishes work
   └─> close(input)
        └─> Fan-Out workers stop receiving tasks
             └─> Workers exit for range input
                  └─> wg.Done() for each worker
                       └─> wg.Wait() completes
                            └─> close(output)
                                 └─> Consumer exits for range output
```

### CNC-003-G: Example with Fan-In (Merging Multiple Sources)

```go
func mergeMultipleSources(sources []<-chan Result) []Result {
    // Fan-In merges multiple independent sources
    merged := fanInSimple(sources...)

    // Collect all results
    var results []Result
    for result := range merged {
        results = append(results, result)
    }

    return results
}
```

---

## CNC-004: Pipeline (Conveyor)

**When to use**: Sequential processing of data through multiple stages.

```go
// Stage represents a pipeline stage
type Stage func(ctx context.Context, input <-chan any) <-chan any

// Pipeline connects multiple stages
func Pipeline(ctx context.Context, input <-chan any, stages ...Stage) <-chan any {
    current := input

    for _, stage := range stages {
        current = stage(ctx, current)
    }

    return current
}

// Example stage: filtering
func filterStage(predicate func(any) bool) Stage {
    return func(ctx context.Context, input <-chan any) <-chan any {
        output := make(chan any)

        go func() {
            defer close(output)
            for {
                select {
                case <-ctx.Done():
                    return
                case v, ok := <-input:
                    if !ok {
                        return
                    }
                    if predicate(v) {
                        select {
                        case output <- v:
                        case <-ctx.Done():
                            return
                        }
                    }
                }
            }
        }()

        return output
    }
}

// Example stage: transformation
func transformStage(transform func(any) any) Stage {
    return func(ctx context.Context, input <-chan any) <-chan any {
        output := make(chan any)

        go func() {
            defer close(output)
            for {
                select {
                case <-ctx.Done():
                    return
                case v, ok := <-input:
                    if !ok {
                        return
                    }
                    select {
                    case output <- transform(v):
                    case <-ctx.Done():
                        return
                    }
                }
            }
        }()

        return output
    }
}
```

---

## CNC-005: errgroup (Parallel Execution with Error Handling)

**When to use**: Parallel execution of tasks with shared error handling and cancellation.

```go
import "golang.org/x/sync/errgroup"

func processItems(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item // capture variable

        g.Go(func() error {
            return processItem(ctx, item)
        })
    }

    // Wait for all goroutines to complete
    // On first error, ctx is cancelled
    return g.Wait()
}
```

### CNC-005-A: errgroup with Parallelism Limiting

```go
func processItemsWithLimit(ctx context.Context, items []Item, limit int) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(limit) // Go 1.21+

    for _, item := range items {
        item := item

        g.Go(func() error {
            return processItem(ctx, item)
        })
    }

    return g.Wait()
}
```

---

## CNC-006: Rate Limiter (Frequency Limiting)

**When to use**: Limiting the frequency of operation execution (RPS).

```go
import "golang.org/x/time/rate"

// RateLimitedProcessor executes operations with frequency limiting
type RateLimitedProcessor struct {
    limiter *rate.Limiter
}

// NewRateLimitedProcessor creates a processor with the specified limit
// rps - operations per second, burst - maximum burst
func NewRateLimitedProcessor(rps float64, burst int) *RateLimitedProcessor {
    return &RateLimitedProcessor{
        limiter: rate.NewLimiter(rate.Limit(rps), burst),
    }
}

// Process executes an operation respecting the limit
func (p *RateLimitedProcessor) Process(ctx context.Context, fn func() error) error {
    if err := p.limiter.Wait(ctx); err != nil {
        return fmt.Errorf("rate limit wait: %w", err)
    }

    return fn()
}

// TryProcess attempts to execute an operation without waiting
func (p *RateLimitedProcessor) TryProcess(fn func() error) (bool, error) {
    if !p.limiter.Allow() {
        return false, nil
    }

    return true, fn()
}
```

---

## CNC-007: Circuit Breaker (Fault Protection)

**When to use**: Protection against cascading failures when calling external services.

```go
// State represents the state of a circuit breaker
type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
    mu               sync.RWMutex
    state            State
    failures         int
    successes        int
    maxFailures      int
    successThreshold int
    timeout          time.Duration
    lastFailure      time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures, successThreshold int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        state:            StateClosed,
        maxFailures:      maxFailures,
        successThreshold: successThreshold,
        timeout:          timeout,
    }
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
    if !cb.canExecute() {
        return ErrCircuitOpen
    }

    err := fn()
    cb.recordResult(err)

    return err
}

func (cb *CircuitBreaker) canExecute() bool {
    cb.mu.RLock()
    defer cb.mu.RUnlock()

    switch cb.state {
    case StateClosed:
        return true
    case StateOpen:
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.mu.RUnlock()
            cb.mu.Lock()
            cb.state = StateHalfOpen
            cb.mu.Unlock()
            cb.mu.RLock()
            return true
        }
        return false
    case StateHalfOpen:
        return true
    }

    return false
}

func (cb *CircuitBreaker) recordResult(err error) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if err != nil {
        cb.failures++
        cb.successes = 0
        cb.lastFailure = time.Now()

        if cb.failures >= cb.maxFailures {
            cb.state = StateOpen
        }
    } else {
        cb.successes++

        if cb.state == StateHalfOpen && cb.successes >= cb.successThreshold {
            cb.state = StateClosed
            cb.failures = 0
        }
    }
}

var ErrCircuitOpen = errors.New("circuit breaker is open")
```

---

## CNC-008: Graceful Shutdown

**When to use**: Proper termination with waiting for goroutines.

```go
// GracefulShutdown manages proper termination
type GracefulShutdown struct {
    wg       sync.WaitGroup
    ctx      context.Context
    cancel   context.CancelFunc
    timeout  time.Duration
}

// NewGracefulShutdown creates a new shutdown manager
func NewGracefulShutdown(timeout time.Duration) *GracefulShutdown {
    ctx, cancel := context.WithCancel(context.Background())
    return &GracefulShutdown{
        ctx:     ctx,
        cancel:  cancel,
        timeout: timeout,
    }
}

// Context returns a context for checking termination
func (gs *GracefulShutdown) Context() context.Context {
    return gs.ctx
}

// Add registers a goroutine
func (gs *GracefulShutdown) Add(delta int) {
    gs.wg.Add(delta)
}

// Done marks a goroutine as finished
func (gs *GracefulShutdown) Done() {
    gs.wg.Done()
}

// Shutdown initiates graceful shutdown
func (gs *GracefulShutdown) Shutdown() error {
    gs.cancel()

    done := make(chan struct{})
    go func() {
        gs.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-time.After(gs.timeout):
        return errors.New("shutdown timeout exceeded")
    }
}
```

---

## CNC-009: Pattern Selection Guide

| Scenario | Recommended Pattern |
|----------|----------------------|
| Processing N tasks with M workers | Worker Pool |
| Limiting concurrent operations | Semaphore |
| Parallelizing and collecting results | Fan-Out / Fan-In |
| Sequential multi-stage processing | Pipeline |
| Parallel execution with cancellation | errgroup |
| Limiting RPS | Rate Limiter |
| Protection against external service failures | Circuit Breaker |
| Proper termination | Graceful Shutdown |

---

## CNC-010: General Recommendations

1. **Always use context** for managing cancellation and timeouts
2. **Close channels on the sender side**, not the receiver
3. **Check ctx.Done()** in long-running operations
4. **Use defer** for resource cleanup (semaphores, mutexes)
5. **Document ownership** of channels (who creates, who closes)
6. **Avoid orphan goroutines** - each goroutine must have a way to exit

---

## CNC-011: References

- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync)
- [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)
