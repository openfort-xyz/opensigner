![OpenSigner](https://github.com/openfort-xyz/opensigner/blob/main/.github/banner.png)

# OpenSigner

Non-custodial private key management infrastructure.

[Video Playground Walkthrough](https://youtu.be/Fwe5cIQNKos)

## Documentation

For documentation and guides, visit [opensigner.dev](https://opensigner.dev).

## Development

Clone the repository with:

```shell
git clone https://github.com/openfort-xyz/opensigner.git
```

## WIP: Build

We currently depend on the unpublished `crypto-js`, so there's some glue code in the `Makefile`.

Build the project with:

```bash
make clean build
```

> [!WARNING]
> The clean build will take a long time since the auth service depends on BetterAuth,
> which uses `@better-auth/cli@latest` for migrations, which takes a long time to install.

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
