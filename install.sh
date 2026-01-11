#!/bin/bash

# vibeauracle Universal Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/nathfavour/vibeauracle/release/install.sh | sh

set -e

REPO="nathfavour/vibeauracle"
GITHUB_URL="https://github.com/$REPO"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if [ "$OS" = "darwin" ]; then
    OS="darwin"
elif [ "$OS" = "linux" ]; then
    # Check for Android (Termux)
    if [ -n "$TERMUX_VERSION" ]; then
        OS="android"
    else
        OS="linux"
    fi
else
    echo "Unsupported OS: $OS"
    exit 1
fi

BINARY_NAME="vibeaura-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
    BINARY_NAME+=".exe"
fi

echo "Detected Platform: $OS/$ARCH"

# Get latest release tag from the standard GitHub "latest" release endpoint
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    # Fallback: get the very first release if "latest" isn't explicitly set (e.g., only prereleases exist)
    LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/')
fi

if [ -z "$LATEST_TAG" ]; then

DOWNLOAD_URL="$GITHUB_URL/releases/download/$LATEST_TAG/$BINARY_NAME"

echo "Downloading $BINARY_NAME ($LATEST_TAG)..."
if command -v curl >/dev/null 2>&1; then
    curl -L "$DOWNLOAD_URL" -o vibeaura
elif command -v wget >/dev/null 2>&1; then
    wget -qO vibeaura "$DOWNLOAD_URL"
else
    echo "Error: curl or wget is required."
    exit 1
fi

chmod +x vibeaura

# Install binary
if [ "$OS" = "android" ]; then
    INSTALL_DIR="$HOME/bin"
    mkdir -p "$INSTALL_DIR"
    mv vibeaura "$INSTALL_DIR/vibeaura"
    chmod +x "$INSTALL_DIR/vibeaura"
    echo "Successfully installed vibeauracle to $INSTALL_DIR/vibeaura"

    # Auto-add to PATH
    SHELL_RC="$HOME/.bashrc"
    if [ -n "$ZSH_VERSION" ]; then
        SHELL_RC="$HOME/.zshrc"
    elif [ -n "$BASH_VERSION" ]; then
        SHELL_RC="$HOME/.bashrc"
    elif [ -f "$HOME/.zshrc" ]; then
        SHELL_RC="$HOME/.zshrc"
    fi

    if ! grep -q "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
        echo "" >> "$SHELL_RC"
        echo "# vibeauracle path" >> "$SHELL_RC"
        echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$SHELL_RC"
        echo "Added $INSTALL_DIR to $SHELL_RC"
    fi
    
    # Make it available immediately for this script and mention sourcing
    export PATH="$PATH:$INSTALL_DIR"
    echo "Please restart your shell or run: source $SHELL_RC"
    vibeaura --help
else
    INSTALL_DIR="/usr/local/bin"
    if [ -w "$INSTALL_DIR" ]; then
        mv vibeaura "$INSTALL_DIR/vibeaura"
    else
        echo "Requesting sudo to install to $INSTALL_DIR..."
        sudo mv vibeaura "$INSTALL_DIR/vibeaura"
    fi
    echo "Successfully installed vibeauracle to $INSTALL_DIR/vibeaura"
    vibeaura --help
fi
