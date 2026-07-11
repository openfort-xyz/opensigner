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
- Verifying a challenge returns an opaque **session token** (default 15 min).
  The client sends it in the `X-MFA-Session` header on gated requests; only
  the token's hash is stored, and the token — not the device fingerprint — is
  the credential. The fingerprint is recorded for auditing only.
- Activating a method revokes every other cached session, so all other
  sessions must verify again.
- Challenges expire (default 5 min), allow limited attempts (default 4), and
  can be cancelled — which rejects the pending operation.
- Enroll/unenroll endpoints are themselves MFA-gated once a method exists, so
  a stolen JWT cannot add or remove methods.

### Known limitations (sample scope)

This is a reference implementation. Before production, add (tracked with the
existing rate-limiting TODO in `middleware.go`):

- **Rate limiting** on challenge creation and SMS delivery — otherwise TOTP
  codes are brute-forceable over many challenges and SMS endpoints enable
  toll fraud / bombing.
- **First-factor enrollment step-up**: while a user has no method, enrollment
  is gated only by the JWT, so a stolen JWT can enroll an attacker's factor
  and lock the user out. Require an out-of-band confirmation for the first
  method, and notify the user on enrollment.
- `TRUST_PROXY_HEADERS=true` assumes exactly one trusted proxy hop (it reads
  the right-most `X-Forwarded-For` entry). Adjust if your topology differs.

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
| `TRUST_PROXY_HEADERS` | `false` | Honor `X-Forwarded-For` for device fingerprints. Enable only behind a trusted reverse proxy — from direct clients the header is attacker-controlled |

## Development

```sh
cd sample
go build ./...
go test ./...
```
