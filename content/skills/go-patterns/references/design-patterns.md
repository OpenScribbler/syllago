# Go Design Patterns

Production-ready design patterns for Go services.

---

## Interface Design

### Accept Interfaces, Return Structs
- Rule: Accept interfaces in function parameters (enables mocking), return concrete types (avoids boxing, gives full API).

### Keep Interfaces Small (1-3 methods)
- Rule: Define interfaces at the consumer (not provider). Small interfaces are easier to implement, mock, and compose.
- Compose larger interfaces from smaller ones via embedding (`ReadWriter` = `Reader` + `Writer`).

### Define Interfaces at Point of Use
- Rule: The consumer package defines only the methods it needs. The provider's concrete type satisfies it implicitly.
- Gotcha: Don't define interfaces in the package that implements them.

## Functional Options

### Basic Pattern
- Rule: Use `type Option func(*T)` + `New(opts ...Option)` for public APIs with many optional parameters.

```go
type Option func(*Server)
func WithPort(port int) Option { return func(s *Server) { s.port = port } }

func NewServer(opts ...Option) *Server {
    s := &Server{port: 8080} // defaults
    for _, opt := range opts { opt(s) }
    return s
}
```

### Validated Options
- Rule: Use `type Option func(*T) error` when options need validation. `New` returns `(*T, error)`.

### Config Struct Alternative
- Rule: For internal services or config-file-driven apps, prefer a config struct with `WithDefaults()` method over functional options.

## Package Design

### Layout Guidelines
- **Flat** (<15 files): All files in root package. Don't create packages prematurely.
- **cmd/internal** (multiple binaries): `cmd/server/main.go`, `cmd/cli/main.go`, shared code in `internal/`.
- **Domain root**: Domain types at root (`user/user.go`), implementations in subdirs (`user/postgres/store.go`).

### Naming
- Rule: Short, lowercase, no underscores. Types don't repeat package name (`list.List` not `list.ListStruct`).
- Avoid: `utils`, `helpers`, `common`, `models` packages.

## Dependency Injection

### Constructor Injection
- Rule: Accept interface dependencies in constructor, return concrete type. Wire in `main()`.

```go
func NewUserService(repo UserRepository, logger *slog.Logger) *UserService {
    return &UserService{repo: repo, logger: logger}
}
```

- Gotcha: Most Go projects never need a DI framework. Explicit wiring in `main()` is sufficient.

### Interface-Based Testing
- Rule: Create mock structs with function fields matching the interface. Override per-test.

```go
type mockRepo struct {
    getFn func(ctx context.Context, id string) (*User, error)
}
func (m *mockRepo) Get(ctx context.Context, id string) (*User, error) {
    return m.getFn(ctx, id)
}
```

## Middleware

### HTTP Middleware Chain
- Rule: `type Middleware func(http.Handler) http.Handler`. Chain in reverse order.

```go
func Chain(h http.Handler, mws ...Middleware) http.Handler {
    for i := len(mws) - 1; i >= 0; i-- { h = mws[i](h) }
    return h
}
```

### gRPC Interceptor
- Rule: Use `grpc.ChainUnaryInterceptor(...)` to compose interceptors.

### Decorator Pattern
- Rule: Wrap an interface implementation with another implementing the same interface. Compose: `logging -> caching -> postgres`.

## Repository Pattern

### Interface + Implementation
- Rule: Define CRUD interface in domain package. Implement in `postgres/`, `mock/` subdirs.
- Gotcha: Use `sql.ErrNoRows` to return domain-specific `NotFoundError`.

### Transaction Handling
- Rule: Use `RunInTx(ctx, func(tx *sql.Tx) error)` helper. Rollback on error, commit on success.

```go
func (s *Store) RunInTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil { return err }
    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }
    return tx.Commit()
}
```

## Graceful Shutdown

### HTTP Server
- Rule: Use `signal.NotifyContext` + `srv.Shutdown(ctx)` with timeout.

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
// start server in goroutine...
<-ctx.Done()
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(shutdownCtx)
```

### Multiple Components
- Rule: Use `errgroup.WithContext` to manage multiple goroutines (server, workers). One failure cancels all.

### Health Check During Shutdown
- Rule: Mark not-ready before shutdown, sleep for LB drain (5s), then shutdown.

## Context Patterns

### Request-Scoped Values
- Rule: Use unexported `type contextKey int` to avoid key collisions. Provide `With*` and `*From` accessor functions.

## Error Handling

### Sentinel Errors and Custom Types
- Rule: Use `var ErrX = errors.New(...)` for well-known conditions. Use custom types when callers need structured data.
- Check with `errors.Is` (sentinels) and `errors.As` (typed errors).

### HTTP Error Handler
- Rule: Define `type appHandler func(w, r) error`. Map domain errors to HTTP status codes in a central `ServeHTTP`.

```go
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if err := fn(w, r); err != nil {
        // Map errors.Is/As to status codes
    }
}
```
