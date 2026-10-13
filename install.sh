#!/bin/bash
# GitForge Installer
# Installs GitForge on Linux, macOS, Windows (WSL), and Termux

set -e

REPO="${REPO:-${GITHUB_REPOSITORY:-ItzPhantomm/gitforge}}"
BINARY_NAME="gitforge"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
success() { echo -e "${GREEN}✅ $1${NC}"; }
warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $OS in
        linux)
            if grep -qi microsoft /proc/version 2>/dev/null; then
                OS="windows"
            elif grep -qi termux /proc/version 2>/dev/null || [ -n "$TERMUX_VERSION" ]; then
                OS="termux"
                INSTALL_DIR="$HOME/.local/bin"
            fi
            ;;
        darwin) OS="darwin" ;;
        *) error "Unsupported OS: $OS" ;;
    esac

    case $ARCH in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        armv7l) ARCH="armv7" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Get latest release version
get_latest_version() {
    info "Fetching latest release..."
    local auth_header=()
    if [ -n "$GITHUB_TOKEN" ]; then
        auth_header=(-H "Authorization: token $GITHUB_TOKEN")
    elif [ -n "$GH_TOKEN" ]; then
        auth_header=(-H "Authorization: token $GH_TOKEN")
    fi

    VERSION=$(curl -sL "${auth_header[@]}" "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        warning "Could not fetch latest version from API, using 'latest'"
        VERSION="latest"
    else
        info "Latest version: $VERSION"
    fi
}

# Download and install
install_binary() {
    local auth_header=()
    if [ -n "$GITHUB_TOKEN" ]; then
        auth_header=(-H "Authorization: token $GITHUB_TOKEN")
    elif [ -n "$GH_TOKEN" ]; then
        auth_header=(-H "Authorization: token $GH_TOKEN")
    fi

    local url="https://github.com/$REPO/releases/download/$VERSION/${BINARY_NAME}_${PLATFORM}.tar.gz"
    
    if [ "$VERSION" = "latest" ]; then
        url="https://github.com/$REPO/releases/latest/download/${BINARY_NAME}_${PLATFORM}.tar.gz"
    fi

    info "Downloading from $url..."
    
    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    if ! curl -fL "${auth_header[@]}" -o "$TMP_DIR/${BINARY_NAME}.tar.gz" "$url"; then
        error "Failed to download $url. Please check if release exists for $PLATFORM"
    fi

    info "Extracting..."
    tar -xzf "$TMP_DIR/${BINARY_NAME}.tar.gz" -C "$TMP_DIR"

    # Find the extracted binary (archive may contain gitforge, gitforge_<platform>, or gitforge_<platform>.exe)
    local EXTRACTED_BIN=""
    if [ -f "$TMP_DIR/$BINARY_NAME" ]; then
        EXTRACTED_BIN="$TMP_DIR/$BINARY_NAME"
    elif [ -f "$TMP_DIR/${BINARY_NAME}_${PLATFORM}" ]; then
        EXTRACTED_BIN="$TMP_DIR/${BINARY_NAME}_${PLATFORM}"
    elif [ -f "$TMP_DIR/${BINARY_NAME}_${PLATFORM}.exe" ]; then
        EXTRACTED_BIN="$TMP_DIR/${BINARY_NAME}_${PLATFORM}.exe"
    else
        EXTRACTED_BIN=$(find "$TMP_DIR" -type f -name "${BINARY_NAME}*" ! -name "*.tar.gz" | head -1)
    fi

    if [ -z "$EXTRACTED_BIN" ] || [ ! -f "$EXTRACTED_BIN" ]; then
        error "Could not find binary in extracted archive"
    fi

    info "Installing to $INSTALL_DIR..."
    local SUDO_CMD=""
    if [ "$OS" != "termux" ] && [ "$(id -u)" -ne 0 ]; then
        if command -v sudo >/dev/null 2>&1; then
            SUDO_CMD="sudo"
        else
            error "Root privileges or sudo required to install to $INSTALL_DIR"
        fi
    fi

    if [ "$OS" = "termux" ]; then
        mkdir -p "$INSTALL_DIR"
        mv "$EXTRACTED_BIN" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        $SUDO_CMD mkdir -p "$INSTALL_DIR"
        $SUDO_CMD mv "$EXTRACTED_BIN" "$INSTALL_DIR/$BINARY_NAME"
        $SUDO_CMD chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi

    success "Installed $BINARY_NAME to $INSTALL_DIR"
}

# Verify installation
verify_install() {
    if command -v $BINARY_NAME >/dev/null 2>&1; then
        local version=$($BINARY_NAME version 2>/dev/null | head -1 || echo "unknown")
        success "GitForge installed successfully!"
        info "Version: $version"
        info "Run 'gitforge --help' to get started"
    else
        warning "Binary installed but not in PATH. Add $INSTALL_DIR to your PATH."
        case $SHELL in
            */bash) echo "  echo 'export PATH=\$PATH:$INSTALL_DIR' >> ~/.bashrc" ;;
            */zsh) echo "  echo 'export PATH=\$PATH:$INSTALL_DIR' >> ~/.zshrc" ;;
            */fish) echo "  echo 'set -gx PATH \$PATH $INSTALL_DIR' >> ~/.config/fish/config.fish" ;;
        esac
    fi
}

# Install shell completions
install_completions() {
    info "Installing shell completions..."
    local SUDO_CMD=""
    if [ "$OS" != "termux" ] && [ "$(id -u)" -ne 0 ]; then
        command -v sudo >/dev/null 2>&1 && SUDO_CMD="sudo"
    fi

    $BINARY_NAME completion bash >/dev/null 2>&1 && {
        if [ "$OS" = "termux" ]; then
            mkdir -p "$HOME/.bash_completion.d"
            $BINARY_NAME completion bash > "$HOME/.bash_completion.d/gitforge" 2>/dev/null || true
            echo "source ~/.bash_completion.d/gitforge" >> ~/.bashrc 2>/dev/null || true
        else
            if [ -d "/etc/bash_completion.d" ]; then
                $SUDO_CMD $BINARY_NAME completion bash | $SUDO_CMD tee /etc/bash_completion.d/gitforge >/dev/null 2>&1 || true
            fi
        fi
    }
    $BINARY_NAME completion zsh >/dev/null 2>&1 && {
        if [ "$OS" = "termux" ]; then
            mkdir -p "$HOME/.zsh/completions"
            $BINARY_NAME completion zsh > "$HOME/.zsh/completions/_gitforge" 2>/dev/null || true
            echo 'fpath+=~/.zsh/completions' >> ~/.zshrc 2>/dev/null || true
        else
            if [ -d "/usr/local/share/zsh/site-functions" ]; then
                $SUDO_CMD $BINARY_NAME completion zsh | $SUDO_CMD tee /usr/local/share/zsh/site-functions/_gitforge >/dev/null 2>&1 || true
            fi
        fi
    }
    $BINARY_NAME completion fish >/dev/null 2>&1 && {
        if [ -d "$HOME/.config/fish/completions" ] || mkdir -p ~/.config/fish/completions 2>/dev/null; then
            $BINARY_NAME completion fish > ~/.config/fish/completions/gitforge.fish 2>/dev/null || true
        fi
    }
    success "Shell completions installed"
}

# Main
main() {
    echo -e "${BLUE}"
    echo "  ╔══════════════════════════════════════╗"
    echo "  ║        GitForge Installer           ║"
    echo "  ║   Beautiful Git for Terminal & Web  ║"
    echo "  ╚══════════════════════════════════════╝"
    echo -e "${NC}"

    detect_platform
    get_latest_version
    install_binary
    verify_install
    install_completions

    echo ""
    success "🎉 Installation complete!"
    echo ""
    echo "Quick start:"
    echo "  gitforge tui        # Launch terminal UI"
    echo "  gitforge web        # Start web UI"
    echo "  gitforge status     # Show repo status"
    echo "  gitforge log        # Show commit history"
    echo "  gitforge doctor     # Run diagnostics"
    echo ""
}

# Run installer
main "$@"