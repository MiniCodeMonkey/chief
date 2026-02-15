#!/bin/bash
# Chief Cloud-Init Setup Script
# https://github.com/MiniCodeMonkey/chief
#
# This script sets up a VPS to run Chief as a systemd service.
# It is designed to be run via cloud-init during VPS provisioning.
#
# Usage (cloud-init user-data):
#   #!/bin/bash
#   curl -fsSL https://raw.githubusercontent.com/MiniCodeMonkey/chief/main/deploy/cloud-init.sh | bash
#
# What this script does:
#   1. Creates a 'chief' user
#   2. Installs the Chief binary
#   3. Installs Claude Code CLI (via npm)
#   4. Creates the workspace directory
#   5. Writes and enables the systemd unit file
#
# After this script runs, you must:
#   1. SSH into the server
#   2. Run: sudo -u chief chief login
#   3. Authenticate Claude Code: sudo -u chief claude
#   4. Start the service: sudo systemctl start chief
#
# This script is idempotent (safe to run multiple times).

set -euo pipefail

GITHUB_REPO="MiniCodeMonkey/chief"
CHIEF_USER="chief"
CHIEF_HOME="/home/${CHIEF_USER}"
WORKSPACE_DIR="${CHIEF_HOME}/projects"
BINARY_PATH="/usr/local/bin/chief"
SERVICE_FILE="/etc/systemd/system/chief.service"

info() {
    echo "==> $1"
}

warn() {
    echo "WARNING: $1"
}

error() {
    echo "ERROR: $1" >&2
    exit 1
}

# Create chief user if it doesn't exist
create_user() {
    if id "${CHIEF_USER}" &>/dev/null; then
        info "User '${CHIEF_USER}' already exists"
    else
        info "Creating user '${CHIEF_USER}'..."
        useradd --create-home --shell /bin/bash "${CHIEF_USER}"
    fi
}

# Install Chief binary
install_chief() {
    info "Installing Chief binary..."
    curl -fsSL "https://raw.githubusercontent.com/${GITHUB_REPO}/main/install.sh" | CHIEF_INSTALL_DIR=/usr/local/bin sh
}

# Install Node.js and Claude Code CLI
install_claude_code() {
    if command -v claude &>/dev/null; then
        info "Claude Code CLI already installed"
        return 0
    fi

    # Install Node.js if not present
    if ! command -v node &>/dev/null; then
        info "Installing Node.js..."
        if command -v apt-get &>/dev/null; then
            curl -fsSL https://deb.nodesource.com/setup_lts.x | bash -
            apt-get install -y nodejs
        elif command -v dnf &>/dev/null; then
            curl -fsSL https://rpm.nodesource.com/setup_lts.x | bash -
            dnf install -y nodejs
        elif command -v yum &>/dev/null; then
            curl -fsSL https://rpm.nodesource.com/setup_lts.x | bash -
            yum install -y nodejs
        else
            warn "Could not install Node.js automatically. Please install it manually."
            return 1
        fi
    fi

    info "Installing Claude Code CLI..."
    npm install -g @anthropic-ai/claude-code
}

# Create workspace directory
create_workspace() {
    if [ -d "${WORKSPACE_DIR}" ]; then
        info "Workspace directory already exists: ${WORKSPACE_DIR}"
    else
        info "Creating workspace directory: ${WORKSPACE_DIR}"
        mkdir -p "${WORKSPACE_DIR}"
    fi
    chown -R "${CHIEF_USER}:${CHIEF_USER}" "${WORKSPACE_DIR}"
}

# Create .chief config directory
create_config_dir() {
    local config_dir="${CHIEF_HOME}/.chief"
    if [ -d "${config_dir}" ]; then
        info "Config directory already exists: ${config_dir}"
    else
        info "Creating config directory: ${config_dir}"
        mkdir -p "${config_dir}"
    fi
    chown -R "${CHIEF_USER}:${CHIEF_USER}" "${config_dir}"
}

# Install and enable systemd service
install_service() {
    info "Installing systemd service..."

    cat > "${SERVICE_FILE}" <<'UNIT'
[Unit]
Description=Chief - Autonomous PRD Agent
Documentation=https://github.com/MiniCodeMonkey/chief
After=network-online.target
Wants=network-online.target
ConditionPathExists=/home/chief/.chief/credentials.yaml

[Service]
Type=simple
User=chief
Group=chief
WorkingDirectory=/home/chief
ExecStart=/usr/local/bin/chief serve --workspace /home/chief/projects --log-file /home/chief/.chief/serve.log
Restart=always
RestartSec=5
Environment=HOME=/home/chief

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=false
ReadWritePaths=/home/chief

[Install]
WantedBy=multi-user.target
UNIT

    systemctl daemon-reload
    systemctl enable chief.service
    info "Service enabled (but NOT started — authentication required first)"
}

# Print post-deploy instructions
print_instructions() {
    echo ""
    echo "============================================"
    echo "  Chief setup complete!"
    echo "============================================"
    echo ""
    echo "Next steps:"
    echo ""
    echo "  1. SSH into this server"
    echo ""
    echo "  2. Authenticate Chief with chiefloop.com:"
    echo "     sudo -u ${CHIEF_USER} chief login"
    echo ""
    echo "  3. Authenticate Claude Code:"
    echo "     sudo -u ${CHIEF_USER} claude"
    echo ""
    echo "  4. Start the Chief service:"
    echo "     sudo systemctl start chief"
    echo ""
    echo "  5. Check service status:"
    echo "     sudo systemctl status chief"
    echo "     sudo journalctl -u chief -f"
    echo ""
    echo "============================================"
}

# Main
main() {
    info "Starting Chief setup..."

    # Check if running as root
    if [ "$(id -u)" -ne 0 ]; then
        error "This script must be run as root (or via cloud-init)"
    fi

    create_user
    install_chief
    install_claude_code
    create_workspace
    create_config_dir
    install_service
    print_instructions

    info "Chief setup complete!"
}

main "$@"
