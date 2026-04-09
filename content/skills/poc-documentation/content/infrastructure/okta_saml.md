## Overview

Configuring Okta as a SAML 2.0 Identity Provider federates human authentication to your Okta tenant. After configuration, a **Sign In with Okta** button appears on the Aembit login page.

## Prerequisites

- Aembit tenant with Administrator access and a Teams or Enterprise subscription
- Okta tenant with Administrator access
- Ability to create a SAML application in Okta

## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{OKTA_SAML_METADATA_URL}}` | Okta Admin Console → your SAML app → **Sign On** tab → **Metadata URL** |
| `{{AEMBIT_SP_ENTITY_ID}}` | Aembit Management Console → **Administration → Identity Providers → New (SAML 2.0)** → **Aembit SP Entity ID** field (auto-generated after entering Metadata URL) |
| `{{AEMBIT_SSO_URL}}` | Same dialog → **Aembit SSO URL** field (auto-generated) |

## Service Configuration

> **Start Here — two-pass flow:** This configuration requires two passes between Okta and Aembit. First, complete the Okta app creation steps (steps 1–4 below) to generate the app's Metadata URL. Then go to Aembit (**Administration → Identity Providers → + New → SAML 2.0**), enter the Metadata URL, and retrieve the auto-generated **Aembit SP Entity ID** and **Aembit SSO URL**. Finally, return to the Okta app and paste those values into the **Single sign-on URL** and **Audience URI** fields. Do not open the Aembit dialog before completing the Okta steps — the SP Entity ID and SSO URL fields will be empty until the Metadata URL is entered.

### Okta Setup

1. In the Okta Admin Console, navigate to **Applications → Applications** and click **Create App Integration**
   - **Sign-in method**: SAML 2.0
   - Click **Next**

2. Enter an app name (e.g., `Aembit SSO`) and click **Next**

3. On the **Configure SAML** tab:
   - **Single sign-on URL**: paste `{{AEMBIT_SSO_URL}}` — copy this from the Aembit Identity Provider dialog (see the Start Here note above)
   - **Audience URI (SP Entity ID)**: paste `{{AEMBIT_SP_ENTITY_ID}}`
   - Under **Attribute Statements**, add the following mappings (required for JIT user creation):

     | Name | Value |
     |------|-------|
     | `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress` | `user.email` |
     | `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname` | `user.firstName` |
     | `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname` | `user.lastName` |

   - Under **Group Attribute Statements**, add a mapping named `groups` filtered to the groups that should have Aembit access (or use a filter that matches all relevant groups)

4. Click **Finish**

5. Open the newly created app, navigate to the **Sign On** tab, and copy the **Metadata URL** (`{{OKTA_SAML_METADATA_URL}}`)

6. Assign the app to the users or groups that should have access to Aembit

## Aembit Configuration

1. In the Aembit Management Console, navigate to **Administration → Identity Providers** and click **+ New**
   - **Name**: descriptive label (e.g., `Okta SSO`)
   - **Identity Provider Type**: select **SAML 2.0**
   - **Metadata URL**: paste `{{OKTA_SAML_METADATA_URL}}`
   - After entering the Metadata URL, Aembit displays the **Aembit SP Entity ID** and **Aembit SSO URL** — copy these back to the Okta SAML app configuration

2. Configure the **Mappings** tab to assign Aembit roles based on Okta group membership:
   - **Okta Group**: enter the exact Okta group name (e.g., `aembit-admins`)
   - **Aembit Role**: select the role to assign (e.g., **Administrator**, **Read Only**)

3. Click **Save**

## Verification

- The Identity Provider appears as **Active** in **Administration → Identity Providers**
- Navigate to the Aembit login page — a **Sign In with Okta** button appears
- Log in using an Okta user assigned to the app and confirm successful authentication
- Navigate to **Administration → Users** and confirm the Okta user appears with the expected role

## Troubleshooting

- **Sign In with Okta button does not appear:** The Identity Provider was not saved successfully, or the Aembit subscription does not support SSO (Teams or Enterprise required). Check **Administration → Identity Providers** for errors
- **SAML assertion rejected:** The **Audience URI** in the Okta app does not match the **Aembit SP Entity ID**. Edit the Okta SAML app and paste the exact value from the Aembit dialog
- **User created without a role (Read Only by default):** The group attribute statement is missing or the group name does not match any configured Mapping. Verify the `groups` attribute statement is configured in the Okta SAML app and that the Mappings tab has entries matching the user's Okta group names
- **Users cannot log in after SSO is enabled:** If Sign-On Policy enforcement is also enabled (see Sign-On Policies module), users without an Okta assignment cannot authenticate. Assign the Okta app to all Aembit users before enabling SSO enforcement
