# RFC: Cloud Backup Recovery (iCloud / Google Drive)

**Status**: Draft
**Author**:
**Created**: 2026-07-11

## Problem Statement

OpenSigner's user-entropy recovery methods put the burden of custody on the user: passwords are
forgotten, and passkeys are lost together with the device ecosystem they live in. Automatic
recovery removes that burden but shifts trust to the project (mitigated, not eliminated, by
Shield OTP). Mainstream wallets (Coinbase Wallet, Privy, Web3Auth) solve this with **cloud
backup**: recovery material stored in the user's own iCloud or Google Drive, protected by an
account the user already maintains and can't easily lose. OpenSigner should offer the same:
a recovery method backed by the user's Apple iCloud or Google Drive.

## Background

Keys are split 2-of-3 (device share, hot share, cold share). The cold share — the *recovery
share* — is stored encrypted in Shield. Each recovery method is defined by **who owns the
entropy that encrypts the cold share** and **how the user proves access to it**:

| Method          | Cold Share Encryption          | Entropy Owner | Access Proof                    |
| --------------- | ------------------------------ | ------------- | ------------------------------- |
| **Password**    | Argon2 + AES-CBC (client-side) | User          | Knows password                  |
| **Automatic**   | AES-GCM (server-side)          | Project/Admin | JWT (+ optional Shield OTP)     |
| **Passkey PRF** | PRF + AES-CBC (client-side)    | User          | Biometric/PIN on enrolled device |

Cold share encryption and decryption for user-entropy methods happen inside the iframe; the
plaintext share and the derived key never leave the client.

## Proposal

Add a fourth row to that table:

| Method           | Cold Share Encryption          | Entropy Owner        | Access Proof                     |
| ---------------- | ------------------------------ | -------------------- | -------------------------------- |
| **Cloud backup** | Argon2 + AES-CBC (client-side) | User (cloud account) | Signs in to iCloud / Google Drive |

A **cloud recovery secret** — 32 random bytes, generated client-side — is stored in the user's
personal cloud storage. The cold share stays in Shield, encrypted client-side with a key derived
from that secret. Neither half is useful alone:

- The cloud account alone yields a random secret, but the ciphertext sits in Shield behind the
  project's auth.
- Shield alone (or Shield + a stolen JWT) yields ciphertext that only the cloud-held secret
  can decrypt.

### Design decision 1: store the *secret* in the cloud, not the share

Two variants were considered:

| | **A. Secret in cloud, share in Shield (chosen)** | B. Share itself in cloud |
| --- | --- | --- |
| Iframe changes | **None** (reuses the password pipeline) | New storage backend + flows |
| Shield changes | **None** (user-entropy share, as with password) | Bypassed entirely |
| Cloud data leak alone | Useless random bytes | One Shamir share (still 1-of-3, but strictly worse) |
| Shield availability needed for recovery | Yes | No |
| Works with Shield OTP / encryption sessions | Yes | No |

Variant B's only advantage is recovery without Shield, at the cost of a parallel storage
protocol and losing every Shield-side control. Variant A is a pure re-parameterization of
password recovery and is what Privy and Web3Auth ship. **Choose A.**

### Design decision 2: the cloud secret is a machine-generated "password"

The iframe already implements: user secret → Argon2 (12 iterations, 64 MiB, 128-bit salt) →
256-bit key → AES-CBC over the cold share. Cloud backup feeds the base64-encoded 32-byte cloud
secret through this exact pipeline as if it were a password.

- **Zero iframe/protocol changes**: `setRecoveryMethod` and `recover` are called with the
  existing password fields, carrying the cloud secret.
- Argon2 over a 256-bit random input adds no security (the input is already at key strength)
  but costs one KDF invocation and keeps a single audited code path. Accepted trade-off.
- Shield stores the share exactly as it does for password recovery (`entropy: user`).

What *is* new is everything around that call: cloud storage providers in the parent app,
enrollment/recovery orchestration, and metadata so apps know which flow to run.

### Design decision 3: cloud access happens in the parent app, not the iframe

Same constraint that drove passkey MFA: the iframe runs in browsers and React Native webviews.
Google OAuth popups and native iCloud APIs are unavailable or broken inside a cross-origin
iframe, and `navigator`-level APIs don't exist in RN webviews. The **parent application**
performs the cloud ceremony and passes the secret into the iframe over the existing penpal
channel — precisely how passkey PRF keys and recovery passwords flow today.

Consequence: cloud provider integrations live in the client SDKs (JS, React Native, Swift,
Unity), packaged behind one interface:

```ts
interface CloudBackupProvider {
  /** e.g. "google-drive" | "icloud" */
  readonly id: string;
  /** Whether this provider can run on the current platform. */
  isAvailable(): Promise<boolean>;
  /** Persist the secret for this signer; must verify by reading back. */
  store(signerId: string, secret: Uint8Array): Promise<void>;
  /** Fetch the secret; throws NotFound if absent. */
  retrieve(signerId: string): Promise<Uint8Array>;
  /** Best-effort removal on method switch. */
  remove(signerId: string): Promise<void>;
}
```

### Providers

| | **Google Drive** | **Apple iCloud** |
| --- | --- | --- |
| Where | `appDataFolder` (hidden, app-scoped) | iCloud Keychain (synchronizable item); CloudKit private DB as fallback |
| Auth | OAuth 2.0, scope `drive.appdata` only | OS-level (signed-in Apple ID) |
| Web | ✅ Google Identity Services token client (popup) | ❌ not offered (CloudKit JS possible; see open questions) |
| React Native | ✅ Google Sign-In SDK + Drive REST | ✅ native module (Keychain w/ `kSecAttrSynchronizable`) |
| Swift SDK | ✅ | ✅ |
| Survives device loss | Yes (any device, sign in to Google) | Yes (any Apple device on the same Apple ID) |
| E2E encrypted by provider | No (Google can read appdata) — harmless: content is a bare secret whose ciphertext lives in Shield | Yes (iCloud Keychain) |

Notes:
- `drive.appdata` grants access **only** to the app's own hidden folder — the app never sees
  user files, and other apps never see the secret. OAuth access tokens are held in memory for
  the duration of the ceremony and never sent to any OpenSigner service.
- The Google OAuth client ID belongs to the **developer's application** (parent app origin),
  consistent with decision 3. Secrets are therefore scoped per app, like passkeys are scoped
  per RP domain — the same isolation trade-off already accepted for passkey recovery.
- iCloud Keychain is preferred over iCloud Drive files on Apple platforms: end-to-end
  encrypted, synced, and not user-deletable by casual file management.

### Cloud file format (Google Drive)

One file per signer, name `opensigner-recovery-<signerId>.json`:

```json
{
  "version": 1,
  "signerId": "sig_...",
  "secret": "<base64, 32 bytes>",
  "createdAt": 1783800000
}
```

Keychain equivalent: service `opensigner-recovery`, account `<signerId>`, value = secret.
Listing the appDataFolder (or keychain service) enumerates every signer recoverable from this
cloud account. `version` allows envelope evolution (e.g. wrapped secrets, see open questions).

### Enrollment flow

```
Parent app                     Cloud                        iframe                    Shield
    |  1. sign in / authorize    |                             |                         |
    |--------------------------->|                             |                         |
    |  2. generate 32-byte secret (crypto.getRandomValues)     |                         |
    |  3. store + read-back verify                             |                         |
    |--------------------------->|                             |                         |
    |  4. setRecoveryMethod(previous method proof,             |                         |
    |     new "password" = base64(secret))                     |                         |
    |------------------------------------------------------->  |  re-encrypt cold share  |
    |                            |                             |------------------------>|
    |  5. record metadata {method: cloud, provider}            |                         |
```

Ordering is deliberate: the secret is durably in the cloud **and verified by read-back**
*before* Shield's copy is re-encrypted. A failure at step 3 leaves the previous method intact;
a failure at step 4 leaves an orphaned (harmless) cloud file that the next attempt overwrites.

### Recovery flow

```
Parent app                     Cloud                        iframe                 Shield / Hot storage
    |  1. read metadata → method=cloud, provider=google        |                         |
    |  2. sign in / authorize    |                             |                         |
    |--------------------------->|                             |                         |
    |  3. retrieve(signerId) → secret                          |                         |
    |<---------------------------|                             |                         |
    |  4. recover(encryptionKey = base64(secret))              |                         |
    |------------------------------------------------------->  | fetch + decrypt cold    |
    |                            |                             | share, fetch hot share, |
    |                            |                             | reconstruct, re-split   |
```

Hot share retrieval remains subject to hot-storage MFA (see the MFA RFC) — cloud backup and
MFA compose: full key recovery on a new device requires cloud account access **and** a valid
JWT **and**, if enrolled, an MFA verification.

### Metadata and discovery

Apps must know, before prompting the user, that a signer uses cloud recovery and with which
provider — otherwise they would show a password prompt for a machine-held secret. Store
non-secret metadata alongside the signer in hot storage:

```
recovery_method:  "cloud"
recovery_details: { "provider": "google-drive" | "icloud", "createdAt": ... }
```

Exposed on the existing account/signer read endpoints (the hosted API already models this as
`recoveryMethod` / `recoveryMethodDetails`). The sample hot_storage service gains a
`recovery_metadata` column set during enrollment. Metadata is advisory UX state, not a
security control.

### Rotation, switching, unenrollment

- **Switch to cloud**: `setRecoveryMethod` with proof of the previous method (existing flow).
- **Switch away / unenroll**: `setRecoveryMethod` to the new method, then best-effort
  `remove()` of the cloud secret. A stale cloud secret is useless once the share is
  re-encrypted under new entropy.
- **Rotate secret**: generate a new secret, `setRecoveryMethod` cloud → cloud. Overwrite the
  file only after the Shield update succeeds (inverse ordering from enrollment: keep the *old*
  working secret until the new one is live, then overwrite; the file's single-slot nature makes
  this a two-file dance — write `…<signerId>.json.new`, flip after success).

### Threat analysis

| Compromise | Attacker obtains | Private key at risk? |
| --- | --- | --- |
| Cloud account only | Random 32-byte secret | No — Shield ciphertext is auth-gated |
| Shield DB only | Cold share ciphertext | No — needs cloud secret |
| Stolen user JWT only | Hot share (MFA-gated), Shield ciphertext | No — needs cloud secret |
| JWT + cloud account | All of the above | **Yes** — equivalent to password recovery with a leaked password; hot-storage MFA is the remaining barrier |
| Project/admin insider | Nothing new (user entropy) | No — strictly better than automatic recovery |
| Google (provider) insider | appdata secret | No — no Shield access; iCloud Keychain variant is E2E encrypted besides |

Residual risks to document for integrators:
- **Cloud account takeover** is the method's root of trust. The secret inherits the user's
  cloud account security (2FA on Google/Apple strongly recommended — and typically already
  enforced by those providers).
- **Data loss**: user deletes app data in Drive / disables iCloud Keychain → recovery method
  lost. Identical failure class to a forgotten password; apps should detect `NotFound` at
  recovery time and route to support / alternate method. (True multi-method redundancy is an
  open question below.)
- **Wrong account**: user signs into a different Google/Apple account → `NotFound`. The UX
  must surface which account the backup was made with (store a non-identifying hint, e.g.
  email hash prefix, in metadata).

### What changes where

| Component | Change |
| --- | --- |
| **iframe** | **None** — cloud secret rides the existing password fields of `setRecoveryMethod` / `recover` |
| **Shield** | **None** — user-entropy share, as password recovery today |
| **Client SDKs** (JS / RN / Swift / Unity) | `CloudBackupProvider` interface + Google Drive and iCloud implementations; enroll/recover orchestration helpers |
| **hot_storage** (this repo's sample) | `recovery_metadata` on the signer + expose on reads |
| **iframe/sample** (this repo's demo) | Google Drive (web) enrollment + recovery demo, like the MFA demo |
| **docs** | New section in `security/recovery-methods`, signup/login action pages, threat analysis row |

### Configuration

No app-level toggles (consistent with the MFA decision). Per-platform provider availability is
determined at runtime by `isAvailable()`. The only integration input is the developer's Google
OAuth client ID (and, on Apple platforms, the app's iCloud entitlement), both of which live in
the parent app — no OpenSigner service configuration.

## Open questions

1. **Secret scope**: per-signer (proposed) or per-user? Per-user gives one cloud object and
   one authorization for all signers, but couples their rotation and blast radius.
2. **Server-wrapped secret**: should the cloud file hold `wrap(secret, project_key)` instead of
   the raw secret, so recovery requires a live project endpoint (defense against cloud+JWT
   compromise, at the cost of a new server dependency and losing "project can disappear"
   neutrality)? Proposed: no for v1; the `version` field leaves room.
3. **iCloud on web**: CloudKit JS + Sign in with Apple could extend iCloud recovery to
   browsers. Heavy integration, rare demand. Proposed: out of scope for v1.
4. **Multiple concurrent recovery methods**: today `setRecoveryMethod` replaces. Cloud backup
   makes "password + cloud" redundancy attractive, but that is a Shield/protocol change (one
   encrypted share per method). Out of scope; worth its own RFC.
5. **Android Block Store** as a third provider (Google's purpose-built key backup) — natural
   later addition behind the same interface.
