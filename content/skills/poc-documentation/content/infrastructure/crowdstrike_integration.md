## Overview

The CrowdStrike Integration connects your CrowdStrike Falcon tenant to Aembit, enabling device posture checks via CrowdStrike-based Access Conditions. Configure this integration once - it is shared by all CrowdStrike Access Conditions in your Aembit tenant.

## Prerequisites

- CrowdStrike Falcon subscription with API access

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{CROWDSTRIKE_BASE_URL}}` | CrowdStrike Falcon Console → Support → API Clients and Keys → Base URL (e.g., `api.crowdstrike.com`) |
| `{{CROWDSTRIKE_CLIENT_ID}}` | CrowdStrike Falcon Console → Support → API Clients and Keys → your API client → Client ID |
| `{{CROWDSTRIKE_CLIENT_SECRET}}` | CrowdStrike Falcon Console → Support → API Clients and Keys → your API client → Client Secret |

## Service Configuration

### CrowdStrike Setup

1. In the CrowdStrike Falcon Console, navigate to **API Clients and Keys** and create a new API client
   - Required scopes: **Hosts: Read**
   - Copy the **Client ID** (`{{CROWDSTRIKE_CLIENT_ID}}`), **Client Secret** (`{{CROWDSTRIKE_CLIENT_SECRET}}`), and **Base URL** (`{{CROWDSTRIKE_BASE_URL}}`)

## Aembit Configuration

1. In the Aembit Management Console, navigate to **Integrations** and configure a new **CrowdStrike Integration**
   - Enter `{{CROWDSTRIKE_CLIENT_ID}}` and `{{CROWDSTRIKE_CLIENT_SECRET}}` from the CrowdStrike step above
   - Enter your CrowdStrike **Base URL**: `{{CROWDSTRIKE_BASE_URL}}`
   - Verify sync status shows **Successfully Synchronized**

## Verification

- The CrowdStrike Integration in **Integrations** shows **Successfully Synchronized**

## Troubleshooting

- **Integration sync fails:** Verify the CrowdStrike Client ID, Client Secret, and Base URL (`{{CROWDSTRIKE_BASE_URL}}`) are correct. Confirm the API client has **Hosts: Read** scope enabled
