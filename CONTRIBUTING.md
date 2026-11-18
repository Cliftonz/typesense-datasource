# Contributing

## Development setup

Requirements: Node.js (see `.nvmrc`), Go, [Mage](https://magefile.org), and Docker.

```bash
npm install
npm run build      # frontend
mage -v            # backend binaries
```

Use `npm run dev` to rebuild the frontend on change.

## Running locally

```bash
docker compose up --build
```

This starts Grafana with the plugin loaded, plus two Typesense servers with API key `test-key`:

| Service | Typesense version | Host port |
| --- | --- | --- |
| `typesense27` | 27.1 | 8107 |
| `typesense30` | 30.2 | 8109 |

## Tests

The Go tests run against both Typesense servers, so start them first.

```bash
go test ./pkg/...
npm run test:ci
```

## Releasing

1. Add the release notes to `CHANGELOG.md`.
2. Run `npm version <major|minor|patch>` and push the tag: `git push origin main --follow-tags`.
3. The `v*` tag triggers `.github/workflows/release.yml`, which builds the plugin, signs it, and creates a draft GitHub release.

Signing needs a repository secret named `GRAFANA_ACCESS_POLICY_TOKEN`: a Grafana Cloud access policy token with realm `<org> (all-stacks)` and scope `plugins:write`. See [Sign a plugin](https://grafana.com/developers/plugin-tools/publish-a-plugin/sign-a-plugin).
