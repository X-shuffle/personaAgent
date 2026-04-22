# Contributing to PersonaAgent

Thanks for your interest in contributing.

## Development setup

1. Fork this repository and create a feature branch from `main`.
1. Install dependencies:

```bash
go mod tidy
```

1. Copy environment file and configure required variables:

```bash
cp .env.example .env
```

1. Run the server locally:

```bash
go run ./cmd/server
```

1. (Desktop) Run the desktop launcher in another terminal:

```bash
cd apps/desktop
wails dev
```

Optional frontend-only debug:

```bash
cd apps/desktop/frontend
npm install
npm run dev
```

## Contribution guidelines

- Keep PRs focused on one change.
- Follow existing code style and project structure.
- Add or update tests for behavior changes.
- Avoid committing secrets (`.env`, API keys, real credentials).

## Testing

Run full test suite before opening PR:

```bash
go test ./...
```

If your change touches desktop:

```bash
cd apps/desktop/frontend
npm install
npm run build
```

## Commit and PR checklist

- [ ] Code compiles and tests pass
- [ ] New behavior includes tests
- [ ] Documentation updated if needed
- [ ] No secrets or sensitive data committed

## Reporting issues

When opening an issue, include:

- What you expected
- What actually happened
- Steps to reproduce
- Logs/error snippets (if available)

## Code of conduct

Please follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
