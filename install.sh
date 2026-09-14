#!/usr/bin/env sh
set -e

REPO="rimraf-adi/echo"
BIN_NAME="echo"
INSTALL_DIR="${HOME}/.local/bin"

# 1. Detect OS
OS="$(uname -s)"
case "$OS" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux" ;;
    *)
        echo "Error: Unsupported operating system: $OS" >&2
        exit 1
        ;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)   ARCH="amd64" ;;
    arm64|aarch64)  ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# 3. Get latest release version tag
echo "Detecting latest release for ${REPO}..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    echo "Notice: No GitHub release tag found yet, falling back to build via Go..."
    if command -v go >/dev/null 2>&1; then
        echo "Go is installed. Installing via go install..."
        go install github.com/rimraf-adi/echo/cmd/echo@latest
        echo "Installed successfully!"
        exit 0
    else
        echo "Error: Could not retrieve latest release from GitHub." >&2
        exit 1
    fi
fi

ARCHIVE_NAME="echo-${LATEST_TAG}-${OS}-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${ARCHIVE_NAME}"

echo "Downloading Echo ${LATEST_TAG} for ${OS}/${ARCH}..."
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"

mkdir -p "$INSTALL_DIR"
mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
chmod +x "${INSTALL_DIR}/${BIN_NAME}"

echo "Installed ${BIN_NAME} to ${INSTALL_DIR}/${BIN_NAME}"

# 4. Check PATH and configure shell profile if needed
add_to_path() {
    PROFILE_FILE="$1"
    LINE_TO_ADD='export PATH="$HOME/.local/bin:$PATH"'
    if [ -f "$PROFILE_FILE" ]; then
        if ! grep -q "$LINE_TO_ADD" "$PROFILE_FILE"; then
            echo "" >> "$PROFILE_FILE"
            echo "# Echo VCS" >> "$PROFILE_FILE"
            echo "$LINE_TO_ADD" >> "$PROFILE_FILE"
            echo "Added ${INSTALL_DIR} to ${PROFILE_FILE}"
        fi
    fi
}

case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
        add_to_path "$HOME/.zshrc"
        add_to_path "$HOME/.bashrc"
        add_to_path "$HOME/.profile"
        export PATH="$INSTALL_DIR:$PATH"
        ;;
esac

echo ""
echo "Echo installation complete!"
echo "Run 'echo --help' to get started."
