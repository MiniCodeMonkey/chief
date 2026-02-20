---
description: Install Chief on macOS or Linux via Homebrew, install script, manual download, or from source. Single binary with no runtime dependencies.
---

# Installation

Chief is distributed as a single binary with no runtime dependencies. Choose your preferred installation method below.

## Prerequisites

Before installing Chief, ensure you have **Claude Code CLI** installed and authenticated:

::: code-group

```bash [npm (recommended)]
# Install Claude Code globally
npm install -g @anthropic-ai/claude-code

# Authenticate (opens browser)
claude login
```

```bash [npx (no install)]
# Run directly without installing
npx @anthropic-ai/claude-code login
```

:::

::: tip Verify Claude Code Installation
Run `claude --version` to confirm Claude Code is installed. Chief will not work without it.
:::

### Optional: GitHub CLI (`gh`)

If you want Chief to automatically create pull requests when a PRD completes, install the [GitHub CLI](https://cli.github.com/):

```bash
# macOS
brew install gh

# Linux
# See https://github.com/cli/cli/blob/trunk/docs/install_linux.md

# Authenticate
gh auth login
```

The `gh` CLI is only required for automatic PR creation. All other features work without it.

## Homebrew (Recommended)

The easiest way to install Chief on **macOS** or **Linux**:

```bash
brew install minicodemonkey/chief/chief
```

This method:
- Automatically handles updates via `brew upgrade`
- Installs to `/opt/homebrew/bin/chief` (Apple Silicon) or `/usr/local/bin/chief` (Intel/Linux)
- Works on macOS (Apple Silicon and Intel) and Linux (x64 and ARM64)

### Updating

The easiest way to update is Chief's built-in update command, which works regardless of how you installed:

```bash
chief update
```

If you installed via Homebrew, you can also use:

```bash
brew update && brew upgrade chief
```

Chief automatically checks for updates on startup and notifies you when a new version is available.

## Install Script

Download and install with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/minicodemonkey/chief/main/install.sh | bash
```

The script automatically detects your platform and downloads the appropriate binary.

### Script Options

| Option | Description | Example |
|--------|-------------|---------|
| `--version` | Install a specific version | `--version v0.1.0` |
| `--dir` | Install to a custom directory | `--dir /opt/chief` |
| `--help` | Show all available options | `--help` |

**Examples:**

```bash
# Install a specific version
curl -fsSL https://raw.githubusercontent.com/minicodemonkey/chief/main/install.sh | bash -s -- --version v0.1.0

# Install to a custom directory
curl -fsSL https://raw.githubusercontent.com/minicodemonkey/chief/main/install.sh | bash -s -- --dir ~/.local/bin

# Both options combined
curl -fsSL https://raw.githubusercontent.com/minicodemonkey/chief/main/install.sh | bash -s -- --version v0.1.0 --dir /opt/chief
```

::: info Custom Directory
If you install to a custom directory, make sure it's in your `PATH`:
```bash
export PATH="$HOME/.local/bin:$PATH"
```
Add this to your shell profile (`.bashrc`, `.zshrc`, etc.) to persist it.
:::

## Manual Binary Download

Download the binary for your platform from the [GitHub Releases page](https://github.com/minicodemonkey/chief/releases).

### Platform Matrix

| Platform | Architecture | Binary Name | Notes |
|----------|-------------|-------------|-------|
| macOS | Apple Silicon (M1/M2/M3) | `chief-darwin-arm64` | Recommended for modern Macs |
| macOS | Intel (x64) | `chief-darwin-amd64` | For older Intel-based Macs |
| Linux | x64 (AMD64) | `chief-linux-amd64` | Most common Linux servers |
| Linux | ARM64 | `chief-linux-arm64` | Raspberry Pi 4, AWS Graviton |

### Installation Steps

::: code-group

```bash [macOS Apple Silicon]
# Download the binary
curl -LO https://github.com/minicodemonkey/chief/releases/latest/download/chief-darwin-arm64

# Make it executable
chmod +x chief-darwin-arm64

# Move to a directory in your PATH
sudo mv chief-darwin-arm64 /usr/local/bin/chief
```

```bash [macOS Intel]
# Download the binary
curl -LO https://github.com/minicodemonkey/chief/releases/latest/download/chief-darwin-amd64

# Make it executable
chmod +x chief-darwin-amd64

# Move to a directory in your PATH
sudo mv chief-darwin-amd64 /usr/local/bin/chief
```

```bash [Linux x64]
# Download the binary
curl -LO https://github.com/minicodemonkey/chief/releases/latest/download/chief-linux-amd64

# Make it executable
chmod +x chief-linux-amd64

# Move to a directory in your PATH
sudo mv chief-linux-amd64 /usr/local/bin/chief
```

```bash [Linux ARM64]
# Download the binary
curl -LO https://github.com/minicodemonkey/chief/releases/latest/download/chief-linux-arm64

# Make it executable
chmod +x chief-linux-arm64

# Move to a directory in your PATH
sudo mv chief-linux-arm64 /usr/local/bin/chief
```

:::

::: tip Detect Your Architecture
Not sure which binary you need? Run these commands:
```bash
# macOS
uname -m  # arm64 = Apple Silicon, x86_64 = Intel

# Linux
uname -m  # x86_64 = AMD64, aarch64 = ARM64
```
:::

## Building from Source

Build Chief from source if you want the latest development version or need to customize the build.

### Prerequisites

- **Go 1.21** or later ([install Go](https://go.dev/doc/install))
- **Git** for cloning the repository

### Build Steps

```bash
# Clone the repository
git clone https://github.com/minicodemonkey/chief.git
cd chief

# Build the binary
go build -o chief ./cmd/chief

# Optionally install to your GOPATH/bin
go install ./cmd/chief
```

### Build with Version Info

For a release-quality build with version information embedded:

```bash
go build -ldflags "-X main.version=$(git describe --tags --always)" -o chief ./cmd/chief
```

### Verify the Build

```bash
./chief --version
```

## Verifying Installation

After installing via any method, verify Chief is working correctly:

```bash
# Check the version
chief --version

# View help
chief --help

# Check that Claude Code is accessible
claude --version
```

Expected output:

```
$ chief --version
chief version vX.Y.Z

$ claude --version
Claude Code vX.Y.Z
```

::: warning Troubleshooting
If `chief` is not found after installation:
1. Check that the installation directory is in your `PATH`
2. Open a new terminal window/tab to reload your shell
3. Run `which chief` to see if it's found and where

See the [Troubleshooting Guide](/troubleshooting/common-issues) for more help.
:::

## Next Steps

Now that Chief is installed:

1. **[Quick Start Guide](/guide/quick-start)** - Get running with your first PRD
2. **[How Chief Works](/concepts/how-it-works)** - Understand the autonomous agent concept
3. **[CLI Reference](/reference/cli)** - Explore all available commands
