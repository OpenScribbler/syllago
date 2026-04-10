## Prerequisites

- Target API gateway hostname and port
- SPIFFE trust domain for your organization
- Audience value the API gateway expects in the JWT `aud` claim
- Desired token lifetime in minutes
- Signing algorithm supported by the API gateway (RS256 or ES256)
- Aembit Management Console access to retrieve tenant OIDC/JWKS endpoints

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| {{API_GW_HOST}} | Hostname of the target API gateway (e.g., `api.internal.example.com`) |
| {{API_GW_PORT}} | Port for the API gateway (typically `443`) |
| {{SPIFFE_TRUST_DOMAIN}} | Your organization's SPIFFE trust domain (e.g., `example.com`) |
| {{JWT_AUDIENCE}} | Audience claim the API gateway validates - confirm with the gateway team |
| {{JWT_LIFETIME_MINUTES}} | Token validity in minutes - balance security requirements with rotation overhead |

## Service Configuration

### API Gateway JWT Validation Setup

Configure the API gateway to trust JWTs issued by Aembit using OIDC Auto-Discovery:

```
https://{{AEMBIT_TENANT_ID}}.id.useast2.aembit.io/.well-known/openid-configuration
```

The gateway will automatically retrieve the JWKS endpoint and signing keys. This URL is also displayed in the Credential Provider dialog after saving — copy it directly from the Aembit Management Console.

**JWT validation rules to configure on the gateway:**

| Claim | Expected Value |
|-------|---------------|
| `iss` | Aembit issuer: found in the OIDC discovery document (`issuer` field) |
| `aud` | {{JWT_AUDIENCE}} |
| `sub` | SPIFFE ID format: `spiffe://{{SPIFFE_TRUST_DOMAIN}}/...` |
| `exp` | Enforced automatically: reject expired tokens |

## Aembit Configuration

1. Navigate to **Credential Providers** and click **+ New**
   - Name: descriptive name identifying this credential provider
   - Type: **JWT-SVID Token**
   - **Subject (SPIFFE ID):** To populate the SPIFFE subject from the calling workload's identity, use:
     `spiffe://{{SPIFFE_TRUST_DOMAIN}}/${oidc.identityToken.decode.payload.sub}`
     This produces a SPIFFE ID of the form: `spiffe://example.com/system:serviceaccount:my-namespace:my-sa`
   - **Audience:** `{{JWT_AUDIENCE}}`
   - **Lifetime:** `{{JWT_LIFETIME_MINUTES}}` minutes
   - **Algorithm:** RS256 or ES256: match the algorithm your API gateway is configured to accept
   - Click **Save**

2. Navigate to **Server Workloads** and click **+ New**
   - Host: `{{API_GW_HOST}}`
   - Port: `{{API_GW_PORT}}`
   - Protocol: **HTTPS**
   - Click **Save**

## Verification

- The Credential Provider shows as **Active** in the Aembit Management Console
- The application makes successful API calls to `{{API_GW_HOST}}` without any static credentials
- The API gateway accepts the JWT and returns a 200 response (not 401 Unauthorized)
- Navigate to **Activity** in the Aembit Management Console and confirm log entries show the workload authenticated and received a JWT-SVID credential
- In the API gateway logs, confirm the authenticated principal matches the expected SPIFFE ID (`spiffe://{{SPIFFE_TRUST_DOMAIN}}/...`)

## Troubleshooting

- **API gateway returns 401:** The gateway's JWKS or OIDC discovery URL is not pointing to the Aembit endpoint, or the `iss` / `aud` claims do not match the gateway's validation rules. Re-check the issuer and audience values configured on the gateway against the OIDC discovery document
- **Credential Provider shows Inactive:** The JWT-SVID Credential Provider configuration is incomplete. Verify the Subject, Audience, Lifetime, and Algorithm fields are all populated and click Save
- **`${oidc.identityToken.decode.payload.sub}` appears literally in the issued token:** This dynamic expression requires the calling workload to present a Kubernetes service account token. Confirm the pod's service account token is mounted at `/var/run/secrets/kubernetes.io/serviceaccount/token` and that the Trust Provider is configured to accept Kubernetes OIDC tokens
- **Token rejected: algorithm mismatch:** The gateway is configured for RS256 but the Credential Provider is set to ES256 (or vice versa). Update the **Algorithm** field in the Credential Provider to match the gateway's expected algorithm
