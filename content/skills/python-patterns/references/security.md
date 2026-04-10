# Python Security Anti-Patterns

Critical security patterns for **reviewing** and **writing** Python code.

---

## Input Handling

### `eval()` / `exec()` with Untrusted Input
**Severity**: critical

- Rule: Never `eval()`/`exec()` user input. Use `ast.literal_eval()` for Python literals, JSON/YAML parsers for config.

### Pickle with Untrusted Data
**Severity**: critical

- Rule: `pickle.loads()` executes arbitrary code. Use JSON, MessagePack, or Protocol Buffers for untrusted data.

### Insecure YAML Deserialization
**Severity**: critical

- Rule: Always use `yaml.safe_load()`. Never `yaml.load()` without `Loader=yaml.SafeLoader`.

---

## Injection

### SQL Injection
**Severity**: critical

- Rule: Never use f-strings or `%` formatting in SQL. Always use parameterized queries or ORM query builders.

### Command Injection
**Severity**: critical

- Rule: Never use `os.system()` or `subprocess` with `shell=True` and user input. Use argument lists: `subprocess.run(["cmd", arg])`.

### Path Traversal
**Severity**: high

- Rule: Resolve user-provided paths with `Path.resolve()` and verify with `is_relative_to(base_dir)` before serving.

---

## Secrets and Randomness

### Hardcoded Secrets
**Severity**: critical

- Rule: Use environment variables, `.env` files (never committed), or secret managers. Add `.env` to `.gitignore`.

### Insecure Random Generation
**Severity**: high

- Rule: Use `secrets` module for tokens, session IDs, reset codes. `random` module is predictable (Mersenne Twister).
