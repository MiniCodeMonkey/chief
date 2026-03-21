# Contributing to Chief

Thanks for your interest in contributing to Chief! Here's how to get started.

## Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (for end-to-end testing)
- [golangci-lint](https://golangci-lint.run/welcome/install-local/) (optional, for linting)

## Getting Started

```bash
git clone https://github.com/minicodemonkey/chief.git
cd chief
make build
```

## Development Workflow

```bash
make build       # Build the binary to ./bin/chief
make test        # Run all tests
make lint        # Run linters
make fmt         # Format code
make vet         # Run go vet
make run         # Build and run the TUI
```

## Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b my-feature`)
3. Make your changes
4. Run `make test` and `make lint` to verify
5. Commit using [conventional commits](https://www.conventionalcommits.org/) (e.g. `feat:`, `fix:`, `docs:`)
6. Open a pull request against `main`

## Reporting Bugs

Open an issue on [GitHub Issues](https://github.com/minicodemonkey/chief/issues) with:

- Steps to reproduce
- Expected vs actual behavior
- Chief version (`chief --version` or `git describe --tags`)

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](README.md#license).
