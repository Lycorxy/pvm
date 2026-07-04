#!/usr/bin/env bash
# pvm — Polyglot Version Manager · one-liner installer (macOS / Linux)
#
# Usage:
#   curl -fsSL https://gitee.com/lucky-zsh/pvm/raw/main/scripts/install.sh | bash
#   curl -fsSL ... | bash -s -- --version v1.0.0
#
# Environment overrides:
#   PVM_HOME           custom install root (default: $HOME/.pvm)
#   PVM_REPO           Gitee repo in "owner/name" form (default: lucky-zsh/pvm)
#   PVM_INSTALL_VERSION  specific tag to install (default: latest)
#   PVM_NO_MODIFY_PROFILE=1  skip editing shell rc file

set -euo pipefail

PVM_REPO="${PVM_REPO:-lucky-zsh/pvm}"
PVM_HOME="${PVM_HOME:-$HOME/.pvm}"

# Parse args
VERSION="${PVM_INSTALL_VERSION:-}"
while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: install.sh [--version vX.Y.Z]"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

msg()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; exit 1; }

# --- detect OS / arch ---
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin) OS=darwin ;;
  linux)  OS=linux ;;
  *) die "unsupported OS: $OS (this installer supports macOS and Linux; use install.ps1 on Windows)" ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported arch: $ARCH" ;;
esac

# --- resolve version ---
if [ -z "$VERSION" ]; then
  msg "resolving latest release of $PVM_REPO ..."
  if command -v curl >/dev/null 2>&1; then
    VERSION="$(curl -fsSL "https://gitee.com/api/v3/repos/$PVM_REPO/releases/latest" \
      | grep -oE '"tag_name"\s*:\s*"[^"]+"' | head -n1 | cut -d\" -f4)"
  elif command -v wget >/dev/null 2>&1; then
    VERSION="$(wget -qO- "https://gitee.com/api/v3/repos/$PVM_REPO/releases/latest" \
      | grep -oE '"tag_name"\s*:\s*"[^"]+"' | head -n1 | cut -d\" -f4)"
  else
    die "need curl or wget to resolve latest version"
  fi
  [ -n "$VERSION" ] || die "could not resolve latest version from Gitee"
fi
msg "installing pvm $VERSION ($OS/$ARCH) to $PVM_HOME"

# --- prepare dirs ---
mkdir -p "$PVM_HOME/bin" "$PVM_HOME/shims" "$PVM_HOME/installs" "$PVM_HOME/cache"

# --- download binary ---
ASSET="pvm-$OS-$ARCH"
URL="https://gitee.com/$PVM_REPO/releases/download/$VERSION/$ASSET"
TARGET="$PVM_HOME/bin/pvm"

msg "downloading $URL"
if command -v curl >/dev/null 2>&1; then
  curl -fSL --progress-bar "$URL" -o "$TARGET" \
    || die "download failed: $URL"
else
  wget -q --show-progress -O "$TARGET" "$URL" \
    || die "download failed: $URL"
fi
chmod +x "$TARGET"

# --- initial reshim so shims dir has a baseline ---
"$TARGET" reshim >/dev/null 2>&1 || true

# --- update PATH in shell rc ---
add_to_rc() {
  local rc="$1"
  [ -f "$rc" ] || return 0
  if grep -q 'PVM_HOME' "$rc" 2>/dev/null; then
    msg "already configured: $rc"
    return 0
  fi
  msg "adding pvm to $rc"
  {
    echo ""
    echo "# pvm (Polyglot Version Manager)"
    echo "export PVM_HOME=\"$PVM_HOME\""
    echo "export PATH=\"\$PVM_HOME/shims:\$PVM_HOME/bin:\$PATH\""
  } >> "$rc"
}

if [ "${PVM_NO_MODIFY_PROFILE:-}" != "1" ]; then
  shell_name="$(basename "${SHELL:-}")"
  case "$shell_name" in
    bash) add_to_rc "$HOME/.bashrc"; add_to_rc "$HOME/.bash_profile" ;;
    zsh)  add_to_rc "$HOME/.zshrc" ;;
    fish)
      fish_conf="$HOME/.config/fish/config.fish"
      if [ -f "$fish_conf" ] && ! grep -q 'PVM_HOME' "$fish_conf"; then
        msg "adding pvm to $fish_conf"
        {
          echo ""
          echo "# pvm"
          echo "set -gx PVM_HOME \"$PVM_HOME\""
          echo "set -gx PATH \"\$PVM_HOME/shims\" \"\$PVM_HOME/bin\" \$PATH"
        } >> "$fish_conf"
      fi
      ;;
    *) warn "unknown shell '$shell_name' — add this to your rc file manually:
    export PVM_HOME=\"$PVM_HOME\"
    export PATH=\"\$PVM_HOME/shims:\$PVM_HOME/bin:\$PATH\"" ;;
  esac
fi

echo
msg "pvm $VERSION installed to $TARGET"
echo
echo "Next steps:"
echo "  1. Restart your shell (or: source ~/.${shell_name:-bashrc}rc)"
echo "  2. pvm install node@20.11.0"
echo "  3. cd into a project and run: pvm init"
echo
