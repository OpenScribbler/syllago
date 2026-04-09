# Python Web & API Patterns

Production patterns for FastAPI, Pydantic v2, SQLAlchemy 2.0, authentication, middleware, and deployment.

---

## FastAPI Patterns

### Project Structure
- `main.py` (app + router includes), `config.py` (pydantic-settings), `dependencies.py` (shared deps), `models/` (Pydantic schemas), `db/` (engine + ORM models), `routers/` (route handlers), `services/` (business logic).

### Dependency Injection
- Rule: Use `Annotated[T, Depends(func)]` type aliases. Yield dependencies for resource cleanup (try/finally). Override for testing via `app.dependency_overrides`.
- Class-based deps: `class Paginator` with `__init__` taking `Query()` params.
- Global deps: `FastAPI(dependencies=[Depends(verify_api_key)])`.
- Gotcha: Use `Depends(get_db)` not `Depends(get_db())`.

### APIRouter
- Rule: `APIRouter(prefix="/users", tags=["users"])`. Include in main app with `app.include_router(router)`. Version with prefix: `prefix="/api/v1"`.

### Response Models
- Rule: Separate input/output models. `UserCreate` (with password), `UserResponse` (without). Set `model_config = {"from_attributes": True}` for ORM objects.
- Gotcha: Never return DB models directly (leaks internal fields).

### Lifespan Events
- Rule: Use `@asynccontextmanager async def lifespan(app)` with yield. Replace deprecated `@app.on_event()`.

### Settings
- Rule: Use `pydantic_settings.BaseSettings` + `@lru_cache` factory + `Annotated[Settings, Depends(get_settings)]` for testable config.

### Pagination
- Rule: Generic `PaginatedResponse[T]` model with `Paginator` dependency class.

---

## Pydantic v2

### Model Configuration
- `from_attributes=True`: Read from ORM objects. `str_strip_whitespace=True`. `populate_by_name=True`.
- Gotcha: Use `model_config = ConfigDict(...)` not old `class Config:` syntax.

### Validators
- `@field_validator("field")` + `@classmethod`: Validate single field.
- `@model_validator(mode="after")`: Cross-field validation (e.g., password confirmation).
- Modes: `before` (raw input), `after` (validated), `wrap` (wraps default), `plain` (replaces default).

### Computed Fields
- Rule: `@computed_field @property` -- appears in serialization and schema.

### Discriminated Unions
- Rule: `Annotated[Union[CatAction, DogAction], Field(discriminator="action")]` for polymorphic models.

### Serialization Control
- `model_dump(exclude_none=True, exclude_unset=True, by_alias=True, exclude={"internal_id"})`.
- `@field_serializer("field")` for custom serialization.

---

## SQLAlchemy 2.0

### Setup
- Sync: `create_engine(url, pool_size=5, max_overflow=10, pool_pre_ping=True)`.
- Async: `create_async_engine(url)` + `async_sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)`.

### Declarative Models (2.0 Style)
- Rule: Use `Mapped[type]` + `mapped_column()` syntax. Use `TimestampMixin` for created_at/updated_at with `server_default=func.now()`.

### New-Style Queries
- Rule: Use `select(User).where(...)` not legacy `session.query(User)`.
- `session.scalars(stmt).all()` for multiple, `session.scalar(stmt)` for single.
- Eager loading: `selectinload` for collections, `joinedload` for single objects.

### Alembic Migrations
- `alembic revision --autogenerate -m "message"` then `alembic upgrade head`.
- Gotcha: Don't make schema changes without Alembic -- loses migration history.

### Common Mistakes
- Not setting `expire_on_commit=False` -- lazy load errors after commit.
- Not using `pool_pre_ping=True` -- stale connections in production.
- Missing eager loading -- N+1 queries.

---

## Authentication

### JWT with OAuth2
- Rule: Use `PyJWT` + `pwdlib` + `OAuth2PasswordBearer(tokenUrl="token")`. Create `get_current_user` dependency that decodes token and returns User.
- Type alias: `CurrentUser = Annotated[User, Depends(get_current_user)]`.
- Set token expiration. Use `secrets` module for SECRET_KEY generation.

### OAuth2 Scopes (RBAC)
- Rule: Define scopes in `OAuth2PasswordBearer(scopes={...})`. Use `Security(get_current_user, scopes=["users:read"])` on endpoints.

### API Key Auth
- Rule: Support both header (`X-API-Key`) and query param. Use `APIKeyHeader` + `APIKeyQuery` with `Security()`.

### Common Mistakes
- Hardcoded secrets. No token expiration. HS256 in multi-service (use RS256/ES256). Not rate-limiting login.

---

## Middleware

### Custom Middleware
- Rule: `@app.middleware("http") async def mw(request, call_next)`. Last added = outermost (processes request first).
- Common: request timing, request ID propagation, logging, rate limiting.

### CORS
- Rule: If `allow_credentials=True`, cannot use `["*"]` for `allow_origins` -- must specify explicitly.

### Rate Limiting
- Rule: In-memory `defaultdict(list)` for single process. Use Redis for multi-process deployments.

### Common Mistakes
- Blocking calls in middleware. Not catching exceptions (bypass other middleware). Modifying response body after `call_next` (already streamed).

---

## Background Tasks

### FastAPI BackgroundTasks
- Rule: For lightweight post-response tasks (email, logging). `background_tasks.add_task(fn, *args)`.

### Celery
- Rule: For CPU-intensive, distributed, scheduled, retryable tasks. Use `@celery_app.task(bind=True, max_retries=3)`.
- Workflows: `chain()` for sequential, `group()` for parallel, `chord()` for fan-out + aggregate.
- Beat: `celery_app.conf.beat_schedule` for periodic tasks.
- Gotcha: Pass IDs to tasks, not ORM objects (must be picklable).

---

## Error Handling

### Consistent Error Response
- Rule: Define `AppException` base with `error`, `message`, `status_code`. Subclass for `NotFoundError`, `ConflictError`, etc.
- Register `@app.exception_handler(AppException)` returning `JSONResponse` with consistent format.
- Handle `RequestValidationError` separately with field-level detail.
- Always log unhandled exceptions server-side, return generic message to client.

---

## Deployment

### Uvicorn + Gunicorn
- Dev: `uvicorn app.main:app --reload`.
- Prod: `gunicorn app.main:app --worker-class uvicorn.workers.UvicornWorker --workers N`.
- Worker count: `(2 * CPU_CORES) + 1`.

### Docker
- Rule: Install deps first (cache layer), copy code second. Non-root user. `--no-cache-dir` with pip.
- Use uv for faster builds: `COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv`.

### Health Checks
- Rule: `/healthz` (liveness -- is process running), `/readyz` (readiness -- can serve requests, checks DB).

---

## Modern Tooling

### uv Package Manager
- Rule: 10-100x faster than pip. `uv init`, `uv add fastapi`, `uv sync`, `uv run uvicorn ...`.
- Lock file: `uv.lock`. Python management: `uv python pin 3.12`.

### Ruff
- Rule: Replaces flake8, isort, black. `ruff check .` for lint, `ruff format .` for formatting.
- Configure in `pyproject.toml` `[tool.ruff.lint]` with `select = ["E", "F", "I", "N", "UP", "B", "S", "RUF"]`.

### Type Checking
- pyright: Faster, better FastAPI inference. Use for FastAPI projects.
- mypy: Plugin system, larger community. Use if existing mypy config.

### Pre-commit
- Rule: `ruff-pre-commit` for lint+format, `pre-commit-hooks` for trailing whitespace and secrets detection. Pin versions.
