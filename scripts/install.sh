#!/usr/bin/env bash
# pvm — Polyglot Version Manager · one-line installer (macOS / Linux)
#
# Usage:
#   curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/Lycorxy/pvm/master/scripts/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- --version v0.0.2 --source github
#
# Environment overrides:
#   PVM_HOME                  install root (default: $HOME/.pvm)
#   PVM_INSTALL_VERSION       release tag to install (default: latest)
#   PVM_INSTALL_SOURCE        auto, gitee, or github (default: auto)
#   PVM_GITEE_REPO            Gitee repository (default: lycorxy/pvm)
#   PVM_GITHUB_REPO           GitHub repository (default: Lycorxy/pvm)
#   PVM_REPO                  compatibility override for both repositories
#   PVM_REQUIRE_CHECKSUM=1    fail if checksums.txt is unavailable
#   PVM_NO_MODIFY_PROFILE=1   do not modify the shell profile

if [ -z "${BASH_VERSION:-}" ]; then
  printf 'error: this installer must be run with bash\n' >&2
  exit 1
fi

set -euo pipefail
umask 022

msg()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

[ -n "${HOME:-}" ] || die 'HOME is not set'

PVM_HOME="${PVM_HOME:-$HOME/.pvm}"
PVM_INSTALL_SOURCE="${PVM_INSTALL_SOURCE:-auto}"
PVM_GITEE_REPO="${PVM_GITEE_REPO:-${PVM_REPO:-lycorxy/pvm}}"
PVM_GITHUB_REPO="${PVM_GITHUB_REPO:-${PVM_REPO:-Lycorxy/pvm}}"
PVM_REQUIRE_CHECKSUM="${PVM_REQUIRE_CHECKSUM:-0}"
PVM_NO_MODIFY_PROFILE="${PVM_NO_MODIFY_PROFILE:-0}"
VERSION="${PVM_INSTALL_VERSION:-}"

while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      [ $# -ge 2 ] || die '--version requires a value'
      VERSION="$2"
      shift 2
      ;;
    --source)
      [ $# -ge 2 ] || die '--source requires auto, gitee, or github'
      PVM_INSTALL_SOURCE="$2"
      shift 2
      ;;
    --no-modify-profile)
      PVM_NO_MODIFY_PROFILE=1
      shift
      ;;
    -h|--help)
      cat <<'EOF'
Usage: install.sh [options]

Options:
  --version vX.Y.Z        Install a specific release (default: latest)
  --source SOURCE         Use auto, gitee, or github (default: auto)
  --no-modify-profile     Do not update the shell profile
  -h, --help              Show this help
EOF
      exit 0
      ;;
    *) die "unknown argument: $1" ;;
  esac
done

case "$PVM_INSTALL_SOURCE" in
  auto|gitee|github) ;;
  *) die "unsupported source: $PVM_INSTALL_SOURCE (use auto, gitee, or github)" ;;
esac
case "$PVM_REQUIRE_CHECKSUM" in
  0|1) ;;
  *) die 'PVM_REQUIRE_CHECKSUM must be 0 or 1' ;;
esac
case "$PVM_NO_MODIFY_PROFILE" in
  0|1) ;;
  *) die 'PVM_NO_MODIFY_PROFILE must be 0 or 1' ;;
esac

if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
  die 'curl or wget is required'
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin) OS=darwin ;;
  linux) OS=linux ;;
  *) die "unsupported OS: $OS (use install.ps1 on Windows)" ;;
esac

MACHINE="$(uname -m)"
if [ "$OS" = darwin ] && [ "$MACHINE" = x86_64 ] && command -v sysctl >/dev/null 2>&1; then
  if [ "$(sysctl -in sysctl.proc_translated 2>/dev/null || true)" = 1 ]; then
    MACHINE=arm64
  fi
fi
case "$MACHINE" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported architecture: $MACHINE" ;;
esac

export PVM_HOME
mkdir -p "$PVM_HOME/bin" "$PVM_HOME/shims" "$PVM_HOME/installs" "$PVM_HOME/cache" "$PVM_HOME/tmp"

WORK_DIR=""
STAGED_BINARY=""
cleanup() {
  [ -z "${STAGED_BINARY:-}" ] || rm -f "$STAGED_BINARY"
  [ -z "${WORK_DIR:-}" ] || rm -rf "$WORK_DIR"
}
trap cleanup EXIT
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pvm-install.XXXXXX")"
STAGED_BINARY="$(mktemp "$PVM_HOME/bin/.pvm-install.XXXXXX")"

# Download to a temporary path. curl is preferred; wget is the fallback.
download_file() {
  local url="$1"
  local output="$2"

  rm -f "$output"
  if command -v curl >/dev/null 2>&1; then
    if curl --fail --silent --show-error --location \
      --retry 2 --connect-timeout 15 --max-time 300 \
      --speed-limit 1024 --speed-time 30 --user-agent 'pvm-installer' \
      --output "$output" "$url"; then
      return 0
    fi
    rm -f "$output"
  fi

  if command -v wget >/dev/null 2>&1; then
    if wget --quiet --tries=3 --timeout=30 \
      --user-agent='pvm-installer' --output-document="$output" "$url"; then
      return 0
    fi
    rm -f "$output"
  fi

  return 1
}

repo_for_source() {
  case "$1" in
    gitee) printf '%s\n' "$PVM_GITEE_REPO" ;;
    github) printf '%s\n' "$PVM_GITHUB_REPO" ;;
  esac
}

api_url_for_source() {
  local source="$1"
  local repo
  repo="$(repo_for_source "$source")"
  case "$source" in
    gitee) printf 'https://gitee.com/api/v5/repos/%s/releases/latest\n' "$repo" ;;
    github) printf 'https://api.github.com/repos/%s/releases/latest\n' "$repo" ;;
  esac
}

asset_url_for_source() {
  local source="$1"
  local release="$2"
  local asset="$3"
  local repo
  repo="$(repo_for_source "$source")"
  if [ "$release" = latest ]; then
    case "$source" in
      gitee) printf 'https://gitee.com/%s/releases/latest/download/%s\n' "$repo" "$asset" ;;
      github) printf 'https://github.com/%s/releases/latest/download/%s\n' "$repo" "$asset" ;;
    esac
  else
    case "$source" in
      gitee) printf 'https://gitee.com/%s/releases/download/%s/%s\n' "$repo" "$release" "$asset" ;;
      github) printf 'https://github.com/%s/releases/download/%s/%s\n' "$repo" "$release" "$asset" ;;
    esac
  fi
}

resolve_latest() {
  local source="$1"
  local response="$WORK_DIR/release-$source.json"
  local tag

  download_file "$(api_url_for_source "$source")" "$response" || return 1
  tag="$(LC_ALL=C sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$response" | head -n 1)"
  [ -n "$tag" ] || return 1
  printf '%s\n' "$tag"
}

normalize_version() {
  case "$1" in
    v*|latest) printf '%s\n' "$1" ;;
    [0-9]*) printf 'v%s\n' "$1" ;;
    *) printf '%s\n' "$1" ;;
  esac
}

calculate_sha256() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
  else
    return 1
  fi
}

verify_checksum() {
  local source="$1"
  local release="$2"
  local asset="$3"
  local binary="$4"
  local checksum_file="$WORK_DIR/checksums-$source.txt"
  local checksum_url expected actual

  checksum_url="$(asset_url_for_source "$source" "$release" checksums.txt)"
  if ! download_file "$checksum_url" "$checksum_file" >/dev/null 2>&1; then
    if [ "$PVM_REQUIRE_CHECKSUM" = 1 ]; then
      warn "checksums.txt is unavailable from $source"
      return 1
    fi
    warn "checksums.txt is unavailable; continuing with executable validation"
    return 0
  fi

  expected="$(LC_ALL=C awk -v asset="$asset" '
    {
      file = $2
      sub(/^\*/, "", file)
      sub(/\r$/, "", file)
      if (file == asset) {
        print $1
        exit
      }
    }
  ' "$checksum_file" | LC_ALL=C tr -cd '0-9A-Fa-f')"

  if [ "${#expected}" -ne 64 ]; then
    if [ "$PVM_REQUIRE_CHECKSUM" = 1 ]; then
      warn "no checksum entry found for $asset"
      return 1
    fi
    warn "no checksum entry found for $asset; continuing with executable validation"
    return 0
  fi

  if ! actual="$(calculate_sha256 "$binary")"; then
    if [ "$PVM_REQUIRE_CHECKSUM" = 1 ]; then
      warn 'no SHA-256 utility is available'
      return 1
    fi
    warn 'no SHA-256 utility is available; continuing with executable validation'
    return 0
  fi

  expected="$(printf '%s' "$expected" | tr '[:upper:]' '[:lower:]')"
  actual="$(printf '%s' "$actual" | tr '[:upper:]' '[:lower:]')"
  [ "$actual" = "$expected" ] || die "checksum mismatch for $asset"
  msg "verified SHA-256 for $asset"
}

ASSET="pvm-$OS-$ARCH"
TARGET="$PVM_HOME/bin/pvm"
REQUESTED_VERSION=""
if [ -n "$VERSION" ]; then
  REQUESTED_VERSION="$(normalize_version "$VERSION")"
fi

case "$PVM_INSTALL_SOURCE" in
  auto) SOURCE_ORDER=(gitee github) ;;
  gitee) SOURCE_ORDER=(gitee) ;;
  github) SOURCE_ORDER=(github) ;;
esac

SELECTED_SOURCE=""
DOWNLOAD_URL=""
BINARY_VERSION=""

for candidate_source in "${SOURCE_ORDER[@]}"; do
  candidate_release="$REQUESTED_VERSION"
  if [ -z "$candidate_release" ]; then
    msg "resolving latest release from $candidate_source ..."
    if ! candidate_release="$(resolve_latest "$candidate_source")"; then
      warn "could not query the latest release from $candidate_source; trying its latest download URL"
      candidate_release=latest
    fi
  fi

  candidate_url="$(asset_url_for_source "$candidate_source" "$candidate_release" "$ASSET")"
  msg "downloading $ASSET from $candidate_source"
  if ! download_file "$candidate_url" "$STAGED_BINARY"; then
    warn "download failed from $candidate_source"
    continue
  fi

  if ! verify_checksum "$candidate_source" "$candidate_release" "$ASSET" "$STAGED_BINARY"; then
    warn "checksum verification is required but unavailable from $candidate_source"
    continue
  fi

  chmod 0755 "$STAGED_BINARY"
  if ! candidate_output="$("$STAGED_BINARY" version 2>&1)"; then
    warn "the downloaded file from $candidate_source is not executable on $OS/$ARCH"
    continue
  fi

  candidate_binary_version="$(printf '%s\n' "$candidate_output" | awk '$1 == "pvm" {print $2; exit}')"
  candidate_binary_version="${candidate_binary_version#v}"
  if [ -z "$candidate_binary_version" ]; then
    warn "the downloaded file from $candidate_source did not report a valid pvm version"
    continue
  fi

  if [ "$candidate_release" != latest ]; then
    expected_version="${candidate_release#v}"
    if [ "$candidate_binary_version" != "$expected_version" ]; then
      warn "release $candidate_release contains pvm $candidate_binary_version; trying another source"
      continue
    fi
  fi

  SELECTED_SOURCE="$candidate_source"
  DOWNLOAD_URL="$candidate_url"
  BINARY_VERSION="$candidate_binary_version"
  break
done

[ -n "$SELECTED_SOURCE" ] || die "could not download a valid $ASSET release"
[ ! -d "$TARGET" ] || die "installation target is a directory: $TARGET"

msg "installing pvm v$BINARY_VERSION ($OS/$ARCH) to $PVM_HOME"
if ! mv -f "$STAGED_BINARY" "$TARGET"; then
  die "could not install pvm to $TARGET"
fi
STAGED_BINARY=""
chmod 0755 "$TARGET"

if [ "$PVM_NO_MODIFY_PROFILE" = 1 ]; then
  msg 'initializing shims without modifying the shell profile'
  if ! "$TARGET" reshim; then
    warn "reshim reported warnings; run '$TARGET reshim' after configuring PATH"
  fi
else
  msg 'configuring pvm and the shell profile'
  if ! "$TARGET" setup; then
    die "pvm was installed to $TARGET, but setup failed; run '$TARGET setup' to retry"
  fi
fi

if ! INSTALLED_VERSION="$("$TARGET" version 2>&1)"; then
  die "pvm was installed but failed its final validation: $TARGET"
fi

echo
msg "$INSTALLED_VERSION installed successfully"
echo "  Source: $SELECTED_SOURCE"
echo "  URL:    $DOWNLOAD_URL"
echo "  Path:   $TARGET"
echo

shell_name="$(basename "${SHELL:-bash}")"
if [ "$PVM_NO_MODIFY_PROFILE" = 1 ]; then
  echo 'Add these lines to your shell profile:'
  echo "  export PVM_HOME=\"$PVM_HOME\""
  echo '  export PATH="$PVM_HOME/shims:$PVM_HOME/bin:$PATH"'
else
  echo 'Restart your terminal, or reload the current shell profile:'
  case "$shell_name" in
    zsh) echo '  source ~/.zshrc' ;;
    bash) echo '  source ~/.bashrc' ;;
    fish) echo '  source ~/.config/fish/config.fish' ;;
    *) echo "  restart $shell_name" ;;
  esac
fi

echo
echo 'Verify the installation:'
echo '  pvm version'
echo '  pvm doctor'
echo
