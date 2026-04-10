# Aembit Edge API

REST API for the Aembit Edge data plane.

**Base URL**: `https://{tenant}.aembit.io`
**Optional Header**: `X-Aembit-ResourceSet` — scope to a resource set

## Overview

The Edge API has two endpoints used by Agent Proxies for workload authentication and credential retrieval. These are NOT typically called by application code directly.

## Response Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 400 | Invalid or incomplete request |
| 401 | Authentication failed |
| 500 | Server error |

## Endpoints

### POST /edge/v1/auth

Authenticates a Client Workload to Aembit Edge using a Trust Provider.

**Request:** `AuthRequest`
```json
{
  "clientId": "string|null",
  "client": { ...ClientWorkloadDetails }
}
```

**Response (200):** `TokenDTO`
```json
{
  "accessToken": "string",
  "tokenType": "Bearer",
  "expiresIn": 3600
}
```

### POST /edge/v1/credentials

Retrieves credentials for a Client Workload based on configured Access Policies.

**Request:** `ApiCredentialsRequest`
```json
{
  "client": { ...ClientWorkloadDetails },
  "server": { ...ServerWorkloadDetails },
  "credentialType": "string"
}
```

**Response (200):** `ApiCredentialsResponse`
```json
{
  "credentialType": "string",
  "expiresAt": "datetime|null",
  "data": { ...EdgeCredentials }
}
```

## Request Schemas

### ClientWorkloadDetails

Identity and attestation from the runtime environment.

| Field | Type | Description |
|-------|------|-------------|
| `sourceIP` | string | Client source IP |
| `aws` | AwsDTO | AWS attestation |
| `azure` | AzureAttestationDTO | Azure attestation |
| `gcp` | GcpAttestationDTO | GCP attestation |
| `k8s` | K8sDTO | Kubernetes attestation |
| `host` | HostDTO | Host-level identity |
| `os` | OsDTO | OS environment variables |
| `github` | object | GitHub Actions attestation |
| `terraform` | object | Terraform Cloud attestation |
| `gitlab` | object | GitLab CI attestation |
| `oidc` | object | Generic OIDC attestation |

### ServerWorkloadDetails

| Field | Type |
|-------|------|
| `host` | string |
| `port` | int |
| `transportProtocol` | `TCP` |

## Platform Attestation Schemas

**AwsDTO:**
- `instanceIdentityDocument`, `instanceIdentityDocumentSignature` — EC2 identity
- `lambda` → `LambdaDTO { arn }` — Lambda function ARN
- `ecs` → `AwsEcsDTO { containerMetadata, taskMetadata }` — ECS metadata
- `stsGetCallerIdentity` → `StsGetCallerIdentityDTO { headers, region }` — STS caller

**AzureAttestationDTO:**
- `attestedDocument` → `AzureAttestedDocumentDTO { encoding, signature, nonce }`

**GcpAttestationDTO:**
- `identityToken`, `instanceDocument`

**K8sDTO:**
- `serviceAccountToken`

**HostDTO:**
- `hostname`, `domainName`, `systemSerialNumber`
- `process` → `ProcessDTO { name, pid, userId, userName, exePath }`
- `sensors` → `SensorsDTO { crowdStrike → CrowdStrikeDTO { agentId } }`
- `networkInterfaces` → `[NetworkInterfacesDTO { name, macAddress, ipv4Addresses, ipv6Addresses }]`

**OsDTO:**
- `environment` → `EnvironmentDTO { K8S_POD_NAME, CLIENT_WORKLOAD_ID, KUBERNETES_PROVIDER_ID, AEMBIT_RESOURCE_SET_ID }`

## Response Schemas

### EdgeCredentials

Returned credential data (format varies by type). All fields are `string|null` — only relevant fields populated.

| Type | Fields Populated |
|------|-----------------|
| API Key | `apiKey` |
| OAuth Token | `token` |
| Username/Password | `username`, `password` |
| AWS STS | `awsAccessKeyId`, `awsSecretAccessKey`, `awsSessionToken` |

### CredentialProviderTypes (Edge enum)

`Unknown`, `ApiKey`, `UsernamePassword`, `GoogleWorkloadIdentityFederation`, `OAuthToken`, `AwsStsFederation`

**Note:** This Edge enum covers credential types the Agent Proxy handles directly. Additional types exist in the Cloud API.
