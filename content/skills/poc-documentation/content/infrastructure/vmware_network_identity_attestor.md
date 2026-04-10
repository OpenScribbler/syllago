## Overview

The Network Identity Attestor (NIA) provides cryptographically verifiable identity for workloads running on VMware vSphere VMs. It acts as a bridge between your VMware infrastructure and Aembit by observing VM MAC addresses, looking up VM metadata via the vCenter API, and issuing signed attestation documents that the Agent Proxy presents to Aembit Cloud.

Deploy the NIA as a dedicated VM on the same L2 network segment as the client workload VMs it will attest. Each L2 segment requires its own NIA instance.

> **Customer-managed TLS required.** Aembit Managed TLS is not supported for the NIA. You must provide your own TLS and signing certificates for all components (Agent Controller, NIA, and Agent Proxy).

## Prerequisites

- Dedicated Ubuntu 24.04 VM on the same L2 network segment as the client workload VMs
- VM has outbound access to the vCenter API endpoint
- vCenter API credentials with read access to VM metadata
- Customer-provided TLS certificate and private key for the NIA (CN must match the NIA hostname, full chain required)
- Customer-provided attestation signing certificate and private key (RSA key pair, must NOT be a CA certificate)

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{NIA_HOSTNAME}}` | Hostname assigned to the NIA VM |
| `{{NIA_VERSION}}` | NIA release version provided by Aembit |
| `{{VCENTER_URL}}` | vCenter Server URL (e.g., `https://vcenter.example.com`) |
| `{{TLS_CERT_PATH}}` | Path to the TLS certificate file on the NIA VM |
| `{{TLS_KEY_PATH}}` | Path to the TLS private key file on the NIA VM |
| `{{SIGNING_CERT_PATH}}` | Path to the attestation signing certificate on the NIA VM |
| `{{SIGNING_KEY_PATH}}` | Path to the attestation signing private key on the NIA VM |
| `{{VCENTER_CREDENTIALS_PATH}}` | Path to the vCenter credentials file on the NIA VM (format: `username:password`) |

## Part 1: Prepare Certificates

Two certificate pairs are required. Generate these from your organization's internal PKI or certificate authority.

**TLS certificate** - secures HTTPS communication between the Agent Proxy and the NIA:
- CN must match the NIA hostname (`{{NIA_HOSTNAME}}`)
- SAN required
- Full certificate chain (leaf first, then intermediates)

**Attestation signing certificate** - signs the identity documents the NIA issues:
- RSA key pair (ECDSA and Ed25519 are not supported)
- Must NOT be a CA certificate
- CN does not need to match a DNS name
- SAN is not required
- Single certificate (no chain required)

Place both certificate pairs on the NIA VM at accessible paths and note the paths for Part 2.

Create a vCenter credentials file containing the vCenter username and password in the format `username:password`:

```bash
echo "{{VCENTER_USERNAME}}:{{VCENTER_PASSWORD}}" > {{VCENTER_CREDENTIALS_PATH}}
chmod 600 {{VCENTER_CREDENTIALS_PATH}}
```

## Part 2: Install Network Identity Attestor

1. SSH into the NIA VM

2. Download and extract the NIA package:

```bash
wget https://releases.aembit.io/netid_attestor/{{NIA_VERSION}}/linux/amd64/aembit_netid_attestor_linux_amd64_{{NIA_VERSION}}.tar.gz
tar xf aembit_netid_attestor_linux_amd64_{{NIA_VERSION}}.tar.gz
cd aembit_netid_attestor_linux_amd64_{{NIA_VERSION}}
```

3. Run the installer with the required environment variables:

```bash
sudo TLS_PEM_PATH={{TLS_CERT_PATH}} \
     TLS_KEY_PATH={{TLS_KEY_PATH}} \
     AEMBIT_ATTESTATION_SIGNING_KEY_PATH={{SIGNING_KEY_PATH}} \
     AEMBIT_ATTESTATION_SIGNING_CERTIFICATE_PATH={{SIGNING_CERT_PATH}} \
     AEMBIT_VCENTER_URL={{VCENTER_URL}} \
     AEMBIT_VCENTER_CREDENTIALS_FILE={{VCENTER_CREDENTIALS_PATH}} \
     AEMBIT_LOG_LEVEL=debug \
     ./install
```

4. Verify the NIA is running:

```bash
curl -k https://localhost/health
```

A response of `{"status":"Healthy"}` confirms the NIA is running and connected to vCenter.

## Verification

- `curl -k https://localhost/health` returns `{"status":"Healthy"}`
- The NIA service is running: `sudo systemctl status aembit_netid_attestor`

## Troubleshooting

- **Health endpoint returns Unhealthy:** The NIA cannot communicate with vCenter. Verify the `{{VCENTER_URL}}` is reachable from the NIA VM, and that the credentials file contains valid credentials in `username:password` format. Check logs: `sudo journalctl -u aembit_netid_attestor -n 50`
- **TLS connection errors from Agent Proxy:** The client workload VM does not trust the CA that issued the NIA TLS certificate. Install the root CA certificate in the client VM's trust store and run `sudo update-ca-certificates`
- **NIA service fails to start:** Verify that the TLS private key is in PKCS8 format. If using a PKCS1 key, convert it: `openssl pkcs8 -in tls.key -topk8 -out tls.pkcs8.key -nocrypt`
