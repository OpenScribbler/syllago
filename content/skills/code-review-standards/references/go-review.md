# Go Code Review: False Positives and Verification

Patterns to avoid false positives when reviewing Go code. Every finding MUST pass the relevant verification check before reporting.

---

## Non-Issues: Do NOT Flag

### Example and Test Code
- Rule: Example code prioritizes clarity over production robustness. `log.Fatal`, ignored errors, and minimal handling are acceptable in `examples/`, `demo/`, `testdata/`, and `*_test.go` directories.
- Flag only if: example demonstrates incorrect API usage, would cause security issues if copy-pasted, or hides important behavior.

### math/rand for Non-Cryptographic Purposes
**Severity**: non-issue when used for jitter, shuffling, sampling, load balancing, or test data.
- Rule: Only flag `math/rand` when the value has security implications (tokens, nonces, session IDs, keys, passwords).
- Decision: Could an attacker benefit from predicting this value? No -> non-issue.

### Intentional Panic for Unrecoverable Situations
- Rule: Documented panics on crypto/rand failure, impossible states (`default: panic("unreachable")`), or startup config validation are idiomatic.
- Flag only if: panic is used for recoverable errors (file not found, network timeout) in library code.

### Debug Logging of Presence Flags
- Rule: Logging `hasAPIKey=true`, `tokenLength=32`, or `credentialCount=3` is safe -- these are presence indicators, not secret values.
- Flag only if: actual credential values appear in log output.

### fmt.Errorf vs errors.New for Static Messages
- Rule: `fmt.Errorf("static string")` and `errors.New("static string")` are functionally identical. Do not flag.

### interface{} vs any
- Rule: Pure alias since Go 1.18. Both are acceptable. Do not flag.

### Unused Context Parameter
- Rule: `ctx context.Context` in interface methods or public APIs exists for consistency and future-proofing.
- Flag only if: function is private AND not implementing an interface AND context will clearly never be needed.

### External Service as Fallback
- Rule: Using an external service as a fallback after local detection fails is acceptable.
- Flag only if: external service is the primary method with no local alternative.

### Placeholder Values with Documentation
- Rule: Constants like `DefaultTenantID = "example-tenant"` with explanatory comments are intentional design.
- Flag only if: production code has hardcoded credentials without documentation.

### Unused Parameters in Public APIs
- Rule: Unused parameters in public APIs may exist for consistency across a function family or future-proofing. Check for `_ = param` idiom with comment.

### Documented Intentional Design
- Rule: If code comments explain the design rationale, do not flag the behavior as an issue. If your analysis concludes "this is intentional and correct," skip it.

### Unused Constants
- Rule: Constants documenting expected values are acceptable even if unused. Flag only if: constant duplicates a literal used elsewhere (DRY violation) or is truly purposeless.

### Mixed Sentinel + Custom Error Types
- Rule: Using both `var ErrX = errors.New(...)` and custom error types is idiomatic Go (see `io.EOF` vs `*os.PathError`). Do NOT flag as "inconsistent error handling."

### Mock/Test/Demo Code
- Rule: Mock services are intentionally simplified. Test code skips error handling for brevity. Demo code shows concepts. Flag as informational at most: "Ensure production code differs."

### Explicit Opt-In Insecure Paths
- Rule: If insecure behavior (e.g., `InsecureSkipVerify: true`) requires explicit user configuration, it is a design choice, not a vulnerability.
- Recommendation: Suggest a warning log when the insecure path is taken, not a code change.

### Well-Known Development Credentials
- Rule: Default credentials for Keycloak (`admin/admin`), PostgreSQL (`postgres/postgres`), Redis (no password), RabbitMQ (`guest/guest`) in dev/demo contexts are acceptable.
- Flag as informational: "Ensure production deployments use different credentials."

### Framework/Plugin SDK Trust Boundaries
- Rule: Data from trusted internal interfaces (SPIRE WorkloadAttestor PIDs, K8s admission webhook requests) is pre-validated by the framework. Skip redundant validation findings.
- Flag only if: data crosses an external trust boundary or framework guarantees are undocumented.

---

## Real Issues: ALWAYS Flag

### Inconsistent Resource Limiting
**Severity**: high
- Rule: Size limits applied to error paths but not success paths (or vice versa) indicate incomplete protection.
- Example: `io.LimitReader` on error response but unbounded `io.ReadAll` on success response.
- Fix: Apply consistent limits via `io.LimitReader` or `http.MaxBytesReader` on all paths.

### Credential/Secret Value Logging
**Severity**: critical
- Rule: Logging passwords, API keys, tokens, private keys, session IDs, or auth headers is always a finding.
- Gotcha: `log.Printf("request: %+v", authRequest)` may expose secrets in struct fields. Check for `String()` redaction method before flagging.

### Missing Size Limits on Untrusted Input
**Severity**: high
- Rule: `io.ReadAll(resp.Body)` or `os.ReadFile(userPath)` without size limits risks memory exhaustion from untrusted sources.
- Fix: `io.LimitReader(r, maxSize)` or `http.MaxBytesReader(w, r.Body, maxSize)`.

---

## Verification Steps (Complete BEFORE Reporting)

### 1. Trace Control Flow
Before claiming "resource not cleaned up on error path": identify WHERE the sensitive operation occurs, trace ALL exit paths, check if early returns happen BEFORE or AFTER the operation. Early validation returns before a sensitive copy are NOT unprotected.

### 2. Check Synchronization Primitives
Before claiming race conditions: search the struct for `sync.Mutex`, `sync.RWMutex`, `atomic.*`, or channel fields. Read method comments about thread-safety.

### 3. Check Timeout Mechanisms at Multiple Levels
Before claiming "no timeout": check context level (`WithTimeout`), HTTP client level (`Timeout`), transport/dial level, and request level. One timeout at ANY layer may be sufficient.

### 4. Check Redaction Implementations
Before claiming "sensitive data may be logged": check if the struct implements `fmt.Stringer` or `fmt.GoStringer` with redaction, or has a custom `MarshalJSON`.

### 5. Verify Exact Type/Field Names
Similar names have different security implications. `service-account-name` (safe reference) vs `service-account-token` (contains secret). Read the EXACT name and check documentation.

### 6. Verify Citations and Evidence
Re-read the actual file after drafting findings. Copy/paste exact code snippets -- never paraphrase from memory. Verify line numbers with grep. Red flag: citing line numbers from memory.

### 7. Trace Complete Path Construction
Before claiming path traversal: trace the COMPLETE path including all prefixes AND suffixes. A path like `{base}/{input}/address` can only read files named `address`.

### 8. Distinguish Deployment-Time Config from Runtime Input
Environment variables set by cluster admins at deployment time are trusted by design. Ask: WHO can set this value? WHEN is it set? Could an attacker who modifies it already do worse?

### 9. Check Documentation Before Claiming Confusion
Read the actual docs (comments, README, values.yaml) before flagging something as "confusing" or "error-prone." If documentation explains the design, it is not a finding.

### 10. Understand Non-Fatal/Best-Effort Patterns
Before flagging "error not handled": check if the operation is intentionally best-effort (logged and continued). Indicators: error is logged, comment explains, function name includes "Try"/"Optional", failure does not affect core functionality.

---

## Reviewer Principles

1. **Context matters.** A pattern fine in one context may be problematic in another.
2. **Check documentation.** Many apparent issues are intentional when documented.
3. **Consider the threat model.** Internal tools have different requirements than public APIs.
4. **Prefer actionable feedback.** If flagging, explain the specific risk and suggest a fix.
