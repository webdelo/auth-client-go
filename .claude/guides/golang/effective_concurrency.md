# Effective Concurrency in Go

This document contains rules for effective concurrent programming in Go. Each rule has a unique identifier (EC-XXXX) for reference in code reviews and discussions.

> **Sources**: [go.dev/doc/effective_go](https://go.dev/doc/effective_go), [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md), [Go Concurrency Patterns](https://go.dev/blog/pipelines)

> **Related**: For ready-to-use code patterns, see [concurrency_patterns.md](../../guides/libs/concurrency_patterns.md)

---

## 1. Core Principles

### EC-0001: Share memory by communicating
**"Do not communicate by sharing memory; instead, share memory by communicating."**

Instead of protecting shared memory with mutexes, **pass values through channels**:

```go
// EC-0001: GOOD - communication via channels
func process(requests <-chan Request) {
    for req := range requests {
        result := handle(req)
        // handle result
    }
}

// EC-0001: LESS GOOD - mutex protection (use only when necessary)
var (
    mu    sync.Mutex
    state map[string]string
)
func process() {
    mu.Lock()
    state["key"] = "value"
    mu.Unlock()
}
```

### EC-0002: Prefer channels for coordination, mutex for state protection
- Use **channels** for coordination between goroutines
- Use **`sync.Mutex`** for simple shared-memory state protection

---

## 2. Goroutine Lifecycle

### EC-0010: Every goroutine must have a clear lifecycle
Each launched goroutine must have a way to exit:
- Context cancellation
- Channel close
- Done signal

```go
// EC-0010: CORRECT - goroutine has exit mechanism
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    select {
    case <-ctx.Done():
        return
    case msg := <-ch:
        process(msg)
    }
}()
```

### EC-0011: Always cancel contexts to signal shutdown
```go
// EC-0011: CORRECT
ctx, cancel := context.WithCancel(context.Background())
defer cancel()  // MUST call cancel!
```

### EC-0012: Avoid goroutine leaks
Goroutine leaks happen when a goroutine cannot exit. Each goroutine must:
- Have a way to exit (context, channel close, done signal)
- Be tracked if long-running (WaitGroup)

```go
// EC-0012: BAD - goroutine leak
go func() {
    for {  // infinite loop with no exit
        process()
    }
}()

// EC-0012: GOOD - goroutine can exit
go func() {
    for {
        select {
        case <-ctx.Done():
            return  // exit mechanism
        case <-time.After(time.Second):
            process()
        }
    }
}()
```

### EC-0013: Check ctx.Done() in long-running operations
```go
// EC-0013: CORRECT
select {
case <-ctx.Done():
    return ctx.Err()
default:
    // Continue operation
}
```

---

## 3. Channel Usage

### EC-0020: Unbuffered (0) or size 1 channels by default
```go
// EC-0020: GOOD - unbuffered for synchronization
ch := make(chan struct{})

// EC-0020: GOOD - size 1 for single item
ch := make(chan Result, 1)

// EC-0020: REQUIRES JUSTIFICATION - larger buffers
ch := make(chan Task, 100)  // needs comment explaining why
```

Larger buffers require explicit justification in code.

### EC-0021: Close channels on sender side only
```go
// EC-0021: CORRECT - sender closes
func produce(ch chan<- int) {
    for i := 0; i < 10; i++ {
        ch <- i
    }
    close(ch)  // sender closes
}

// EC-0021: WRONG - receiver closing
func consume(ch <-chan int) {
    for v := range ch {
        process(v)
    }
    // close(ch) - NEVER do this!
}
```

### EC-0022: Respect channel semantics
- Receiving from a closed channel returns zero value
- Sending to a closed channel panics
- Always check if channel is closed when receiving

```go
// EC-0022: CORRECT - check if channel closed
value, ok := <-ch
if !ok {
    // channel is closed
}
```

### EC-0023: Use buffered channels as semaphores
```go
// EC-0023: Semaphore pattern
sem := make(chan bool, 3)  // max 3 parallel
for i := 0; i < 100; i++ {
    go func() {
        sem <- true          // acquire slot
        defer func() { <-sem }()  // release slot
        process()
    }()
}
```

---

## 4. Mutex and Synchronization

### EC-0030: Zero-value mutex is ready to use
```go
// EC-0030: CORRECT
var mu sync.Mutex
mu.Lock()
defer mu.Unlock()

// EC-0030: UNNECESSARY
mu := new(sync.Mutex)
```

### EC-0031: Embedding mutex in unexported structs
```go
// EC-0031: GOOD - for unexported types
type counter struct {
    sync.Mutex
    value int
}

func (c *counter) Inc() {
    c.Lock()
    defer c.Unlock()
    c.value++
}

// EC-0031: GOOD - for exported types use private field
type Counter struct {
    mu    sync.Mutex
    value int
}

func (c *Counter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.value++
}
```

### EC-0032: Maximum 3 mutexes per struct
Having more than 3 mutexes in a struct indicates poor architecture. Consider restructuring.

**If protecting multiple fields separately:**
- Use named mutexes that indicate what they protect
- Example: `itemsMu sync.Mutex` to protect `items` field

### EC-0033: Always use defer with mutex unlock
```go
// EC-0033: CORRECT
mu.Lock()
defer mu.Unlock()
// operations

// EC-0033: RISKY - may not unlock on panic
mu.Lock()
// operations
mu.Unlock()
```

### EC-0034: Use RWMutex for read-heavy workloads
```go
// EC-0034: CORRECT - multiple readers, single writer
var (
    rwMu  sync.RWMutex
    cache map[string]string
)

func get(key string) string {
    rwMu.RLock()
    defer rwMu.RUnlock()
    return cache[key]
}

func set(key, value string) {
    rwMu.Lock()
    defer rwMu.Unlock()
    cache[key] = value
}
```

---

## 5. WaitGroup Usage

### EC-0040: Add before starting goroutine
```go
// EC-0040: CORRECT
var wg sync.WaitGroup
for i := 0; i < n; i++ {
    wg.Add(1)  // before go
    go func() {
        defer wg.Done()
        process()
    }()
}
wg.Wait()

// EC-0040: WRONG - race condition
var wg sync.WaitGroup
for i := 0; i < n; i++ {
    go func() {
        wg.Add(1)  // inside goroutine - too late!
        defer wg.Done()
        process()
    }()
}
wg.Wait()
```

### EC-0041: Use errgroup for error handling
```go
import "golang.org/x/sync/errgroup"

// EC-0041: CORRECT
func processItems(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item  // capture variable

        g.Go(func() error {
            return processItem(ctx, item)
        })
    }

    return g.Wait()  // returns first app_error
}
```

---

## 6. Context Propagation

### EC-0050: Pass context as first parameter
```go
// EC-0050: CORRECT
func DoSomething(ctx context.Context, data string) error {
    // ...
}
```

### EC-0051: Never store context in structs
```go
// EC-0051: WRONG
type Service struct {
    ctx context.Context  // don't do this!
}

// EC-0051: CORRECT - pass context to methods
func (s *Service) Process(ctx context.Context) error {
    // use ctx here
}
```

### EC-0052: Use context.Background() or context.TODO() as root
- `context.Background()` - for main execution path
- `context.TODO()` - when appropriate context is not yet clear
- **Never use `nil` as context**

### EC-0053: Set reasonable timeouts
```go
// EC-0053: CORRECT
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

result, err := operation(ctx)
```

### EC-0054: Handle context errors appropriately
```go
// EC-0054: Differentiate between cancellation and timeout
if err := ctx.Err(); err != nil {
    if errors.Is(err, context.Canceled) {
        // user-initiated cancellation
    } else if errors.Is(err, context.DeadlineExceeded) {
        // timeout
    }
}
```

### EC-0055: Always call cancel function
```go
// EC-0055: CORRECT - always defer cancel
ctx, cancel := context.WithCancel(parentCtx)
defer cancel()

// EC-0055: CORRECT - with timeout
ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
defer cancel()
```

Failing to call cancel leaks resources until the parent context is canceled.

### EC-0056: Use custom types for context keys
```go
// EC-0056: CORRECT - custom unexported type prevents collisions
type userIDKey struct{}

func WithUserID(ctx context.Context, userID string) context.Context {
    return context.WithValue(ctx, userIDKey{}, userID)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
    userID, ok := ctx.Value(userIDKey{}).(string)
    return userID, ok
}

// EC-0056: WRONG - string keys can collide
ctx = context.WithValue(ctx, "userID", userID)
```

---

## 7. Atomic Operations

### EC-0060: Use atomic for simple counters and flags
```go
import "sync/atomic"

// EC-0060: CORRECT
var started atomic.Bool

func Start() {
    if started.Swap(true) {
        return  // already started
    }
    // start logic
}
```

### EC-0061: Prefer atomic over mutex for single values
```go
// EC-0061: GOOD - atomic for counter
var counter atomic.Int64

func Inc() {
    counter.Add(1)
}

// EC-0061: OVERKILL - mutex for simple counter
var (
    mu      sync.Mutex
    counter int64
)

func Inc() {
    mu.Lock()
    counter++
    mu.Unlock()
}
```

### EC-0062: Use Swap for entry guards, not Store
When using atomic flag to prevent concurrent method entry, **use `Swap`** (returns previous value) instead of `Store` (returns nothing).

```go
// EC-0062: CORRECT - Swap returns previous value for check
var started atomic.Bool

func Start() {
    if started.Swap(true) {
        return  // was already true, another goroutine started first
    }
    // guaranteed single entry
    doStart()
}

// EC-0062: WRONG - Store + Load has race condition
var started atomic.Bool

func Start() {
    if started.Load() {  // RACE: another goroutine can pass this check
        return
    }
    started.Store(true)  // both goroutines may reach here!
    doStart()
}

// EC-0062: WRONG - CompareAndSwap is valid but Swap is simpler for bool
var started atomic.Bool

func Start() {
    if !started.CompareAndSwap(false, true) {
        return
    }
    doStart()
}
```

**Why Swap:**
- `Swap(true)` atomically sets value AND returns previous value
- If previous was `true` → another goroutine already started
- If previous was `false` → this goroutine is the first

### EC-0063: Don't mix atomic and mutex for same protection
For protecting the same value/entry point, use **either** atomic **or** mutex, never both.

```go
// EC-0063: WRONG - mixing atomic and mutex for same flag
type Service struct {
    mu      sync.Mutex
    started atomic.Bool
}

func (s *Service) Start() {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.started.Load() {  // redundant: mutex already protects
        return
    }
    s.started.Store(true)
    // start logic
}

// EC-0063: CORRECT - atomic alone (simpler, faster)
type Service struct {
    started atomic.Bool
}

func (s *Service) Start() {
    if s.started.Swap(true) {
        return
    }
    // start logic
}

// EC-0063: CORRECT - mutex alone (when protecting multiple fields)
type Service struct {
    mu      sync.Mutex
    started bool
    config  Config
}

func (s *Service) Start() {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.started {
        return
    }
    s.started = true
    s.config = loadConfig()
    // start logic
}
```

**Guidelines:**
- **Atomic** - for single flag/counter with no other state to protect
- **Mutex** - when multiple fields must be updated atomically together

### EC-0064: Maximum 1 mutex per method
A method should lock **at most one mutex**. Multiple mutex locks in a single method indicate architectural problems.

```go
// EC-0064: WRONG - multiple mutexes in one method (deadlock risk!)
func (s *Service) Process(data Data) {
    s.stateMu.Lock()
    defer s.stateMu.Unlock()

    s.cacheMu.Lock()        // BAD: second mutex
    defer s.cacheMu.Unlock()

    // process...
}

// EC-0064: WRONG - nested mutex acquisition
func (s *Service) Update() {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.other.mu.Lock()       // BAD: locking another object's mutex
    defer s.other.mu.Unlock()
}
```

**Problems with multiple mutexes:**
- **Deadlock risk** - if another method locks in different order
- **Hard to reason** - complex lock ordering rules
- **Performance** - holding multiple locks increases contention
- **Code smell** - method likely does too much

**Solutions:**

```go
// EC-0064: CORRECT - split into separate methods
func (s *Service) Process(data Data) {
    state := s.getState()      // locks stateMu internally
    cached := s.getCache(key)  // locks cacheMu internally

    result := compute(state, cached)

    s.saveResult(result)       // locks resultMu internally
}

func (s *Service) getState() State {
    s.stateMu.RLock()
    defer s.stateMu.RUnlock()
    return s.state
}

func (s *Service) getCache(key string) Value {
    s.cacheMu.RLock()
    defer s.cacheMu.RUnlock()
    return s.cache[key]
}
```

```go
// EC-0064: CORRECT - consolidate into single mutex if fields are related
type Service struct {
    mu    sync.RWMutex
    state State
    cache map[string]Value  // always accessed with state
}

func (s *Service) Process(data Data) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // access both state and cache under single lock
}
```

```go
// EC-0064: CORRECT - redesign to avoid shared state
type Service struct {
    processor *Processor  // owns its own mutex
    cache     *Cache      // owns its own mutex
}

func (s *Service) Process(data Data) {
    state := s.processor.GetState()  // Processor handles its locking
    cached := s.cache.Get(key)       // Cache handles its locking

    // no mutex in this method at all
}
```

**Rule of thumb:** If you need multiple mutexes in one method, restructure:
1. Split method into smaller methods, each with one lock
2. Consolidate related fields under single mutex
3. Extract components that manage their own synchronization

---

## 8. Worker Patterns

### EC-0070: Use fixed number of workers
```go
// EC-0070: CORRECT - fixed worker count
const numWorkers = 5
requests := make(chan Request)

for i := 0; i < numWorkers; i++ {
    go func() {
        for req := range requests {
            handle(req)
        }
    }()
}
```

### EC-0071: Limit concurrency with semaphores
```go
import "golang.org/x/sync/semaphore"

// EC-0071: CORRECT - use standard semaphore
func processWithLimit(ctx context.Context, items []Item, limit int) error {
    sem := semaphore.NewWeighted(int64(limit))
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item

        if err := sem.Acquire(ctx, 1); err != nil {
            return err
        }

        g.Go(func() error {
            defer sem.Release(1)
            return processItem(ctx, item)
        })
    }

    return g.Wait()
}
```

---

## 9. Graceful Shutdown

### EC-0080: Signal shutdown via context cancellation
```go
// EC-0080: CORRECT
ctx, cancel := context.WithCancel(context.Background())

// Start workers with context
go worker(ctx)

// On shutdown signal
cancel()
```

### EC-0081: Wait for all goroutines to complete
```go
// EC-0081: CORRECT - graceful shutdown pattern
type Server struct {
    wg     sync.WaitGroup
    ctx    context.Context
    cancel context.CancelFunc
}

func (s *Server) Start() {
    s.ctx, s.cancel = context.WithCancel(context.Background())

    s.wg.Add(1)
    go s.worker()
}

func (s *Server) Shutdown(timeout time.Duration) error {
    s.cancel()

    done := make(chan struct{})
    go func() {
        s.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        return errors.New("shutdown timeout")
    }
}

func (s *Server) worker() {
    defer s.wg.Done()
    for {
        select {
        case <-s.ctx.Done():
            return
        default:
            // work
        }
    }
}
```

### EC-0082: Handle OS signals for graceful shutdown
```go
// EC-0082: CORRECT
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigCh
    cancel()  // trigger shutdown
}()
```

---

## 10. Race Condition Prevention

### EC-0090: Use -race flag in tests
```bash
# EC-0090: CORRECT
go test -race ./...
```

### EC-0091: Capture loop variables in goroutines
```go
// EC-0091: CORRECT - capture variable
for i := 0; i < n; i++ {
    i := i  // capture
    go func() {
        process(i)
    }()
}

// EC-0091: WRONG - race condition
for i := 0; i < n; i++ {
    go func() {
        process(i)  // i changes!
    }()
}
```

### EC-0092: Never share state between goroutines without synchronization
Every access to shared state must be:
- Protected by mutex
- Through channels
- Using atomic operations

---

## 11. Testing Concurrent Code

### EC-0100: Use t.Parallel() for independent tests
```go
// EC-0100: CORRECT
func TestSomething(t *testing.T) {
    t.Parallel()

    tests := map[string]struct{...}{...}

    for name, tc := range tests {
        tc := tc  // capture!

        t.Run(name, func(t *testing.T) {
            t.Parallel()
            // test
        })
    }
}
```

### EC-0101: Don't use t.Parallel() with shared state
When NOT to use `t.Parallel()`:
- Tests with shared global variables
- Integration tests with DB
- Tests with file system

### EC-0102: Use synctest for time-dependent code (Go 1.24+)
```go
import "testing/synctest"

// EC-0102: CORRECT
synctest.Run(func() {
    synctest.Wait()  // wait for goroutines
    result := processor.ProcessWithTimeout(ctx, 5*time.Second)
    assert.NotNil(t, result)
})
```

---

## Summary - Key Principles

1. **Clear ownership** - every goroutine has a defined lifecycle
2. **Communicate via channels** - prefer channels over shared memory
3. **Context everywhere** - propagate context for cancellation
4. **Protect shared state** - mutex or atomic for shared data
5. **Graceful shutdown** - always provide clean exit mechanism
6. **Test with -race** - catch race conditions early

---

## References

- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Share Memory By Communicating](https://go.dev/blog/codelab-share)
- [Context](https://go.dev/blog/context)
- [golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync)
