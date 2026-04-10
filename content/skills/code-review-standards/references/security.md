# Security Review Patterns

Detailed security checklist for code review.

---

## Injection Attacks

### SQL Injection
**Severity**: critical
- Rule: Never use string concatenation/formatting for SQL queries. Use parameterized queries or ORM query builders.
- Check: Raw SQL with `f"..."`, `%s`, `+`, or `.format()`. ORM raw query methods without parameterization. Dynamic table/column names from user input.

### Command Injection
**Severity**: critical
- Rule: Never pass user input through shell interpretation. Use array-form subprocess calls.
- Check: `os.system()`, `subprocess.run(shell=True)`, `exec()`, `eval()` with user data. In Go: `exec.Command("sh", "-c", userInput)`.

### XSS (Cross-Site Scripting)
**Severity**: high
- Rule: Never render user input as raw HTML. Use framework auto-escaping or explicit sanitization.
- Check: `innerHTML`, `dangerouslySetInnerHTML`, missing Content-Security-Policy headers, template `|safe` filters on user data.

## Authentication and Authorization

### Token Handling
- [ ] Tokens have appropriate expiration (access: minutes, refresh: days)
- [ ] Tokens validated on every request
- [ ] Token refresh doesn't extend indefinitely
- [ ] Revocation mechanism exists

### Password Storage
- [ ] Using bcrypt, scrypt, or Argon2 (NOT MD5, SHA1, plain SHA256)
- [ ] Unique salt per password
- [ ] Appropriate cost factor (bcrypt >= 12)

### Session Management
- [ ] Session IDs regenerated after login
- [ ] Secure and HttpOnly flags on cookies
- [ ] Session timeout implemented
- [ ] Logout invalidates server-side session

## Sensitive Data Exposure

### Secrets in Code
**Severity**: critical
- Rule: No passwords, API keys, tokens, or private keys in source code. Use environment variables, secret managers, or encrypted config.

### Logging Sensitive Data
**Severity**: high
- Rule: Never log passwords, tokens, API keys, credit card numbers, SSNs, or full request/response bodies containing PII.
- Gotcha: Struct logging with `%+v` or `%v` may expose sensitive fields. Check for `String()` redaction methods. See [go-review.md](go-review.md) verification step 4.

## Race Conditions (TOCTOU)
**Severity**: high
- Rule: Check-then-act patterns without locking are vulnerable. Handle errors at point of use instead of checking existence first.
- Check: `fileExists()` followed by `readFile()`, shared state modified by concurrent operations without synchronization.

## Cryptographic Weaknesses

### Weak Algorithms
| Avoid | Use Instead |
|-------|-------------|
| MD5, SHA1 (for security) | SHA-256, SHA-3 |
| DES, 3DES | AES-256-GCM |
| RSA < 2048 bits | RSA >= 2048 or ECDSA |
| ECB mode | GCM or CBC with HMAC |

### Random Number Generation
**Severity**: high (when misused)
- Rule: Use cryptographically secure randomness (`crypto/rand`, `secrets` module) for tokens, nonces, session IDs, API keys, passwords.
- Non-security uses (jitter, shuffling, sampling, test data) are fine with `math/rand` or `random`. See [go-review.md](go-review.md) for the full decision tree.
