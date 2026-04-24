<!-- modeled after: grahama1970/claude-code-mcp-enhanced CLAUDE.md -->
## 🚀 Getting Started

Clone the repository and run the bootstrap script. The project targets
Linux and macOS; Windows users should use WSL2. The first build takes a
few minutes because it compiles the native extensions from source.

Once the bootstrap finishes, run the verification command to make sure
your environment can execute the test suite end to end.

## 🔧 Configuration

Configuration lives in config.yaml at the repository root. Every key has
a default; the file only needs to list values that differ from the
default. Secrets belong in .env, never in config.yaml.

Environment variables override file values. This is convenient for CI,
where secrets are injected via the job environment.

## 📦 Building

The canonical build target produces a reproducible artifact. Reproducible
builds are enforced in CI; if your local build diverges, that is a bug.

Artifacts are named with a content hash and signed with the release key.

## 🧪 Testing

Unit tests run in under a minute. Integration tests need a local service
container and take longer. End-to-end tests run nightly.

Every new feature lands with tests. A PR without tests is a PR that needs
more work.

## 🐛 Debugging

Reach for the structured logs first. The log correlation id makes it
easy to follow a request through the services. Attach a debugger only
after the logs have pointed at the likely scope.

## 📚 Documentation

Docs live in the docs/ directory and are published alongside releases.
When a behavior changes, update the doc in the same PR that changes the
code.
