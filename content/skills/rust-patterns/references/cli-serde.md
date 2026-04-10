# CLI and Serialization Patterns

## Clap CLI

### Basic Structure
- Rule: Use `#[derive(Parser)]` on a struct for argument parsing; use `#[derive(Subcommand)]` enum for subcommands
- Rule: Use `#[arg(short, long)]` for flags; `#[arg(default_value_t = X)]` for defaults
- Rule: Use `#[arg(env = "VAR")]` for environment variable fallback

### Clap Attributes

| Attribute | Purpose |
|-----------|---------|
| `#[arg(short, long)]` | Enable `-f` and `--flag` |
| `#[arg(default_value_t = X)]` | Default value |
| `#[arg(env = "VAR")]` | Env var fallback |
| `#[arg(value_enum)]` | Enum from string |
| `#[arg(value_parser = fn)]` | Custom validation |
| `#[command(subcommand)]` | Subcommands |

### Value Validation
- Rule: Use `#[derive(ValueEnum)]` for enum arguments that map from strings
- Rule: Use `value_parser = clap::value_parser!(u16).range(1024..=65535)` for numeric range validation
- Rule: Write custom validators as `fn(&str) -> Result<T, String>` and pass to `value_parser`

---

## Tracing

### Setup
- Rule: Use `tracing` + `tracing-subscriber` for structured logging; initialize with `tracing_subscriber::registry().with(fmt::layer()).with(EnvFilter::from_default_env()).init()`
- Rule: Use `RUST_LOG=module=level` env var for runtime log filtering

### Structured Logging
- Rule: Use `#[instrument]` macro on functions for automatic span creation with arguments as fields
- Rule: Use `#[instrument(skip(password))]` to exclude sensitive fields from traces
- Rule: Use `info!()`, `warn!()`, `error!()` etc. inside spans -- fields propagate automatically

### Levels

| Level | Purpose |
|-------|---------|
| `trace!` | Very detailed debugging |
| `debug!` | Development debugging |
| `info!` | Operational information |
| `warn!` | Warning conditions |
| `error!` | Error conditions |

### Production
- Rule: Use JSON output for production (`fmt::layer().json()`); pretty output for development
- Rule: Configure via env: `RUST_LOG=myapp=info,other=warn`

---

## Serde

### Basic Usage
- Rule: Derive `Serialize` + `Deserialize` on structs; use `serde_json`, `toml`, or `serde_yaml` for format-specific encoding/decoding

### Serde Attributes

| Attribute | Purpose |
|-----------|---------|
| `#[serde(rename_all = "camelCase")]` | Field naming convention |
| `#[serde(default)]` | Use `Default` if field missing |
| `#[serde(default = "fn_name")]` | Custom default function |
| `#[serde(skip_serializing_if = "Option::is_none")]` | Skip None fields |
| `#[serde(rename = "apiKey")]` | Rename specific field |
| `#[serde(skip)]` | Skip field entirely |
| `#[serde(flatten)]` | Flatten nested struct |
| `#[serde(alias = "alt_name")]` | Accept alternative names |
| `#[serde(deny_unknown_fields)]` | Reject extra fields |

### Enum Tagging

| Strategy | Attribute | JSON Example |
|----------|-----------|-------------|
| Externally tagged (default) | none | `{"Click": {"x": 10}}` |
| Internally tagged | `#[serde(tag = "type")]` | `{"type": "Click", "x": 10}` |
| Adjacently tagged | `#[serde(tag = "type", content = "data")]` | `{"type": "Text", "data": "hello"}` |
| Untagged | `#[serde(untagged)]` | `42` or `"hello"` |

- Gotcha: Untagged enums try variants in order -- put more specific variants first
- Rule: Use internally tagged for most APIs; adjacently tagged when payload types vary

### Custom Serialization
- Rule: Implement `Serialize`/`Deserialize` manually for redacting secrets or special formats
- Rule: Use `serde_with` crate for common transformations (e.g., `DisplayFromStr`)

### Unknown Fields
- Rule: Use `#[serde(flatten)] extra: HashMap<String, Value>` to capture unknown fields
- Rule: Use `#[serde(deny_unknown_fields)]` for strict parsing in configs

---

## Configuration

### Multi-Source Config
- Rule: Use `config` crate to layer defaults, config files, and env vars (in precedence order)
- Rule: Prefix env vars (e.g., `APP_`) using `Environment::with_prefix("APP")`

### Validation
- Rule: Use `validator` crate with `#[derive(Validate)]` for field validation (`#[validate(length(min = 1))]`, `#[validate(range(min = 1024))]`, `#[validate(url)]`, `#[validate(email)]`)
- Rule: Validate immediately after deserialization before passing config to application code

---

## CLI Error Handling

- Rule: Return `anyhow::Result<()>` from `main()` for automatic error display
- Rule: For custom formatting, use `if let Err(e) = run()` with `e.chain()` to print cause chain
- Rule: Use `bail!()` for early exit with error message
