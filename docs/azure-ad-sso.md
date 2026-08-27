# Microsoft Entra ID (Azure AD) SSO

The ITAM API implements the OIDC authorization-code flow itself (see
`backend/internal/auth/azure.go`) - there is no separate auth proxy. It
exchanges the code, verifies the `id_token`'s signature against Microsoft's
published keys (cached per tenant), and issues its own access/refresh tokens.

## Architecture

```
Browser / mobile app
  │── password ────▶ API POST /auth/login
  │── Microsoft ───▶ API GET /auth/azure/start
  │                       │
  │                       ▼
  │                  Microsoft login
  │                       │
  │                       ▼
  │                  API GET /auth/azure/callback  (registered in Entra)
  │                       │
  └◀── redirect with #access_token ───┘
```

## Entra app registration

1. Open [portal.azure.com](https://portal.azure.com) → **Azure Active Directory** → **App registrations** → **New registration**.
2. Add a **Web** redirect URI pointing at the ITAM API:

   ```
   http://localhost:5607/auth/azure/callback
   ```

   Production: `https://<api-host>/auth/azure/callback`

   This exact value is also shown on **Admin → SSO** in the app.

3. Under **Certificates & secrets**, create a client secret.
4. Under **Token configuration**, ensure `email`, `openid`, `profile` are available (defaults are usually fine).

## Enable in the app

Go to **Admin → SSO** and fill in:

- **Tenant ID** - your Entra directory (tenant) ID
- **Client ID** - the app registration's Application (client) ID
- **Client secret** - the value created above
- **Enable Microsoft sign-in**

Save. No `deploy/.env` changes or container restarts needed - these are
stored in the database (`meta.settings`), with the client secret encrypted
at rest via Vault, and take effect on the next login attempt.

## Clients

| Client | Microsoft login URL | Return URL |
|---|---|---|
| Web app | API `/auth/azure/start?redirect_to=<origin>` | Same origin |
| Flutter web | Same | Its own origin |
| Android native | Same, via system browser | `itam://sso-callback` |

The **Sign in with Microsoft** button appears when `GET /auth/sso/status`
reports the `azure_enabled` setting is on.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| No Microsoft button | Azure SSO not enabled or incomplete config | Check **Admin → SSO** |
| Redirect URI mismatch | Entra URI isn't the API's `/auth/azure/callback` | Match it exactly, including scheme/host |
| Sign-in works but no ITAM access | User not provisioned / no role | Assign roles in **IAM**; the account matching `ADMIN_EMAIL` gets superuser on first login |
| Android does not return to app | Deep link not registered on the OS side | Rebuild app; verify the `itam://sso-callback` scheme is set up in the Android manifest |

## Secret rotation

1. Create a new secret in Entra → **Certificates & secrets**.
2. Paste it into **Admin → SSO → Client secret** and save.

Existing sessions remain valid until their access token expires.
