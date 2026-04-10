## Prerequisites

- Okta SSO Identity Provider configured and verified (complete the Okta SSO module first)
- At least one non-SuperAdmin user with an Aembit account, assigned to the Okta app
- Aembit tenant with Administrator access and the **Sign-On Policy** permission

## Values Reference

No customer-specific placeholder values are required for this module.

## Aembit Configuration

1. Navigate to **Administration → Sign-On Policy**

2. Enable **Require Single Sign-On**:
   - Toggle the setting on
   - Click **Save**
   - Note: SuperAdmins retain native sign-in access even after SSO enforcement is enabled

3. Optionally enable **Require MFA for Native Sign-In** in addition to SSO enforcement:
   - Toggle the setting on
   - Click **Save**

   > **Note:** After enabling MFA enforcement, notify SuperAdmin users that they have 24 hours to configure MFA before native sign-in is blocked for their accounts.

## Verification

- Log out of Aembit and attempt to sign in with email and password as a non-SuperAdmin user — confirm the login is rejected or redirected to Okta
- Log in as the same user via the **Sign In with Okta** button — confirm successful authentication
- Log in as a SuperAdmin user using email and password — confirm SuperAdmin access is not blocked by SSO enforcement
- If MFA enforcement is also enabled, log in as a SuperAdmin with email/password and confirm an MFA prompt appears

## Troubleshooting

- **Non-SuperAdmin users locked out immediately after enabling SSO:** The Okta app is not assigned to those users. In Okta, assign the Aembit SAML/OIDC app to all affected users before enabling SSO enforcement
- **SuperAdmin accounts locked after enabling MFA enforcement:** The 24-hour grace period expired without MFA configuration. A tenant Owner or another SuperAdmin must unlock the account from **Administration → Users** and provide the user time to configure MFA
- **Sign-On Policy settings are grayed out:** Your Aembit account does not have the **Sign-On Policy** permission, or the tenant is on a Professional plan (SSO enforcement requires Teams or Enterprise)
- **SSO enforcement blocks all logins:** If the Okta Identity Provider is misconfigured or the Okta app is down, SSO enforcement may lock all non-SuperAdmin users. SuperAdmins can still log in with email/password to disable enforcement
