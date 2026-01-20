# OpenSigner Docs

Generate docs using [d2](https://d2lang.com/tour/install) and [vocs](https://vocs.dev/docs).

## Setup

### D2

Follow the steps in the D2 tour: [https://d2lang.com/tour/install](https://d2lang.com/tour/install).

### Vocs

```shell
yarn
```

## Build

```shell
yarn build
# Or if working from the root package,
yarn docs:build
```

## Serve

```shell
yarn dev
# Or if working from the root package,
yarn docs:dev
```


## Development

Make sure to install the [d2 vscode extension](https://d2lang.com/tour/vscode/) if you intend to edit the diagrams.

The most relevant files and directories are:

- `pages/` - contains the actual content of the web pages in markdown (`.mdx`).
- `sidebar.ts` - contains an index with every page so users can access them from the sidebar. Pages need to be added manually.
- `public/` - everything in this directory is publicly exposed as a static file from the root path. E.g., `public/manifest.json`
can be retrived through the URL `https://our.endpoint.example/manifest.json`.
- `diagrams/` - contains every `d2` diagram used in the documentation. They are converted to `.svg` diagrams and put in
`public/diagrams` when building the project. Multi-frame diagrams (those with `scenarios` in their `.d2` file contents)
generate multiple `.svg` output files for a single `.d2` input file.
- `public/swagger` - contains every swagger definition present in the documentation.
- `lib/` - contains the `tsx` files used to render dynamic content in the pages (`.mdx` files).
