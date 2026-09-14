#!/usr/bin/env sh
set -e

REPO="rimraf-adi/echo"
BIN_NAME="echo"

# 1. Choose installation target
# If /usr/local/bin is writable, use it directly (already in standard PATH on all Unix/macOS)
if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
    NEED_PATH_CONFIG=0
else
    INSTALL_DIR="${HOME}/.local/bin"
    NEED_PATH_CONFIG=1
fi

# 2. Detect OS
OS="$(uname -s)"
case "$OS" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux" ;;
    *)
        echo "Error: Unsupported operating system: $OS" >&2
        exit 1
        ;;
esac

# 3. Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)   ARCH="amd64" ;;
    arm64|aarch64)  ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# 4. Get latest release version tag
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

# 5. Configure PATH if installed to user directory
if [ "$NEED_PATH_CONFIG" -eq 1 ]; then
    add_to_file() {
        TARGET_FILE="$1"
        LINE_TO_ADD="export PATH=\"${INSTALL_DIR}:\$PATH\""
        
        # Touch file if not present
        mkdir -p "$(dirname "$TARGET_FILE")" 2>/dev/null || true
        touch "$TARGET_FILE" 2>/dev/null || true
        
        if [ -f "$TARGET_FILE" ] && [ -w "$TARGET_FILE" ]; then
            if ! grep -q "$INSTALL_DIR" "$TARGET_FILE"; then
                echo "" >> "$TARGET_FILE"
                echo "# Echo VCS binary path" >> "$TARGET_FILE"
                echo "$LINE_TO_ADD" >> "$TARGET_FILE"
                echo "Added ${INSTALL_DIR} to ${TARGET_FILE}"
            fi
        fi
    }

    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            # Configure standard shell profiles
            add_to_file "$HOME/.zshrc"
            add_to_file "$HOME/.bashrc"
            add_to_file "$HOME/.bash_profile"
            add_to_file "$HOME/.profile"

            # Support fish shell if directory exists
            if [ -d "$HOME/.config/fish" ]; then
                FISH_CONF="$HOME/.config/fish/config.fish"
                FISH_LINE="set -gx PATH \$PATH $INSTALL_DIR"
                touch "$FISH_CONF" 2>/dev/null || true
                if [ -f "$FISH_CONF" ] && ! grep -q "$INSTALL_DIR" "$FISH_CONF"; then
                    echo "" >> "$FISH_CONF"
                    echo "$FISH_LINE" >> "$FISH_CONF"
                    echo "Added ${INSTALL_DIR} to ${FISH_CONF}"
                fi
            fi

            export PATH="$INSTALL_DIR:$PATH"
            ;;
    esac
fi

echo ""
echo "Echo installation complete!"
echo "Run 'echo --help' to get started."
