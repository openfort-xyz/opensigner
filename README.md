![OpenSigner](.github/banner.png)

<h1 align="center">OpenSigner</h1>

<p align="center">
  Vendor‑neutral, non‑custodial key management for embedded wallets.
  <br />
  <a href="https://opensigner.dev">Documentation</a>
  ·
  <a href="https://t.me/openfort">Support</a>
  ·
  <a href="https://youtu.be/Fwe5cIQNKos">Video walkthrough</a>
  ·
  <a href="https://github.com/openfort-xyz/opensigner/issues">Issues</a>
</p>

<p align="center">
  <i>OpenSigner is currently being audited by Quantstamp.</i>
</p>

---

## The Problem

Most “embedded wallet” providers still hold or fully control user keys behind closed, SaaS‑only infrastructure.  
That creates single points of failure, vendor lock‑in, and unclear custody: if the vendor goes down, changes terms, or is compromised, your users’ wallets are at risk.

At the same time, building your own key management layer is hard: you need secure key storage, recovery flows, multi‑device support, and a clean way to plug into existing auth — all without ever taking custody of users’ keys.

## The Solution

OpenSigner is an open‑source, self‑hostable wallet key management system for non‑custodial wallets.
It issues, stores, and recovers cryptographic keys for your users while keeping the signing layer completely under your control — and portable across wallet providers.

Core pillars:

1. **Threshold key management** – Private keys are split into shares (device, hot, cold) using Shamir’s Secret Sharing and only reconstructed ephemerally in memory for signing.
2. **Vendor‑neutral architecture** – Separate the key management and signing layer from any specific wallet SaaS so you can start on Openfort’s cloud, self‑host, or migrate between providers without moving keys.
3. **Auth‑agnostic UX** – Plug into your existing auth (OIDC, passkeys, email, etc.) to turn any user account into a non‑custodial wallet.
4. **Multi‑chain by design** – Create and operate wallets on networks like Ethereum and Solana with the same key management stack.

---

## How It Works

At a high level:

- A new user signs in using your existing auth (e.g. OAuth, passkey, email magic link).
- OpenSigner generates a new key and splits it into three shares:
  - **Device share** – stored on the user’s device (via the iframe).
  - **Hot share** – stored in hot storage for liveness.
  - **Cold share** – stored in cold storage for recovery.
- When your app needs a signature, the iframe reconstructs the key in memory using a threshold of shares, signs the payload, and discards the reconstructed key immediately.
- Your app or wallet infrastructure (e.g. Openfort) uses that signature to drive wallet on any chain.

```text
User / App          OpenSigner                    Chains
┌───────────┐   ┌──────────────────────┐   ┌────────────────────┐
│  Frontend │──▶│ Iframe (device share)│   │ Ethereum, Solana…  │
│  Backend  │   │ Hot storage service  │──▶│ Wallet infra (e.g. │
└───────────┘   │ Cold storage service │   │ Openfort)          │
                └──────────────────────┘   └────────────────────┘
```

---

## Getting Started

Clone the repository:

```bash
git clone https://github.com/openfort-xyz/opensigner.git
cd opensigner
```

Configure the required secrets. Compose has no defaults for these and refuses to
start without them, so a deployment cannot silently come up on credentials that are
published in this repository:

```bash
cp .env.example .env
# then fill in every REQUIRED value; each has a generator command in the comments
```

> **Set the database passwords before the first start.** PostgreSQL fixes its
> passwords when the data volume is first created, so changing them later does
> not update an existing volume.

Build the project:

```bash
make build
```

Run the full stack locally:

```bash
make run
```

> `make clean` is **destructive**: it runs `docker compose down --rmi all -v`, and
> the `-v` deletes the volume, wiping the databases. To stop the stack while
> keeping your data, use `docker compose down`.

Services exposed by default:

- `7050`: iframe
- `7051`: iframe-enabled sample page
- `7052`: auth service
- `7053`: cold storage
- `7054`: hot storage
- `7055`: docs (run with `make docs`)

To start a subset of services (for example if you already have a DB or auth), remove services from the `docker compose` command:

```bash
docker compose up postgres auth_service iframe iframe-sample hot_storage cold_storage docs
```

We also provide `docker-compose.map.db.ports.yml` to map the internal Postgres port to host port `7056`:

```bash
docker compose -f docker-compose.yml -f docker-compose.map.db.ports.yml up --build
```

For configuration details, see [`docker-compose.yml`](docker-compose.yml) and the docs.

---

## Features

- **Open‑source & self‑hostable** – MIT‑licensed, production‑ready key management that you can run on your own infrastructure.
- **Non‑custodial by design** – You never hold users’ complete private keys at rest; keys only exist fully in memory during signing.
- **Threshold cryptography** – Shamir’s Secret Sharing with device, hot, and cold key shares to reduce the blast radius of any single compromise.
- **Multi‑chain support** – Create and manage wallets on Ethereum, Solana, and other supported networks without changing your key stack.
- **Auth‑agnostic** – Integrates with your existing auth stack (OIDC, passkeys, email, etc.) so you don’t have to redesign login and onboarding.
- **Vendor‑neutral** – Works with Openfort and other wallet infrastructure providers; swap vendors without re‑issuing keys or changing user addresses.
- **Transparent & auditable** – Fully open repository and docs so your security team can review the implementation and deployment model.
- **Currently under audit** – Ongoing security audit by Quantstamp.


## Security

If you discover a security vulnerability in OpenSigner, please email **security@openfort.xyz**.  
All reports are reviewed promptly, and issues will be addressed as quickly as possible. Responsible disclosures are highly appreciated and will be acknowledged where appropriate.

For system integrity and image verification (e.g. verifying CI‑built images), see the security section in the documentation.


## Contribution

OpenSigner is a free and open‑source project licensed under the MIT License.
You’re welcome to run it, modify it, and deploy it in your own stack — including production environments.

You can help drive its development by:

- Contributing code, tests, and docs via pull requests.
- Suggesting new features, reporting bugs, and sharing feedback through GitHub issues.

See `CONTRIBUTING.md` for more.

## License

This project is licensed under the MIT License – see the [LICENSE](LICENSE) file for details.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting.
