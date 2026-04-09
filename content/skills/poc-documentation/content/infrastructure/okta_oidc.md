## Overview

Configuring Okta as an OIDC 1.0 Identity Provider federates human authentication to your Okta tenant. After configuration, a **Sign In with Okta** button appears on the Aembit login page.

## Prerequisites

- Aembit tenant with Administrator access and a Teams or Enterprise subscription
- Okta tenant with Administrator access
- Ability to create an OIDC application in Okta

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{OKTA_OIDC_ISSUER_URL}}` | Okta Admin Console → **Security → API → Authorization Servers** → your server → **Issuer URI** |
| `{{OKTA_OIDC_CLIENT_ID}}` | Okta Admin Console → your OIDC app → **General** tab → **Client ID** |
| `{{OKTA_OIDC_CLIENT_SECRET}}` | Okta Admin Console → your OIDC app → **General** tab → **Client Secrets** section |
| `{{AEMBIT_REDIRECT_URL}}` | Aembit Management Console → **Administration → Identity Providers → New (OIDC 1.0)** → **Redirect URL** field (auto-generated) |

### Okta Setup

1. In the Okta Admin Console, navigate to **Applications → Applications** and click **Create App Integration**
   - **Sign-in method**: OIDC - OpenID Connect
   - **Application type**: Web Application
   - Click **Next**

2. Select the Assignments method for the app and click Save.  All other values can be left as default. We will update the Sign-in redirect URIs in a future step.

3. Click Edit on the Public keys section of the General tab of the Okta app and make the following changes:
   - Set the Public keys Configuration to Use a URL to fetch keys dynamically
   - Set the Public keys URL to `https://{{AEMBIT_TENANT_ID}}.id.useast2.aembit.io/.well-known/
   - Click Save

4. Click Edit on the Client Credentials section of the General tab of the Okta app and make the following changes:
   - Set the Client authentication type to `Public key / Private key`
   - Check the Require PKCE as additional verification box
openid-configuration/jwks`
   - Click Save

4. Copy the **Client ID** (`{{OKTA_OIDC_CLIENT_ID}}`) from the app's **General** tab

## Aembit Configuration

1. In the Aembit Management Console, navigate to **Administration → Identity Providers** and click **+ New**
   - **Name**: Okta OIDC
   - **Identity Provider Type**: select **OIDC 1.0**
   - **Identity Provider Base URL**: `{{OKTA_OIDC_ISSUER_URL}}`
   - **Identity Provider Client ID**: paste `{{OKTA_OIDC_CLIENT_ID}}`
   - **PKCE Required**: enable
   - **Authentication Method**: Public Private Keypair
   - **Scopes**: `openid profile email` (add `groups` if using group-based role mappings)
   - Click Save

2. Open the Identity Provider created in the previous step and copy the **Aembit Redirect URL** displayed and add it to the Okta app's **Sign-in redirect URIs**

## Verification

- The Identity Provider appears as **Active** in **Administration → Identity Providers**
- Clicking the Verify Configuration button on the Identity Provider in Aembit shows Verified Successfully
- Navigate to the Aembit login page — a **Sign In with Okta OIDC** button appears
- Log in using an Okta user assigned to the app and confirm successful authentication
- Navigate to **Administration → Users** and confirm the Okta user appears with the expected role

## Troubleshooting

- **OIDC redirect fails:** The Aembit Redirect URL was not added to the Okta app's sign-in redirect URIs. Edit the Okta OIDC app and add the exact URL
