# Aembit Cloud API

REST API for the Aembit Cloud control plane.

**Base URL**: `https://{tenant}.aembit.io`
**Auth**: `Authorization: Bearer <access_token>`
**Optional Header**: `X-Aembit-ResourceSet` — scope requests to a resource set

## Response Codes

| Code | Meaning |
|------|---------|
| 200/201 | Success |
| 204 | No content |
| 302 | Redirect (authorization endpoints) |
| 400 | Invalid request |
| 401 | Not authenticated |
| 403 | Forbidden |
| 500 | Server error |

## Pagination & Filtering

List endpoints support: `page`, `per-page`, `filter`, `order`, `group-by`, `search`

List response structure:
```json
{"page": 1, "perPage": 25, "order": "asc", "statusCode": 200, "recordsTotal": 100, "data": [...]}
```

## Standard CRUD Endpoints

These resources support: GET (list), POST (create), PUT (update — `externalId` in body), GET `/{id}`, PATCH `/{id}` (partial update), DELETE `/{id}`

| Resource | Path | Version | Notes |
|----------|------|---------|-------|
| Access Conditions | `/api/v1/access-conditions` | v1 | |
| Access Policies | `/api/v2/access-policies` | v2 | v1 deprecated |
| Agent Controllers | `/api/v1/agent-controllers` | v1 | |
| Client Workloads | `/api/v1/client-workloads` | v1 | |
| Credential Providers | `/api/v2/credential-providers` | v2 | v1 deprecated |
| Credential Provider Integrations | `/api/v1/credential-integrations` | v1 | |
| Discovery Integrations | `/api/v1/discovery-integrations` | v1 | |
| Integrations | `/api/v1/integrations` | v1 | |
| Log Streams | `/api/v1/log-streams` | v1 | |
| Resource Sets | `/api/v1/resource-sets` | v1 | |
| Roles | `/api/v1/roles` | v1 | |
| Routing | `/api/v1/routings` | v1 | |
| Server Workloads | `/api/v1/server-workloads` | v1 | |
| SSO Identity Providers | `/api/v1/sso-idps` | v1 | |
| Standalone Certificate Authorities | `/api/v1/certificate-authorities` | v1 | |
| Trust Providers | `/api/v1/trust-providers` | v1 | |
| Users | `/api/v1/users` | v1 | |

## Additional Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v2/access-policies/{id}/notes` | Add policy note |
| GET | `/api/v2/access-policies/{id}/notes` | List policy notes |
| GET | `/api/v2/access-policies/{id}/credential-mappings` | List credential mappings |
| GET | `/api/v2/access-policies/getByWorkloadIds/{clientId}/{serverId}` | Find policy by workload pair |
| POST | `/api/v1/agent-controllers/{id}/device-code` | Generate device code for registration |
| GET | `/api/v2/credential-providers/{id}/verification` | Verify credential provider config |
| GET | `/api/v2/credential-providers/{id}/authorize` | Get OAuth authorization URL |
| GET | `/api/v1/credential-integrations/list/{type}` | List integrations by type |
| GET | `/api/v1/certificate-authorities/{id}/root-ca` | Download root CA certificate |
| GET | `/api/v1/sso-idps/{id}/verification` | Verify SSO provider config |
| POST | `/api/v1/users/{id}/unlock` | Unlock user account |
| GET | `/api/v1/root-ca` | Download tenant root CA (TLS Decrypt) |
| GET | `/api/alpha/server-workload-drafts/{id}` | Get discovered workload draft (Alpha) |
| GET | `/api/v1/health` | Health check |

### Read-Only Event Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/audit-logs` | List audit logs |
| GET | `/api/v1/audit-logs/{id}` | Get specific audit log |
| GET | `/api/v1/authorization-events` | List authorization events |
| GET | `/api/v1/authorization-events/{id}` | Get specific authorization event |
| GET | `/api/v1/workload-events` | List workload events |
| GET | `/api/v1/workload-events/{id}` | Get specific workload event |

### Configuration Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/compliance-settings` | Get compliance settings |
| PUT | `/api/v1/compliance-settings` | Update compliance settings |
| GET | `/api/v1/signin-policies` | Get sign-on policies |
| PUT | `/api/v1/signin-policies/mfa` | Update MFA policy |
| PUT | `/api/v1/signin-policies/sso` | Update SSO policy |

## Common Entity Fields

All entities include:

| Field | Type | Description |
|-------|------|-------------|
| `externalId` | UUID | Unique identifier |
| `name` | string | Required |
| `description` | string | Optional |
| `isActive` | boolean | Active status |
| `tags` | array | `[{"key": "...", "value": "..."}]` |
| `createdAt` | datetime | Creation timestamp |
| `modifiedAt` | datetime | Last modified |
| `createdBy` | string | Creator identity |
| `modifiedBy` | string | Last modifier |
| `resourceSet` | UUID | Owning resource set |

## Key Schema Details

### AccessPolicyDTO (v2)

| Field | Type | Notes |
|-------|------|-------|
| `clientWorkload` | UUID | Client Workload ID |
| `serverWorkload` | UUID | Server Workload ID |
| `trustProviders` | UUID[] | Trust Provider IDs |
| `credentialProviders` | array | Credential provider references |
| `accessConditions` | UUID[] | Access Condition IDs |
| `policyNotes` | array | Timestamped notes with creator |

**Note:** Multiple credential providers per policy supported only for Snowflake JWT (`jwt-token` configured for Snowflake) and AWS STS Federation types.

### ClientWorkloadExternalDTO

| Field | Type | Notes |
|-------|------|-------|
| `identities` | array | `[{"type": "...", "value": "..."}]` |
| `standaloneCertificateAuthority` | UUID | Optional CA for TLS |
| `type` | string | Workload type |
| `accessPolicyCount` | int | Related policies count |

### ServerWorkloadExternalDTO

| Field | Type | Notes |
|-------|------|-------|
| `serviceEndpoint` | object | See WorkloadServiceEndpointDTO |
| `type` | string | Workload type |
| `accessPolicyCount` | int | Related policies count |

### WorkloadServiceEndpointDTO

| Field | Type | Notes |
|-------|------|-------|
| `host` | string | Target hostname |
| `port` | int | Target port |
| `tls` | boolean | TLS enabled |
| `tlsVerification` | string | TLS verification mode |
| `appProtocol` | string | Application protocol |
| `transportProtocol` | string | Transport protocol |
| `httpHeaders` | array | Custom HTTP headers |
| `workloadServiceAuthentication` | object | Auth method and scheme |

### TrustProviderDTO

| Field | Type | Notes |
|-------|------|-------|
| `provider` | string | Trust provider type |
| `matchRules` | array | Attestation match rules (attribute-value pairs) |
| `certificate` | string | Certificate for validation |
| `jwks` | string | JWKS for token validation |
| `oidcUrl` | string | OIDC discovery URL |
| `symmetricKey` | string | Symmetric key |
| `pemType` | string | PEM type |
| `agentControllerIds` | UUID[] | Associated controllers |
| `agentControllersCount` | int | Controller count |
| `accessPolicyCount` | int | Related policies count |

### CredentialProviderDTO (v2)

| Field | Type | Notes |
|-------|------|-------|
| `type` | string | Credential provider type (see enum below) |
| `roleId` | UUID | Associated role |
| `lifetimeTimeSpanSeconds` | int | Credential TTL |
| `lifetimeExpiration` | datetime | Absolute expiration |
| `providerDetailJSON` | string | Type-specific configuration (JSON) |
| `accessPolicyCount` | int | Related policies count |

### AgentControllerDTO

| Field | Type | Notes |
|-------|------|-------|
| `trustProviderId` | UUID | Associated trust provider |
| `tlsCertificates` | array | TLS certificate configs |
| `isHealthy` | boolean | Health status |
| `lastReportedUptime` | int64 | Uptime in seconds |
| `lastReportedHealthTime` | datetime | Last health report |
| `allowedTlsHostname` | string | Allowed TLS hostname |
| `version` | string | Agent controller version |

### AuthorizationEventDTO

| Field | Type | Notes |
|-------|------|-------|
| `meta` | object | Event metadata (timestamp, ID) |
| `outcome` | object | Result and reason |
| `clientRequest` | object | Original request details |
| `environment` | object | Environment context |
| `clientWorkload` | object | Client details |
| `serverWorkload` | object | Server details |
| `accessPolicy` | object | Matched policy |
| `trustProviders` | array | Attestation results |
| `accessConditions` | array | Condition evaluation results |
| `credentialProvider` | object | Credential provider result |

### LogStreamDTO

| Field | Type | Notes |
|-------|------|-------|
| `dataType` | string | Log data type |
| `type` | object | Destination config (type-specific) |

### ResourceSetDTO

Beyond common fields, includes count fields for all associated resources (`serverWorkloadCount`, `clientWorkloadCount`, `accessPolicyCount`, `trustProviderCount`, `accessConditionCount`, `credentialProviderCount`) plus `roles`, `rolesDetails`, and `users` arrays.

## Enum Values

### Credential Provider Types
`aembit-access-token`, `api-key`, `aws-sts`, `azure-entra-federation`, `google-workflow-id`, `gitlab-managed-account`, `jwt-token`, `oauth2-authcode`, `oauth-client-credential`, `username-password`, `vault-client-token`

**Note:** AWS Secrets Manager and Azure Key Vault credential providers exist as features but their API type strings are not in the published API model. Check live docs or the UI for current configuration.

### Credential Provider Integration Types
`GitLab`, `AwsIamRole`

### Log Stream Destination Types
`AwsS3Bucket`, `GcsBucket`, `SplunkHttpEventCollector`, `CrowdstrikeHttpEventCollector`

### Authorization Event Outcomes
`Success`, `Denied`, `Error`

### Policy Credential Mapping Types
`None`, `AccountName`, `HttpHeader`, `HttpBody`
