#!/usr/bin/env bash
# Detect macOS/Linux and native CPU architecture, then run the canonical PVM installer.

if [ -z "${BASH_VERSION:-}" ]; then
  printf 'error: run this installer with bash\n' >&2
  exit 1
fi

set -euo pipefail

SOURCE="${PVM_INSTALL_SOURCE:-auto}"
VERSION="${PVM_INSTALL_VERSION:-}"
DETECT_ONLY=0

usage() {
  cat <<'EOF'
Usage: install-pvm.sh [--version vX.Y.Z] [--source auto|gitee|github] [--detect-only]
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || { printf 'error: --version requires a value\n' >&2; exit 2; }
      VERSION="$2"
      shift 2
      ;;
    --source)
      [ "$#" -ge 2 ] || { printf 'error: --source requires a value\n' >&2; exit 2; }
      SOURCE="$2"
      shift 2
      ;;
    --detect-only)
      DETECT_ONLY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'error: unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$SOURCE" in
  auto|gitee|github) ;;
  *) printf 'error: unsupported source: %s\n' "$SOURCE" >&2; exit 2 ;;
esac

OS_NAME="$(uname -s 2>/dev/null || true)"
case "$OS_NAME" in
  Darwin) PLATFORM=darwin ;;
  Linux) PLATFORM=linux ;;
  *) printf 'error: unsupported operating system: %s\n' "${OS_NAME:-unknown}" >&2; exit 1 ;;
esac

MACHINE="$(uname -m 2>/dev/null || true)"
if [ "$PLATFORM" = darwin ] && [ "$MACHINE" = x86_64 ] && command -v sysctl >/dev/null 2>&1; then
  if [ "$(sysctl -in sysctl.proc_translated 2>/dev/null || true)" = 1 ]; then
    MACHINE=arm64
  fi
fi
case "$MACHINE" in
  x86_64|amd64) ARCHITECTURE=amd64 ;;
  arm64|aarch64) ARCHITECTURE=arm64 ;;
  *) printf 'error: no PVM release is available for architecture: %s\n' "${MACHINE:-unknown}" >&2; exit 1 ;;
esac

ASSET="pvm-$PLATFORM-$ARCHITECTURE"
printf 'platform=%s architecture=%s asset=%s\n' "$PLATFORM" "$ARCHITECTURE" "$ASSET"
[ "$DETECT_ONLY" = 0 ] || exit 0

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
LOCAL_ROOT="$SCRIPT_DIR/../../../.."
LOCAL_INSTALLER="$LOCAL_ROOT/scripts/install.sh"
LOCAL_MODULE="$LOCAL_ROOT/go.mod"

run_installer() {
  local installer="$1"
  local installer_source="${2:-$SOURCE}"
  if [ -n "$VERSION" ]; then
    bash "$installer" --source "$installer_source" --version "$VERSION"
  else
    bash "$installer" --source "$installer_source"
  fi
}

if [ -f "$LOCAL_INSTALLER" ] && [ -f "$LOCAL_MODULE" ] && \
  grep -Eq '^module[[:space:]]+github\.com/pvm/pvm[[:space:]]*$' "$LOCAL_MODULE"; then
  run_installer "$LOCAL_INSTALLER"
  exit $?
fi

if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
  printf 'error: curl or wget is required\n' >&2
  exit 1
fi

TEMP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pvm-bootstrap.XXXXXX")"
cleanup() { rm -f "$TEMP_INSTALLER"; }
trap cleanup EXIT

download() {
  local url="$1"
  rm -f "$TEMP_INSTALLER"
  if command -v curl >/dev/null 2>&1 && curl --fail --silent --show-error --location \
    --retry 2 --connect-timeout 15 --max-time 120 \
    --output "$TEMP_INSTALLER" "$url"; then
    return 0
  fi
  rm -f "$TEMP_INSTALLER"
  if command -v wget >/dev/null 2>&1 && wget --quiet --tries=3 --timeout=30 \
    --output-document="$TEMP_INSTALLER" "$url"; then
    return 0
  fi
  return 1
}

case "$SOURCE" in
  auto)
    INSTALLER_URLS="https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh
https://raw.githubusercontent.com/Lycorxy/pvm/master/scripts/install.sh"
    ;;
  gitee) INSTALLER_URLS="https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh" ;;
  github) INSTALLER_URLS="https://raw.githubusercontent.com/Lycorxy/pvm/master/scripts/install.sh" ;;
esac

INSTALLED_FROM=""
while IFS= read -r url; do
  [ -n "$url" ] || continue
  printf 'downloading installer from %s\n' "$url"
  if ! download "$url" || [ ! -s "$TEMP_INSTALLER" ] || \
    ! grep -q 'PVM_INSTALL_SOURCE' "$TEMP_INSTALLER" || \
    ! grep -q 'pvm-installer' "$TEMP_INSTALLER" || \
    ! bash -n "$TEMP_INSTALLER"; then
    printf 'warning: invalid installer from %s; trying another source\n' "$url" >&2
    continue
  fi

  case "$url" in
    *gitee.com*) installer_source=gitee ;;
    *) installer_source=github ;;
  esac
  if run_installer "$TEMP_INSTALLER" "$installer_source"; then
    INSTALLED_FROM="$url"
    break
  fi
  printf 'warning: installer from %s failed; trying another source\n' "$url" >&2
done <<EOF
$INSTALLER_URLS
EOF

if [ -z "$INSTALLED_FROM" ]; then
  printf 'error: unable to install PVM from any configured source\n' >&2
  exit 1
fi
