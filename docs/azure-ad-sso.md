# Microsoft Entra ID (Azure AD) SSO

All authentication in ITAM goes through **GoTrue** — email/password and Microsoft SSO.
The ITAM API only **validates** GoTrue JWTs; it does not run its own OAuth flow.

## Architecture

```
Browser / mobile app
  │── password ──▶ GoTrue /token
  │── Microsoft ─▶ GoTrue /authorize?provider=azure
  │                      │
  │                      ▼
  │                 Microsoft login
  │                      │
  │                      ▼
  │                 GoTrue /callback  (registered in Entra)
  │                      │
  └◀── redirect with #access_token ──┘
         │
         ▼
    ITAM API (validates JWT, JIT user profile)
```

## Entra app registration

1. Open [portal.azure.com](https://portal.azure.com) → **Azure Active Directory** → **App registrations** → **New registration**.
2. Add a **Web** redirect URI pointing at **GoTrue** (not the ITAM API):

   ```
   http://localhost:5606/callback
   ```

   Production: `https://<auth-host>/callback`

3. Under **Certificates & secrets**, create a client secret.
4. Under **Token configuration**, ensure `email`, `openid`, `profile` are available (defaults are usually fine).

## Enable in deploy

Edit `deploy/.env`:

```env
AZURE_ENABLED=true
AZURE_CLIENT_ID=<application-client-id>
AZURE_CLIENT_SECRET=<secret-value>
AZURE_URL=https://login.microsoftonline.com/<tenant-id>/v2.0
AZURE_REDIRECT_URI=http://localhost:5606/callback

# Origins GoTrue may redirect to after OAuth (hash contains tokens)
GOTRUE_URI_ALLOW_LIST=http://localhost:5608,http://localhost:5609,itam://sso-callback
```

Restart the auth container:

```bash
docker compose restart auth
```

## Clients

| Client | Microsoft login URL | Return URL |
|---|---|---|
| Web app (`:5608`) | GoTrue `/authorize?provider=azure&redirect_to=<origin>` | Same origin |
| Flutter web (`:5609`) | Same | `http://localhost:5609` |
| Android native | Same via system browser | `itam://sso-callback` |

The **Sign in with Microsoft** button appears when GoTrue reports Azure enabled (`GET /auth/sso/status` on the API proxies GoTrue `/settings`).

## Admin UI

**Admin → Metadata → SSO** shows read-only status and setup steps. Azure credentials live in `deploy/.env`, not in the metadata database.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| No Microsoft button | `AZURE_ENABLED=false` or incomplete env | Check `deploy/.env`, restart `auth` |
| Redirect URI mismatch | Entra URI is not GoTrue `/callback` | Use `http://localhost:5606/callback` in dev |
| Sign-in works but no ITAM access | User not provisioned / no role | Assign roles in **IAM**; admin email gets superuser on first login |
| Android does not return to app | Deep link missing | Rebuild app; verify `itam://sso-callback` in `GOTRUE_URI_ALLOW_LIST` |

## Secret rotation

1. Create a new secret in Entra → **Certificates & secrets**.
2. Update `AZURE_CLIENT_SECRET` in `deploy/.env`.
3. `docker compose restart auth`.

Existing sessions remain valid until their JWT expires.
