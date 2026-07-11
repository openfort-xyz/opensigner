# Hot Storage

Sample hot storage service (Go). Validates JWTs against the auth service's
JWKS endpoint, stores AES-256-GCM-encrypted device key shares in Postgres,
and serves the device/account API defined in
[the docs](https://opensigner.dev/apis/hot_storage).

## Multi-factor authentication

Hot share retrieval (`/v1/devices/init`, `/v1/devices/{deviceId}`,
`/v2/devices/recover`) is protected by JWT only until a user enrolls an MFA
method. After enrollment, those endpoints answer `403 {"error":"mfa_required"}`
unless the request comes from a device holding an MFA session.

Supported methods:

| Method    | How                                                          |
| --------- | ------------------------------------------------------------ |
| `totp`    | RFC 6238, 30-second codes, `otpauth://` provisioning URI     |
| `sms`     | 6-digit codes through an `SmsProvider` (log for dev, Twilio) |
| `passkey` | WebAuthn; the ceremony runs in the app embedding the iframe  |

Behavior:

- Enrollment is user-scoped: it gates share retrieval for **all** of the
  user's signers, and stays active until the user explicitly unenrolls.
- Verifying a challenge grants a session (default 15 min) scoped to a device
  fingerprint (User-Agent + IP hash).
- Activating a method revokes every other cached session, so all other
  active sessions must verify again.
- Challenges expire (default 5 min), allow limited attempts (default 4), and
  can be cancelled — which rejects the pending operation.
- Enroll/unenroll endpoints are themselves MFA-gated once a method exists, so
  a stolen JWT cannot add or remove methods.

The passkey relying party is the **parent application's domain**, not the
iframe's: `navigator.credentials` is unavailable in React Native webviews and
restricted in cross-origin iframes on WebKit, so the WebAuthn ceremony must
run in the embedding app.

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `AUTH_SERVER_URL` | — (required) | Auth service base URL for JWKS |
| `DB_HOST` / `DB_PORT` / `DB_NAME` / `DB_USER` / `DB_PASS` | — (required) | Postgres connection |
| `DB_SSLMODE` | `require` | Postgres SSL mode |
| `SHARE_ENCRYPTION_KEY` | — (required) | 32-byte hex key for share/secret encryption |
| `ALLOWED_ORIGINS` | `http://localhost:7050,http://localhost:7051` | CORS allowlist |
| `SMS_PROVIDER` | `log` | `log` (dev, prints codes) or `twilio` |
| `TWILIO_ACCOUNT_SID` / `TWILIO_AUTH_TOKEN` / `TWILIO_FROM_NUMBER` | — | Required when `SMS_PROVIDER=twilio` |
| `WEBAUTHN_RP_ID` | `localhost` | Passkey relying party ID (parent app domain) |
| `WEBAUTHN_RP_ORIGINS` | `http://localhost:7051` | Comma-separated allowed WebAuthn origins |
| `WEBAUTHN_RP_NAME` | `OpenSigner` | Relying party display name |
| `MFA_SESSION_TTL_MINUTES` | `15` | MFA session validity |
| `MFA_CHALLENGE_TTL_MINUTES` | `5` | Challenge code expiry |
| `MFA_MAX_ATTEMPTS` | `4` | Attempts per challenge |

## Development

```sh
cd sample
go build ./...
go test ./...
```
