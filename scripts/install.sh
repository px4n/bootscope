#!/bin/bash
# BootScope Installation Script
# Automatically detects OS and architecture and installs the appropriate binary

set -euo pipefail

# Configuration
REPO="px4n/bootscope"
BINARY_NAME="kubectl-bootscope"
DEFAULT_INSTALL_DIR="/usr/local/bin"
GITHUB_API="https://api.github.com"
GITHUB_URL="https://github.com"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script variables
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
VERSION="${VERSION:-latest}"
VERIFY_CHECKSUM="${VERIFY_CHECKSUM:-true}"
CLEANUP_ON_ERROR=true
TEMP_DIR=""

# Usage information
usage() {
    cat << EOF
BootScope Installation Script

Usage: $0 [OPTIONS]

OPTIONS:
    -h, --help              Show this help message
    -v, --version VERSION   Install specific version (default: latest)
    -d, --install-dir DIR   Installation directory (default: $DEFAULT_INSTALL_DIR)
    -s, --skip-checksum     Skip checksum verification
    --no-cleanup            Don't cleanup on error

ENVIRONMENT VARIABLES:
    INSTALL_DIR             Installation directory
    VERSION                 Version to install
    VERIFY_CHECKSUM         Set to 'false' to skip checksum verification

EXAMPLES:
    # Install latest version
    $0

    # Install specific version
    $0 --version v0.2.0

    # Install to custom directory
    $0 --install-dir ~/.local/bin

EOF
}

# Cleanup function
cleanup() {
    if [ "$CLEANUP_ON_ERROR" = true ] && [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}

# Set up error handling
trap cleanup EXIT

# Error handling function
error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

# Warning function
warn() {
    echo -e "${YELLOW}Warning: $1${NC}" >&2
}

# Info function
info() {
    echo -e "${GREEN}$1${NC}"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -d|--install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        -s|--skip-checksum)
            VERIFY_CHECKSUM="false"
            shift
            ;;
        --no-cleanup)
            CLEANUP_ON_ERROR=false
            shift
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Verify prerequisites
command -v curl >/dev/null 2>&1 || error "curl is required but not installed. Please install curl first."

# Check if kubectl is installed (warning only)
if ! command -v kubectl >/dev/null 2>&1; then
    warn "kubectl is not installed. BootScope is a kubectl plugin and requires kubectl to function."
    echo "  Visit https://kubernetes.io/docs/tasks/tools/ for installation instructions."
    echo ""
fi

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux*)     OS='linux';;
    darwin*)    OS='darwin';;
    msys*|mingw*|cygwin*)
        error "Windows detected. Please use the PowerShell installation script: install.ps1"
        ;;
    *)
        error "Unsupported OS: $OS. Supported systems: Linux, macOS. For other systems, please build from source."
        ;;
esac

# Detect Architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)   ARCH='x86_64';;
    aarch64|arm64)  ARCH='arm64';;
    armv7l)         ARCH='armv7';;
    armv6l)         ARCH='armv6';;
    armhf|arm)      ARCH='armv7';;  # Default ARM to v7
    i386|i686)      ARCH='386';;
    *)
        error "Unsupported architecture: $ARCH. Supported architectures: x86_64, arm64, armv7, armv6, 386"
        ;;
esac

info "BootScope Installer"
echo "  OS: $OS"
echo "  Architecture: $ARCH"
echo "  Install directory: $INSTALL_DIR"
echo "  Version: $VERSION"
echo ""

# Create temporary directory
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Strip 'v' prefix from version if present for filename
VERSION_NUM="${VERSION#v}"

# Construct download filename with version
FILENAME="${BINARY_NAME}-${VERSION_NUM}-${OS}-${ARCH}"
INSTALL_NAME="${BINARY_NAME}"

# Get version information
if [ "$VERSION" = "latest" ]; then
    info "Fetching latest release information..."

    # Use curl with proper options for security
    RELEASE_JSON=$(curl -sS --proto '=https' --tlsv1.2 --max-time 30 --max-redirs 3 \
        -H "Accept: application/vnd.github.v3+json" \
        "${GITHUB_API}/repos/${REPO}/releases/latest") || error "Failed to fetch release information"

    # Extract version using grep and sed (portable alternative to jq)
    VERSION=$(echo "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        error "Could not determine latest version. Please specify a version with --version"
    fi
fi

info "Installing BootScope $VERSION..."

# Construct download URLs
DOWNLOAD_URL="${GITHUB_URL}/${REPO}/releases/download/${VERSION}/${FILENAME}"
CHECKSUM_URL="${GITHUB_URL}/${REPO}/releases/download/${VERSION}/checksums.txt"

# Download binary with security options
echo "Downloading ${FILENAME}..."
if ! curl -sSL --proto '=https' --tlsv1.2 --max-time 300 --max-redirs 3 \
    -o "$FILENAME" "$DOWNLOAD_URL"; then
    error "Failed to download binary from $DOWNLOAD_URL"
fi

# Verify download
if [ ! -f "$FILENAME" ]; then
    error "Download failed - file not found"
fi

# Check file size (should be at least 1MB for a Go binary)
FILE_SIZE=$(stat -f%z "$FILENAME" 2>/dev/null || stat -c%s "$FILENAME" 2>/dev/null || echo 0)
if [ "$FILE_SIZE" -lt 1048576 ]; then
    error "Downloaded file is too small (${FILE_SIZE} bytes). This may indicate a failed download."
fi

# Verify checksum if enabled
if [ "$VERIFY_CHECKSUM" = "true" ]; then
    info "Verifying checksum..."

    # Download checksums file
    if curl -sSL --proto '=https' --tlsv1.2 --max-time 60 --max-redirs 3 \
        -o "checksums.txt" "$CHECKSUM_URL" 2>/dev/null; then

        # Extract expected checksum for our file
        EXPECTED_CHECKSUM=$(grep "${FILENAME}" checksums.txt | awk '{print $1}')

        if [ -z "$EXPECTED_CHECKSUM" ]; then
            error "Could not find checksum for ${FILENAME} in checksums.txt"
        fi

        # Calculate actual checksum
        if command -v sha256sum >/dev/null 2>&1; then
            ACTUAL_CHECKSUM=$(sha256sum "$FILENAME" | awk '{print $1}')
        elif command -v shasum >/dev/null 2>&1; then
            ACTUAL_CHECKSUM=$(shasum -a 256 "$FILENAME" | awk '{print $1}')
        else
            warn "No SHA256 tool found. Skipping checksum verification."
            warn "Install sha256sum or shasum for better security."
            ACTUAL_CHECKSUM=""
        fi

        if [ -n "$ACTUAL_CHECKSUM" ]; then
            if [ "$ACTUAL_CHECKSUM" = "$EXPECTED_CHECKSUM" ]; then
                info "Checksum verified successfully"
            else
                error "Checksum verification failed. Expected: $EXPECTED_CHECKSUM, Got: $ACTUAL_CHECKSUM"
            fi
        fi
    else
        warn "Could not download checksums file. Skipping verification."
        warn "To enable checksum verification, ensure checksums.txt is available in the release."
    fi
fi

# Make executable
chmod +x "$FILENAME"

# Verify it's actually a binary
if ! file "$FILENAME" | grep -q "executable\|binary"; then
    error "Downloaded file does not appear to be a valid executable"
fi

# Create install directory if it doesn't exist
if [ ! -d "$INSTALL_DIR" ]; then
    info "Creating installation directory: $INSTALL_DIR"
    if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
        error "Failed to create installation directory. You may need sudo permissions."
    fi
fi

# Install
info "Installing to ${INSTALL_DIR}/${INSTALL_NAME}..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$FILENAME" "${INSTALL_DIR}/${INSTALL_NAME}"
else
    echo "Need sudo permission to install to $INSTALL_DIR"
    if ! sudo mv "$FILENAME" "${INSTALL_DIR}/${INSTALL_NAME}"; then
        error "Failed to install binary"
    fi
fi

# Verify installation
if command -v kubectl-bootscope >/dev/null 2>&1; then
    info "✅ Installation successful!"
    echo ""
    kubectl-bootscope version
    echo ""
    info "To enable shell completion:"
    echo "  Bash:  kubectl bootscope completion bash | sudo tee /etc/bash_completion.d/kubectl-bootscope"
    echo "  Zsh:   echo 'source <(kubectl bootscope completion zsh)' >> ~/.zshrc"
    echo "  Fish:  kubectl bootscope completion fish > ~/.config/fish/completions/kubectl-bootscope.fish"
    echo ""
    info "Try: kubectl bootscope --help"
else
    warn "Installation completed but kubectl-bootscope not found in PATH"
    echo "You may need to:"
    echo "  1. Add $INSTALL_DIR to your PATH:"
    echo "     export PATH=\"\$PATH:$INSTALL_DIR\""
    echo "  2. Restart your terminal"
    echo ""
    echo "To verify installation, run:"
    echo "  ${INSTALL_DIR}/${INSTALL_NAME} version"
fi

# Cleanup is handled by trap
