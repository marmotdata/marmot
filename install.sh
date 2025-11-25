#!/bin/sh
# Marmot Installer

set -e

GITHUB_REPO="marmotdata/marmot"
BINARY_NAME="marmot"
INSTALL_DIR="/usr/local/bin"

# Fetch the latest non-prerelease version
echo "Fetching the latest Marmot release..."
if command -v curl >/dev/null 2>&1; then
    VERSION=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases" | 
              grep '"tag_name":' | 
              grep -v 'preview\|alpha\|beta\|rc' | 
              sed -E 's/.*"([^"]+)".*/\1/' | 
              head -n 1 | 
              sed 's/^v//')
elif command -v wget >/dev/null 2>&1; then
    VERSION=$(wget -q -O- "https://api.github.com/repos/${GITHUB_REPO}/releases" | 
              grep '"tag_name":' | 
              grep -v 'preview\|alpha\|beta\|rc' | 
              sed -E 's/.*"([^"]+)".*/\1/' | 
              head -n 1 | 
              sed 's/^v//')
else
    echo "Neither curl nor wget found. Please install one of them and try again."
    exit 1
fi

if [ -z "$VERSION" ]; then
    echo "Could not determine the latest version. Defaulting to 0.1.0"
    VERSION="0.4.0"
fi

echo "Installing Marmot version ${VERSION}..."

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    darwin) ;;
    linux) ;;
    msys*|mingw*|cygwin*) OS="windows" ;;
    *)
        echo "Unsupported operating system: $OS"
        exit 1
        ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

TEMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'marmot')
cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

if [ -z "$VERSION" ]; then
    echo "Error: Unable to determine Marmot version to install"
    exit 1
fi

echo "Downloading Marmot ${VERSION} for ${OS}/${ARCH}..."

if [ "$OS" = "windows" ]; then
    FILENAME="marmot_${VERSION}_${OS}_${ARCH}.zip"
    BINARY_SUFFIX=".exe"
else
    FILENAME="marmot_${VERSION}_${OS}_${ARCH}.tar.gz"
    BINARY_SUFFIX=""
fi

DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${FILENAME}"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o "${TEMP_DIR}/${FILENAME}"
elif command -v wget >/dev/null 2>&1; then
    wget -q -O "${TEMP_DIR}/${FILENAME}" "$DOWNLOAD_URL"
else
    echo "Neither curl nor wget found. Please install one of them and try again."
    exit 1
fi

cd "$TEMP_DIR"
if [ "$OS" = "windows" ]; then
    if command -v unzip >/dev/null 2>&1; then
        unzip -q "${FILENAME}"
    else
        echo "unzip command not found. Please install unzip and try again."
        exit 1
    fi
else
    tar -xzf "${FILENAME}"
fi

echo "Installing Marmot to ${INSTALL_DIR}..."

mkdir -p "$INSTALL_DIR"

if [ ! -w "$INSTALL_DIR" ]; then
    USE_SUDO=true
    if ! command -v sudo >/dev/null 2>&1; then
        echo "Installation directory is not writable and sudo is not available."
        echo "Please run this script as root or install to a different location."
        exit 1
    fi
fi

if [ "$USE_SUDO" = true ]; then
    sudo mv "$TEMP_DIR/${BINARY_NAME}${BINARY_SUFFIX}" "${INSTALL_DIR}/${BINARY_NAME}${BINARY_SUFFIX}"
    sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}${BINARY_SUFFIX}"
else
    mv "$TEMP_DIR/${BINARY_NAME}${BINARY_SUFFIX}" "${INSTALL_DIR}/${BINARY_NAME}${BINARY_SUFFIX}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}${BINARY_SUFFIX}"
fi

echo "Marmot ${VERSION} has been installed successfully to ${INSTALL_DIR}/${BINARY_NAME}"
echo "Run 'marmot --help' to get started"
