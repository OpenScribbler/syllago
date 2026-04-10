# API Authentication Patterns

This file covers auth method selection and key gotchas. For implementation depth (JWT validation code, OAuth2 flow implementation, mTLS configuration), load `skills/security-audit/SKILL.md`.

## Auth Method Decision Table

| Method | Use For | Stateless | Revocable | Key Gotcha |
|--------|---------|-----------|-----------|------------|
| API key | App identification + rate limiting | Yes | Yes (key rotation) | NOT authentication -- identifies the app, not the user |
| JWT (RS256/ES256) | Stateless microservice auth | Yes | No (until expiry) | Short TTL (5-15 min) + refresh token; never HS256 in multi-service |
| OAuth2 + PKCE | Delegated user access (third-party apps) | Depends on token | Yes (revoke grant) | Implicit flow deprecated -- always use Authorization Code + PKCE |
| mTLS | Zero-trust machine-to-machine | Yes | Yes (cert revocation) | Requires PKI infrastructure; not practical for public APIs |

### Selection Heuristic

1. **Public API with third-party consumers?** -- OAuth2 + PKCE for user data, API keys for app identity
2. **Internal service-to-service?** -- JWT (service accounts) or mTLS (zero-trust mesh)
3. **Mobile/SPA client?** -- OAuth2 Authorization Code + PKCE (never implicit, never client credentials)
4. **Server-to-server with Aembit?** -- Load `skills/aembit-knowledge/SKILL.md` for workload identity patterns

## Key Rules

- **API keys are NOT authentication**: They identify the calling application for rate limiting and analytics. Pair with a real auth mechanism for user identity.
- **RS256/ES256, never HS256 for multi-service**: HS256 requires sharing the secret with every service that validates tokens. RS256/ES256 use asymmetric keys -- only the issuer has the private key.
- **Short JWT TTL**: 5-15 minutes for access tokens. Use refresh tokens (stored server-side, revocable) for long sessions.
- **OAuth2 implicit flow is deprecated**: Use Authorization Code + PKCE for all public clients (SPAs, mobile). PKCE prevents authorization code interception.
- **Never send credentials in URLs**: Query params are logged in server access logs, browser history, and proxy logs. Use `Authorization` header.

## Rate Limiting Integration

API keys are the natural unit for rate limiting:
- One key per client application
- Rate limits tied to key tier (free/pro/enterprise)
- Separate keys for separate environments (dev/staging/prod)
- Rotate keys without downtime: support two active keys during rotation window

## API Security Basics

- **CORS**: Explicit origin allowlist. Never use `*` for authenticated endpoints.
- **Content-Type enforcement**: Reject requests without `Content-Type` header on POST/PUT/PATCH.
- **Security headers on all responses**:
  - `Cache-Control: no-store` (prevent credential caching)
  - `Strict-Transport-Security: max-age=31536000; includeSubDomains` (HSTS)
  - `X-Content-Type-Options: nosniff`
- **Input validation**: Validate and sanitize all input server-side regardless of client-side validation.

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| API key as sole authentication | Keys identify apps, not users | Pair with JWT or OAuth2 |
| HS256 across services | Shared secret = any service can forge tokens | RS256/ES256 (asymmetric) |
| Long-lived JWT (hours/days) | Cannot revoke compromised token | Short TTL (5-15 min) + refresh token |
| OAuth2 implicit flow | Token exposed in URL fragment, no PKCE | Authorization Code + PKCE |
| Credentials in query params | Logged in access logs, browser history | Use Authorization header |
| CORS wildcard `*` with credentials | Browser ignores wildcard when credentials sent | Explicit origin allowlist |
