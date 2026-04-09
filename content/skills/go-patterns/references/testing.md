# Go Testing Patterns

Go-specific testing patterns, mocking, and best practices.

---

## Table-Driven Tests

- Rule: Use named struct slices with `t.Run(tt.name, ...)` for parameterized tests. Include `wantErr bool` field for error cases.
- Gotcha: Use `t.Fatalf` inside subtests only. In a loop without subtests, `t.Fatal` stops ALL remaining cases.

```go
tests := []struct {
    name    string
    input   string
    want    int
    wantErr bool
}{
    {name: "valid", input: "42", want: 42},
    {name: "invalid", input: "abc", wantErr: true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := ParseValue(tt.input)
        if (err != nil) != tt.wantErr {
            t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            return
        }
        if !tt.wantErr && got != tt.want {
            t.Errorf("got %v, want %v", got, tt.want)
        }
    })
}
```

## Testify Assertions

- `assert.*`: continues on failure. `require.*`: stops test (fatal).
- Key functions: `Equal`, `NoError`, `Error`, `ErrorIs`, `ErrorAs`, `Contains`, `Len`, `Nil`, `NotNil`.

## Mocking

### Interface-Based (Manual)
- Rule: Define mock struct with function fields matching the interface. Override per-test.

```go
type mockRepo struct {
    getUserFn func(ctx context.Context, id string) (*User, error)
}
func (m *mockRepo) GetUser(ctx context.Context, id string) (*User, error) {
    return m.getUserFn(ctx, id)
}
```

### mockery (Generated)
```bash
mockery --name=UserRepository --output=mocks
```
- Use `mock.EXPECT().Method(mock.Anything, args).Return(val, nil)`.

## HTTP Testing

### Handler Testing
- Rule: Use `httptest.NewRequest` + `httptest.NewRecorder`. Assert status code and decoded body.

```go
req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
rec := httptest.NewRecorder()
handler.ServeHTTP(rec, req)
assert.Equal(t, http.StatusOK, rec.Code)
```

### Client Testing
- Rule: Use `httptest.NewServer` with custom handler. Pass `server.URL` to client under test.

## Test Helpers

### t.Helper()
- Rule: Always call `t.Helper()` in test helper functions. Makes failure messages point to the test, not the helper.

### Fixtures
- Rule: Store test data in `testdata/` directory. Use `os.ReadFile(filepath.Join("testdata", name))`.
- Tip: Generic `LoadJSONFixture[T](t, name)` reduces boilerplate.

### Test Builders
- Rule: Use builder pattern for complex test objects with sensible defaults. `NewUserBuilder().WithID("x").Inactive().Build()`.

## Context and Timeouts

- Rule: Use `context.WithTimeout` in tests to prevent hangs. Test cancellation with `context.WithCancel`.
- Assert: `errors.Is(err, context.Canceled)` or `context.DeadlineExceeded`.

## Race Detection

```bash
go-dev race ./...
```

- Rule: Always run with race detector in CI. Test concurrent access patterns explicitly with goroutines + `sync.WaitGroup`.

## Benchmarks

```go
func BenchmarkX(b *testing.B) {
    input := setup()
    b.ResetTimer()
    for i := 0; i < b.N; i++ { process(input) }
}
```

- Parallel: `b.RunParallel(func(pb *testing.PB) { for pb.Next() { ... } })`.

```bash
go-dev bench ./...                          # all benchmarks
go-dev bench-run BenchmarkX ./...           # specific, 5s runtime
```

## Test Organization

```
pkg/user/
  service.go
  service_test.go       # unit tests (same package)
testdata/               # fixtures
testutil/               # shared helpers
```

### Build Tags for Integration Tests
```go
//go:build integration

func TestDB(t *testing.T) {
    if testing.Short() { t.Skip("skipping integration test") }
    // ...
}
```

```bash
go-dev test-short ./...                # skip integration
go-dev test-tags integration ./...     # include integration
```
