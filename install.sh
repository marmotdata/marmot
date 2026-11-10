#!/bin/sh
# Marmot Installer

set -e

GITHUB_REPO="marmotdata/marmot"
BINARY_NAME="marmot"
INSTALL_DIR="${MARMOT_INSTALL_DIR:-/usr/local/bin}"

fetch() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$1" -o "$2"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "$2" "$1"
    else
        echo "Neither curl nor wget found. Please install one of them and try again."
        exit 1
    fi
}

# Set MARMOT_VERSION to pin a release; otherwise the latest stable release is used
VERSION="${MARMOT_VERSION#v}"
if [ -z "$VERSION" ]; then
    echo "Fetching the latest Marmot release..."
    if command -v curl >/dev/null 2>&1; then
        LATEST=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest")
    elif command -v wget >/dev/null 2>&1; then
        LATEST=$(wget -q -O- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest")
    fi
    VERSION=$(printf '%s\n' "$LATEST" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/; s/^v//')
fi

if [ -z "$VERSION" ]; then
    echo "Could not determine the latest version. Set MARMOT_VERSION and try again."
    exit 1
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

echo "Downloading Marmot ${VERSION} for ${OS}/${ARCH}..."

if [ "$OS" = "windows" ]; then
    FILENAME="marmot_${VERSION}_${OS}_${ARCH}.zip"
    BINARY_SUFFIX=".exe"
else
    FILENAME="marmot_${VERSION}_${OS}_${ARCH}.tar.gz"
    BINARY_SUFFIX=""
fi

RELEASE_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}"
CHECKSUMS="marmot_${VERSION}_checksums.txt"

fetch "${RELEASE_URL}/${FILENAME}" "${TEMP_DIR}/${FILENAME}"
fetch "${RELEASE_URL}/${CHECKSUMS}" "${TEMP_DIR}/${CHECKSUMS}"

# The checksum file is signed keylessly by the release workflow, so when cosign
# is installed check that signature before trusting the checksums in it
if command -v cosign >/dev/null 2>&1; then
    echo "Verifying the checksum file signature with cosign..."
    fetch "${RELEASE_URL}/${CHECKSUMS}.sigstore.json" "${TEMP_DIR}/${CHECKSUMS}.sigstore.json"
    cosign verify-blob \
        --bundle "${TEMP_DIR}/${CHECKSUMS}.sigstore.json" \
        --certificate-identity "https://github.com/${GITHUB_REPO}/.github/workflows/release.yaml@refs/tags/v${VERSION}" \
        --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
        "${TEMP_DIR}/${CHECKSUMS}" >/dev/null 2>&1 || {
        echo "Signature verification of ${CHECKSUMS} failed. Aborting."
        exit 1
    }
else
    echo "cosign not found, skipping signature verification of ${CHECKSUMS}."
fi

echo "Verifying the SHA-256 checksum of ${FILENAME}..."
EXPECTED=$(awk -v f="$FILENAME" '$2 == f || $2 == "*"f { print $1 }' "${TEMP_DIR}/${CHECKSUMS}")
if [ -z "$EXPECTED" ]; then
    echo "${FILENAME} is not listed in ${CHECKSUMS}. Aborting."
    exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL=$(sha256sum "${TEMP_DIR}/${FILENAME}" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL=$(shasum -a 256 "${TEMP_DIR}/${FILENAME}" | awk '{ print $1 }')
else
    echo "Neither sha256sum nor shasum found, cannot verify the download. Aborting."
    exit 1
fi
if [ "$EXPECTED" != "$ACTUAL" ]; then
    echo "Checksum mismatch for ${FILENAME}: expected ${EXPECTED}, got ${ACTUAL}. Aborting."
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

# docker, crane and oras run docker-credential-marmot to fetch the token
# from `marmot login`. The helper is the marmot binary under that name.
if [ "$OS" != "windows" ]; then
    HELPER_NAME="docker-credential-marmot"
    if [ "$USE_SUDO" = true ]; then
        sudo ln -sf "${BINARY_NAME}" "${INSTALL_DIR}/${HELPER_NAME}"
    else
        ln -sf "${BINARY_NAME}" "${INSTALL_DIR}/${HELPER_NAME}"
    fi
    echo "Linked ${INSTALL_DIR}/${HELPER_NAME} for registry pushes"
fi

echo "Marmot ${VERSION} has been installed successfully to ${INSTALL_DIR}/${BINARY_NAME}"
echo "Run 'marmot --help' to get started"
