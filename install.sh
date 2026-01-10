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

if [ "$OS" == "darwin" ]; then
    OS="darwin"
elif [ "$OS" == "linux" ]; then
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

BINARY_NAME="vibe-${OS}-${ARCH}"
if [ "$OS" == "windows" ]; then
    BINARY_NAME+=".exe"
fi

echo "Detected Platform: $OS/$ARCH"

# Get latest release tag
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    echo "Could not find latest release. Please check $GITHUB_URL/releases"
    exit 1
fi

DOWNLOAD_URL="$GITHUB_URL/releases/download/$LATEST_TAG/$BINARY_NAME"

echo "Downloading $BINARY_NAME ($LATEST_TAG)..."
if command -v curl >/dev/null 2>&1; then
    curl -L "$DOWNLOAD_URL" -o vibe
elif command -v wget >/dev/null 2>&1; then
    wget -qO vibe "$DOWNLOAD_URL"
else
    echo "Error: curl or wget is required."
    exit 1
fi

chmod +x vibe

# Install binary
INSTALL_DIR="/usr/local/bin"
if [ "$OS" == "android" ]; then
    INSTALL_DIR="$PREFIX/bin"
fi

if [ -w "$INSTALL_DIR" ]; then
    mv vibe "$INSTALL_DIR/vibe"
else
    echo "Requesting sudo to install to $INSTALL_DIR..."
    sudo mv vibe "$INSTALL_DIR/vibe"
fi

echo "Successfully installed vibeauracle to $INSTALL_DIR/vibe"
vibe --help

