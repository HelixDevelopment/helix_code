#!/bin/sh
set -eu

REPO="helixcode/helixcode"
VERSION="${1:-latest}"
INSTALL_DIR="${HELIXCODE_DIR:-/usr/local/bin}"

# PARITY-GAP: Windows_NT — this POSIX curl|sh installer deliberately supports
#   only Linux + Darwin (binary tarball into /usr/local/bin). Windows is built
#   and distributed as a .zip package via the Makefile `windows` target and is
#   installed by a separate mechanism, not by this shell script — there is no
#   POSIX-shell install path on native Windows. See §11.4.81 / docs/platforms/.
detect_os() {
    case "$(uname -s)" in
        Linux)   echo "linux" ;;
        Darwin)  echo "darwin" ;;
        *)       echo "unsupported" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)            echo "unsupported" ;;
    esac
}

main() {
    os=$(detect_os)
    arch=$(detect_arch)

    if [ "$os" = "unsupported" ] || [ "$arch" = "unsupported" ]; then
        echo "Unsupported platform: $(uname -s) $(uname -m)" >&2
        echo "See https://helixcode.dev/docs/install for alternatives" >&2
        exit 1
    fi

    echo "HelixCode Installer"
    echo "Platform: $os/$arch"
    echo ""

    if [ "$VERSION" = "latest" ]; then
        download_url="https://github.com/$REPO/releases/latest/download/helixcode-$os-$arch.tar.gz"
    else
        download_url="https://github.com/$REPO/releases/download/v$VERSION/helixcode-$VERSION.$os-$arch.tar.gz"
    fi

    tmpdir=$(mktemp -d)
    echo "Downloading from $download_url ..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$download_url" -o "$tmpdir/helixcode.tar.gz"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$download_url" -O "$tmpdir/helixcode.tar.gz"
    else
        echo "Need curl or wget" >&2
        exit 1
    fi

    tar xzf "$tmpdir/helixcode.tar.gz" -C "$tmpdir"

    mkdir -p "$INSTALL_DIR"
    cp "$tmpdir/helixcode" "$INSTALL_DIR/helixcode"
    chmod +x "$INSTALL_DIR/helixcode"

    rm -rf "$tmpdir"

    echo ""
    echo "HelixCode installed to $INSTALL_DIR/helixcode"
    echo "Run 'helixcode version' to verify."
    echo ""
    echo "Quick start:"
    echo "  1. Copy config example: helixcode init-config"
    echo "  2. Edit config:         \$EDITOR ~/.config/helixcode/config.yaml"
    echo "  3. Start server:        helixcode server"

    # Create config directory with example
    mkdir -p ~/.config/helixcode
    if [ ! -f ~/.config/helixcode/config.yaml ]; then
        "$INSTALL_DIR/helixcode" init-config > ~/.config/helixcode/config.yaml 2>/dev/null || true
        echo "  (config created at ~/.config/helixcode/config.yaml)"
    fi
}

main "$@"
