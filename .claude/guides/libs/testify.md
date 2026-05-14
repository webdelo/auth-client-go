---
prefix: TFY
name: Testify
packages:
  - github.com/stretchr/testify/assert
  - github.com/stretchr/testify/require
  - github.com/stretchr/testify/mock
  - github.com/stretchr/testify/suite
triggers:
  - testify
  - assert
  - require
  - mock
  - suite
description: "assert/require for test value checks, mock for interface stubs with call verification, suite for test lifecycle (setup/teardown)"
---

# Testing with Testify

## TFY-001: Overview

Testify is a set of packages for testing Go code, providing readable assertions, mocking, and test suites.

**Key advantages:**
- Readable error messages
- Two assertion modes: `assert` (continues test) and `require` (stops test)
- Powerful mocking with call verification
- Test suites with lifecycle hooks (setup/teardown)
- 634,000+ dependents — the most popular testing library for Go

**Documentation:**
- GitHub: https://github.com/stretchr/testify
- pkg.go.dev: https://pkg.go.dev/github.com/stretchr/testify
- Linter for testify: https://github.com/Antonboom/testifylint

---

## TFY-002: Core Packages

| Package | Purpose | When to use |
|---------|---------|-------------|
| `assert` | Non-fatal assertions | Additional checks, test continues |
| `require` | Fatal assertions | Critical checks, test stops |
| `mock` | Creating mock objects | Replacing dependencies in unit tests |
| `suite` | Test suites with lifecycle | Grouping tests with common setup/teardown |

---

## TFY-003: assert Package

### TFY-004: Basic Functions

```go
import "github.com/stretchr/testify/assert"

func TestExample(t *testing.T) {
    // Equality
    assert.Equal(t, expected, actual, "optional message")
    assert.NotEqual(t, unexpected, actual)

    // Nil checks
    assert.Nil(t, object)
    assert.NotNil(t, object)

    // Boolean
    assert.True(t, condition)
    assert.False(t, condition)

    // Errors
    assert.NoError(t, err)
    assert.Error(t, err)
    assert.ErrorIs(t, err, targetErr)
    assert.ErrorContains(t, err, "substring")

    // Collections
    assert.Len(t, slice, expectedLength)
    assert.Contains(t, slice, element)
    assert.Empty(t, slice)
    assert.NotEmpty(t, slice)

    // Strings
    assert.Contains(t, str, substring)
    assert.Regexp(t, regexp, str)
}
```

### TFY-005: Object API (Recommended)

```go
func TestWithObjectAPI(t *testing.T) {
    assert := assert.New(t)

    // Without repeating t in each call
    assert.Equal(expected, actual)
    assert.NotNil(result)
    assert.NoError(err)
}
```

### TFY-006: Conditional Checks

```go
func TestConditionalChecks(t *testing.T) {
    // assert returns bool — can be used in conditions
    if assert.NotNil(t, result) {
        assert.Equal(t, "expected", result.Field)
    }
}
```

---

## TFY-007: require Package

**Identical API to assert, but stops the test on failure.**

```go
import "github.com/stretchr/testify/require"

func TestCriticalChecks(t *testing.T) {
    result, err := SomeOperation()

    // Test STOPS here if err != nil
    require.NoError(t, err)
    require.NotNil(t, result)

    // These lines execute only if previous checks passed
    assert.Equal(t, "value", result.Field)
}
```

### TFY-008: require vs assert

| Situation | Package | Reason |
|-----------|---------|--------|
| Error when fetching data | `require` | Further checks are meaningless |
| Nil check before accessing fields | `require` | Avoid panic |
| Checking operation result | `require` | Critical check |
| Checking result properties | `assert` | Test can continue |
| Checking side-effects | `assert` | Collect all errors |

**IMPORTANT:** Do not use `require` in goroutines — it will panic instead of stopping the test.

---

## TFY-009: mock Package

### TFY-010: Creating a Mock Object

```go
import "github.com/stretchr/testify/mock"

// Interface to mock
type UserRepository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, user *User) error
}

// Mock implementation
type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserRepository) Save(ctx context.Context, user *User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}
```

### TFY-011: Setting Up Expectations

```go
func TestWithMock(t *testing.T) {
    mockRepo := new(MockUserRepository)

    // Set up expected call
    expectedUser := &User{ID: "123", Name: "John"}
    mockRepo.On("FindByID", mock.Anything, "123").Return(expectedUser, nil).Once()

    // Call the tested code
    service := NewUserService(mockRepo)
    user, err := service.GetUser(context.Background(), "123")

    // Assertions
    require.NoError(t, err)
    assert.Equal(t, expectedUser, user)

    // Verify all expectations were met
    mockRepo.AssertExpectations(t)
}
```

### TFY-012: Argument Placeholders

```go
// mock.Anything — any value (use for context, logger)
mockRepo.On("FindByID", mock.Anything, "123").Return(user, nil)

// mock.AnythingOfType — any value of specified type
mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*User")).Return(nil)

// mock.MatchedBy — custom matcher
mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(u *User) bool {
    return u.Name != ""
})).Return(nil)
```

### TFY-013: Specifying Call Count

```go
mockRepo.On("FindByID", mock.Anything, "123").Return(user, nil).Once()    // Exactly 1 time
mockRepo.On("FindByID", mock.Anything, "456").Return(nil, nil).Times(2)   // Exactly 2 times
mockRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Maybe()     // 0 or more times
```

### TFY-014: Resetting Expectations

```go
mockCall := mockRepo.On("FindByID", mock.Anything, "123").Return(user, nil).Once()

// After first test
mockRepo.AssertExpectations(t)
mockCall.Unset()

// New expectation
mockRepo.On("FindByID", mock.Anything, "123").Return(nil, errors.New("not found")).Once()
```

---

## TFY-015: suite Package

### TFY-016: Basic Structure

```go
import (
    "testing"
    "github.com/stretchr/testify/suite"
)

type UserServiceTestSuite struct {
    suite.Suite

    // Shared state
    mockRepo *MockUserRepository
    service  *UserService
}

// Executes before EACH test
func (s *UserServiceTestSuite) SetupTest() {
    s.mockRepo = new(MockUserRepository)
    s.service = NewUserService(s.mockRepo)
}

// Executes after EACH test
func (s *UserServiceTestSuite) TearDownTest() {
    s.mockRepo.AssertExpectations(s.T())
}

// Test method
func (s *UserServiceTestSuite) TestGetUser_Success() {
    user := &User{ID: "123", Name: "John"}
    s.mockRepo.On("FindByID", mock.Anything, "123").Return(user, nil).Once()

    result, err := s.service.GetUser(context.Background(), "123")

    s.NoError(err)
    s.Equal(user, result)
}

func (s *UserServiceTestSuite) TestGetUser_NotFound() {
    s.mockRepo.On("FindByID", mock.Anything, "123").Return(nil, ErrNotFound).Once()

    result, err := s.service.GetUser(context.Background(), "123")

    s.Error(err)
    s.Nil(result)
}

// Entry point for running the suite
func TestUserServiceSuite(t *testing.T) {
    suite.Run(t, new(UserServiceTestSuite))
}
```

### TFY-017: Lifecycle Methods

```go
type MyTestSuite struct {
    suite.Suite
    db *sql.DB
}

// Executes once BEFORE all tests
func (s *MyTestSuite) SetupSuite() {
    s.db = setupTestDatabase()
}

// Executes once AFTER all tests
func (s *MyTestSuite) TearDownSuite() {
    s.db.Close()
}

// Executes BEFORE each test
func (s *MyTestSuite) SetupTest() {
    s.db.Exec("TRUNCATE users")
}

// Executes AFTER each test
func (s *MyTestSuite) TearDownTest() {
    // cleanup
}
```

**IMPORTANT:** Suite does not support `t.Parallel()`.

---

## TFY-018: Integration with mockery

It is recommended to use [mockery](https://vektra.github.io/mockery/latest/) for autogenerating mocks.

See [mockery.md](mockery.md) for details.

Generated mocks use the Expecter API:

```go
func TestWithMockery(t *testing.T) {
    mockRepo := mocks.NewMockUserRepository(t)

    // Expecter API — type-safe
    mockRepo.EXPECT().
        FindByID(mock.Anything, "123").
        Return(&User{ID: "123"}, nil).
        Once()

    // ...
}
```

---

## TFY-019: Table-Driven Tests with Testify

```go
func TestProcessor_Process(t *testing.T) {
    t.Parallel()

    tests := map[string]struct {
        args         struct{ orderNr string }
        prepareMocks func(*MockRepository)
        want         func(*testing.T, *Result, error)
    }{
        "happy_path": {
            args: struct{ orderNr string }{orderNr: "ORD123"},
            prepareMocks: func(repo *MockRepository) {
                repo.On("Find", "ORD123").Return(&Order{Nr: "ORD123"}, nil).Once()
            },
            want: func(t *testing.T, result *Result, err error) {
                require.NoError(t, err)
                assert.Equal(t, "processed", result.Status)
            },
        },
        "return_error_when_repo_fails": {
            args: struct{ orderNr string }{orderNr: "ORD456"},
            prepareMocks: func(repo *MockRepository) {
                repo.On("Find", "ORD456").Return(nil, errors.New("db error")).Once()
            },
            want: func(t *testing.T, result *Result, err error) {
                require.Error(t, err)
                assert.Contains(t, err.Error(), "db error")
            },
        },
    }

    for name, tc := range tests {
        tc := tc // capture for parallel

        t.Run(name, func(t *testing.T) {
            t.Parallel()

            // New mocks for each test
            mockRepo := new(MockRepository)
            tc.prepareMocks(mockRepo)

            processor := NewProcessor(mockRepo)
            result, err := processor.Process(tc.args.orderNr)

            tc.want(t, result, err)
            mockRepo.AssertExpectations(t)
        })
    }
}
```

---

## TFY-020: Best Practices

### TFY-021: Use require for Critical Checks

```go
// GOOD
result, err := operation()
require.NoError(t, err)
require.NotNil(t, result)
assert.Equal(t, "value", result.Field)

// BAD — panic if result == nil
result, err := operation()
assert.NoError(t, err)
assert.Equal(t, "value", result.Field)  // panic!
```

### TFY-022: Always Specify Call Count for Mocks

```go
// GOOD
mockRepo.On("Find", "123").Return(user, nil).Once()

// BAD — not controlling call count
mockRepo.On("Find", "123").Return(user, nil)
```

### TFY-023: Verify Expectations at Test End

```go
func TestSomething(t *testing.T) {
    mockRepo := new(MockRepository)
    mockRepo.On("Find", "123").Return(user, nil).Once()

    // ... test code ...

    mockRepo.AssertExpectations(t)  // REQUIRED
}
```

### TFY-024: Don't Use mock.Anything for Business Data

```go
// GOOD — verify exact value
mockRepo.On("Find", "ORD123").Return(order, nil).Once()

// ACCEPTABLE — for context, logger
mockRepo.On("Find", mock.Anything, "ORD123").Return(order, nil).Once()

// BAD — not verifying argument
mockRepo.On("Find", mock.Anything).Return(order, nil).Once()
```

### TFY-025: Use Object API for Readability

```go
// GOOD
func TestSomething(t *testing.T) {
    assert := assert.New(t)
    require := require.New(t)

    require.NoError(err)
    assert.Equal(expected, actual)
}
```

### TFY-026: Use testifylint

Add to `.golangci.yaml`:

```yaml
linters:
  enable:
    - testifylint

linters-settings:
  testifylint:
    enable-all: true
```

---

## TFY-027: Common Mistakes

### TFY-028: require in Goroutine

```go
// WRONG — will panic instead of stopping test
go func() {
    require.NoError(t, err)  // PANIC!
}()

// CORRECT — use assert or channel
errChan := make(chan error, 1)
go func() {
    if err != nil {
        errChan <- err
        return
    }
    errChan <- nil
}()
require.NoError(t, <-errChan)
```

### TFY-029: Shared Mocks Between Parallel Tests

```go
// WRONG
mockRepo := new(MockRepository)  // Shared!

for name, tc := range tests {
    t.Run(name, func(t *testing.T) {
        t.Parallel()
        tc.prepareMocks(mockRepo)  // Race condition!
    })
}

// CORRECT
for name, tc := range tests {
    t.Run(name, func(t *testing.T) {
        t.Parallel()
        mockRepo := new(MockRepository)  // New for each test
        tc.prepareMocks(mockRepo)
    })
}
```

### TFY-030: Missing Capture for Parallel

```go
// WRONG
for name, tc := range tests {
    t.Run(name, func(t *testing.T) {
        t.Parallel()
        // tc can change before execution!
    })
}

// CORRECT
for name, tc := range tests {
    tc := tc  // Capture!
    t.Run(name, func(t *testing.T) {
        t.Parallel()
        // tc is safe
    })
}
```

---

## TFY-031: Useful Resources

- [testify GitHub](https://github.com/stretchr/testify) — source code and examples
- [mockery](https://vektra.github.io/mockery/latest/) — mock generator
- [testifylint](https://github.com/Antonboom/testifylint) — linter for testify
- [Unit Test Standards](../guides/golang/unit_tests_standards.md) — test writing standards

---

## TFY-032: Related Documents

- [Unit Test Standards](../guides/golang/unit_tests_standards.md)
- [Mockery Configuration](mockery.md)
