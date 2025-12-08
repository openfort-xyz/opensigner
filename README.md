![OpenSigner](https://github.com/openfort-xyz/opensigner/blob/main/.github/banner.png)

<p align="center">
  <h2 align="center">
    OpenSigner
  </h2>

  <p align="center">
    Non-custodial private key management infrastructure.
    <br />
    <a href="https://opensigner.dev"><strong>Learn more »</strong></a>
    <br />
    <br />
    <a href="https://t.me/openfort">Telegram</a>
    · 
    <a href="https://youtu.be/Fwe5cIQNKos">Video Walkthrough</a>
    · 
    <a href="https://github.com/openfort-xyz/opensigner/issues">Issues</a>
  </p>

</p>


## About the Project

OpenSigner is an open-source, self-hostable key management stack for non-custodial wallets. It lets you issue, store, and recover cryptographic keys for your users without ever taking custody of those keys yourself. Keys are split into shares (device, hot, and cold storage) using Shamir’s Secret Sharing and only reconstructed ephemerally in memory when a signature is needed, then discarded. You can plug OpenSigner into your existing auth (OIDC, passkeys, email, etc.) and create wallets on networks like Ethereum and Solana without locking yourself into a single provider. 

### Why OpenSigner

Most “embedded wallet” solutions still hold or control user keys behind closed, SaaS-only infrastructure, which creates single points of failure and strong vendor lock-in. With OpenSigner, the key management layer is fully open-source and self-hostable, so you can run it on your own infra, start on Openfort’s cloud and migrate later, or mix both. Threshold cryptography and key sharding reduce the blast radius of breaches, while a vendor-neutral architecture keeps wallets non-custodial and portable across providers. 

## Contribution

OpenSigner is a free and open-source project licensed under the MIT License. You’re free to run it, modify it, and deploy it in your own stack, including production environments. 

You can help drive its development by:

- Contributing code, tests, and docs via pull requests to the [OpenSigner repository](https://github.com/openfort-xyz/opensigner).
- Suggesting new features, reporting bugs, and sharing feedback through GitHub issues.

For contribution details, please refer to the `CONTRIBUTING.md` file in the repository.

## Security

If you discover a security vulnerability within OpenSigner, please email **security@openfort.xyz**. 

All reports are reviewed promptly, and issues will be addressed as quickly as possible. Responsible disclosures are highly appreciated and will be acknowledged where appropriate.


## Development

Clone the repository with:

```shell
git clone https://github.com/openfort-xyz/opensigner.git
```

Build the project with:

```bash
make clean build
```

> [!WARNING]
> The clean build will take a some time since the auth service depends on BetterAuth,
> which uses `@better-auth/cli@latest` for migrations, which takes a some time to install.

Run it with:

```bash
make run
```

The following ports will be accessible from the host:

- `7050`: iframe
- `7051`: iframe-enabled page sample
- `7052`: auth service
- `7053`: cold storage
- `7054`: hot storage
- `7055`: docs (not included in `make run`, but `make docs`)

To run only some specific services (e.g. you already have a database running, or an auth service, or...),
run the following command removing the services you don't want to start:

```shell
docker-compose up postgres mysql auth_service iframe iframe-sample hot_storage cold_storage docs
```

The containers being run through our docker-compose setup can be configured through environment variables,
e.g. set `COLD_STORAGE_DB_HOST` before running the `make` or `docker-compose` command.

We also provide an additional file called `docker-compose.map.db.ports.yml` that maps the internal postgres and mysql ports to `7056` and `7057` respectively,
it can be invoked via
```shell
docker-compose -f docker-compose.yml -f docker-compose.map.db.ports.yml up --build
```

For the full reference, check out [`docker-compose.yml`](/openfort-xyz/opensigner/blob/main/docker-compose.yml).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
