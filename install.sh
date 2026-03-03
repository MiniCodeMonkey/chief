#!/bin/sh
# Melliza Install Script
# https://github.com/minicodemonkey/melliza
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/MiniCodeMonkey/melliza/refs/heads/main/install.sh | sh
#
# Or with a specific version:
#   curl -fsSL https://raw.githubusercontent.com/MiniCodeMonkey/melliza/refs/heads/main/install.sh | sh -s -- --version v0.1.0
#
# This script:
#   - Detects OS (darwin/linux) and architecture (amd64/arm64)
#   - Downloads the correct binary from GitHub releases
#   - Verifies checksum before installing
#   - Installs to /usr/local/bin or ~/.local/bin (if no sudo)
#   - Is idempotent (safe to run multiple times)

set -e

# Colors for output (disabled if not a tty)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# Configuration
GITHUB_REPO="minicodemonkey/melliza"
BINARY_NAME="melliza"
VERSION=""

# Print colored message
info() {
    printf "${BLUE}==>${NC} %s\n" "$1"
}

success() {
    printf "${GREEN}==>${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}Warning:${NC} %s\n" "$1"
}

error() {
    printf "${RED}Error:${NC} %s\n" "$1" >&2
    exit 1
}

# Detect OS
detect_os() {
    OS="$(uname -s)"
    case "$OS" in
        Darwin)
            echo "darwin"
            ;;
        Linux)
            echo "linux"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            error "Windows is not supported by this install script. Please download from GitHub releases."
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac
}

# Detect architecture
detect_arch() {
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64|amd64)
            echo "amd64"
            ;;
        arm64|aarch64)
            echo "arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac
}

# Get latest version from GitHub
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | \
            grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | \
            grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download file
download() {
    url="$1"
    output="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$output"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download and extract content to stdout
download_content() {
    url="$1"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$url"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Verify checksum
verify_checksum() {
    archive="$1"
    expected_checksum="$2"

    if command -v sha256sum >/dev/null 2>&1; then
        actual_checksum=$(sha256sum "$archive" | cut -d ' ' -f 1)
    elif command -v shasum >/dev/null 2>&1; then
        actual_checksum=$(shasum -a 256 "$archive" | cut -d ' ' -f 1)
    else
        warn "Neither sha256sum nor shasum found. Skipping checksum verification."
        return 0
    fi

    if [ "$actual_checksum" != "$expected_checksum" ]; then
        error "Checksum verification failed!\nExpected: $expected_checksum\nActual:   $actual_checksum"
    fi

    return 0
}

# Find installation directory
find_install_dir() {
    # Try /usr/local/bin first (requires sudo on most systems)
    if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
        echo "/usr/local/bin"
        return 0
    fi

    # Check if we can use sudo
    if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
        echo "/usr/local/bin"
        return 0
    fi

    # Fall back to ~/.local/bin
    LOCAL_BIN="$HOME/.local/bin"
    if [ ! -d "$LOCAL_BIN" ]; then
        mkdir -p "$LOCAL_BIN"
    fi
    echo "$LOCAL_BIN"
}

# Check if directory is in PATH
check_path() {
    dir="$1"
    case ":$PATH:" in
        *":$dir:"*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Install binary
install_binary() {
    src="$1"
    install_dir="$2"

    # Check if we need sudo
    if [ -w "$install_dir" ]; then
        cp "$src" "$install_dir/$BINARY_NAME"
        chmod +x "$install_dir/$BINARY_NAME"
    else
        info "Installing to $install_dir requires elevated permissions..."
        sudo cp "$src" "$install_dir/$BINARY_NAME"
        sudo chmod +x "$install_dir/$BINARY_NAME"
    fi
}

# Parse command line arguments
parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --version|-v)
                VERSION="$2"
                shift 2
                ;;
            --help|-h)
                cat <<EOF
Melliza Install Script

Usage:
    curl -fsSL https://raw.githubusercontent.com/MiniCodeMonkey/melliza/refs/heads/main/install.sh | sh
    curl -fsSL https://raw.githubusercontent.com/MiniCodeMonkey/melliza/refs/heads/main/install.sh | sh -s -- --version v0.1.0

Options:
    --version, -v VERSION    Install a specific version (e.g., v0.1.0)
    --help, -h               Show this help message

Environment Variables:
    MELLIZA_INSTALL_DIR        Override installation directory
EOF
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                ;;
        esac
    done
}

# Main installation function
main() {
    parse_args "$@"

    info "Installing Melliza..."

    # Detect platform
    OS=$(detect_os)
    ARCH=$(detect_arch)
    info "Detected platform: ${OS}/${ARCH}"

    # Get version
    if [ -z "$VERSION" ]; then
        info "Fetching latest version..."
        VERSION=$(get_latest_version)
        if [ -z "$VERSION" ]; then
            error "Failed to determine latest version. Please specify a version with --version"
        fi
    fi
    info "Version: $VERSION"

    # Construct download URLs
    # Version without 'v' prefix for archive name
    VERSION_NUM="${VERSION#v}"
    ARCHIVE_NAME="${BINARY_NAME}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"
    CHECKSUMS_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/checksums.txt"

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    # Download checksums
    info "Downloading checksums..."
    CHECKSUMS_FILE="$TMP_DIR/checksums.txt"
    if ! download "$CHECKSUMS_URL" "$CHECKSUMS_FILE" 2>/dev/null; then
        warn "Could not download checksums file. Proceeding without verification."
        CHECKSUMS_FILE=""
    fi

    # Download archive
    info "Downloading $ARCHIVE_NAME..."
    ARCHIVE_PATH="$TMP_DIR/$ARCHIVE_NAME"
    if ! download "$DOWNLOAD_URL" "$ARCHIVE_PATH"; then
        error "Failed to download $DOWNLOAD_URL"
    fi

    # Verify checksum
    if [ -n "$CHECKSUMS_FILE" ] && [ -f "$CHECKSUMS_FILE" ]; then
        info "Verifying checksum..."
        EXPECTED_CHECKSUM=$(grep "$ARCHIVE_NAME" "$CHECKSUMS_FILE" | cut -d ' ' -f 1)
        if [ -n "$EXPECTED_CHECKSUM" ]; then
            verify_checksum "$ARCHIVE_PATH" "$EXPECTED_CHECKSUM"
            success "Checksum verified!"
        else
            warn "Checksum for $ARCHIVE_NAME not found in checksums file"
        fi
    fi

    # Extract archive
    info "Extracting archive..."
    tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"

    # Find installation directory
    if [ -n "$MELLIZA_INSTALL_DIR" ]; then
        INSTALL_DIR="$MELLIZA_INSTALL_DIR"
        if [ ! -d "$INSTALL_DIR" ]; then
            mkdir -p "$INSTALL_DIR"
        fi
    else
        INSTALL_DIR=$(find_install_dir)
    fi

    # Install binary
    info "Installing to $INSTALL_DIR..."
    install_binary "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR"

    # Verify installation
    if [ -x "$INSTALL_DIR/$BINARY_NAME" ]; then
        success "Melliza installed successfully to $INSTALL_DIR/$BINARY_NAME"

        # Check if install dir is in PATH
        if ! check_path "$INSTALL_DIR"; then
            warn "$INSTALL_DIR is not in your PATH"
            echo ""
            echo "Add it to your PATH by adding this line to your shell profile:"
            echo ""
            echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
            echo ""
        fi

        # Show version
        echo ""
        "$INSTALL_DIR/$BINARY_NAME" --version 2>/dev/null || true
    else
        error "Installation failed. Binary not found at $INSTALL_DIR/$BINARY_NAME"
    fi
}

# Run main
main "$@"
