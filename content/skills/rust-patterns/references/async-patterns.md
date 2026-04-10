# Rust Async Patterns

## Runtime Setup

- Rule: Use `#[tokio::main]` for applications; use `Runtime::new()` + `block_on()` when you need manual runtime control
- Rule: Configure worker threads with `Builder::new_multi_thread().worker_threads(N)` for production deployments

---

## Task Spawning

- Rule: Use `tokio::spawn()` for fire-and-forget background tasks -- returns `JoinHandle<T>` for optional result retrieval
- Rule: Track `JoinHandle`s and `.await` them to catch panics and errors -- unjoined tasks silently drop errors
- Gotcha: Spawned tasks must be `Send + 'static` -- they cannot borrow local data

---

## Concurrent Execution

### join! and try_join!
- Rule: Use `tokio::join!(a, b, c)` to run futures concurrently and wait for all; use `tokio::try_join!` for early return on first error
- Gotcha: `join!` runs futures concurrently on the SAME task (not parallel) -- for true parallelism, `spawn` each then `join!` the handles

### select!
- Rule: Use `tokio::select!` to race futures -- first to complete wins, others are cancelled
- Rule: Common pattern: `select!` between work future and timeout/shutdown signal

---

## Channels

### Channel Selection

| Channel | Pattern | Use When |
|---------|---------|----------|
| `mpsc::channel(N)` | Multiple producer, single consumer | Task-to-task communication with backpressure |
| `oneshot::channel()` | Single value, single use | Request-response between tasks |
| `broadcast::channel(N)` | Multiple consumers | Pub/sub, event broadcasting |
| `watch::channel(init)` | Latest-value only | Config updates, state broadcasting |

- Rule: Always use bounded channels -- unbounded channels can exhaust memory under load
- Gotcha: `broadcast` receivers created after a `send()` miss that message; `watch` always sees the latest value

---

## Concurrency Limiting

Use `Semaphore` to cap concurrent operations:

```rust
let sem = Arc::new(Semaphore::new(10));
for url in urls {
    let permit = sem.clone().acquire_owned().await?;
    tokio::spawn(async move {
        let result = fetch(&url).await;
        drop(permit); // release when done
        result
    });
}
```

---

## Graceful Shutdown

Combine `broadcast` channel with `tokio::select!` for clean shutdown:

```rust
let (shutdown_tx, _) = broadcast::channel::<()>(1);
let mut shutdown_rx = shutdown_tx.subscribe();

tokio::spawn(async move {
    loop {
        tokio::select! {
            _ = shutdown_rx.recv() => break,
            _ = do_work() => {}
        }
    }
});

signal::ctrl_c().await?;
let _ = shutdown_tx.send(());
```

---

## Timeout

- Rule: Wrap any future with `tokio::time::timeout(duration, future)` -- returns `Err(Elapsed)` on timeout
- Rule: For request-level timeouts, prefer `tokio::time::timeout` over library-specific timeout configs for consistency

---

## Send and Sync

| Type | Send | Sync | Notes |
|------|------|------|-------|
| `Arc<T>` | If T: Send+Sync | If T: Send+Sync | Thread-safe sharing |
| `Rc<T>` | No | No | Single-thread only |
| `Mutex<T>` | If T: Send | Yes | Locked access |
| `RwLock<T>` | If T: Send | If T: Send+Sync | Read/write locking |
| `Cell<T>` | If T: Send | No | Interior mutability |
| `RefCell<T>` | If T: Send | No | Runtime borrow check |

- Rule: Most types are auto `Send + Sync`; only `unsafe impl` when wrapping raw pointers you've verified are safe
- Gotcha: `Rc` in an async task causes compile error -- use `Arc` instead

---

## Atomics

- Rule: Use `AtomicUsize`/`AtomicBool` for simple counters and flags instead of `Mutex`
- Rule: Use `Ordering::Relaxed` for standalone counters; `SeqCst` when unsure; `Acquire`/`Release` for paired read/write synchronization

---

## Data Parallelism

- Rule: Use `rayon::par_iter()` for CPU-bound data parallelism (see performance.md)
