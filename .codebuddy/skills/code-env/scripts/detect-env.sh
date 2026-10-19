#!/usr/bin/env bash
# Detect the current operating system, CPU architecture, shell, and existing PVM
# installation, then print the matching PVM install target.
#
# Output: key=value lines plus a human-readable recommendation.
# Exit codes: 0 supported, 1 unsupported environment, 2 usage error.

if [ -z "${BASH_VERSION:-}" ]; then
  printf 'error: run this script with bash\n' >&2
  exit 2
fi

set -euo pipefail

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      printf 'Usage: detect-env.sh\n'
      exit 0
      ;;
    *)
      printf 'error: unknown argument: %s\n' "$1" >&2
      exit 2
      ;;
  esac
done

# --- operating system ---
OS_NAME="$(uname -s 2>/dev/null || true)"
case "$OS_NAME" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  MINGW*|MSYS*|CYGWIN*) os=windows-gitbash ;;
  *) os=unknown ;;
esac

# --- CPU architecture (resolve Rosetta to native Apple Silicon) ---
MACHINE="$(uname -m 2>/dev/null || true)"
rosetta=false
if [ "$os" = darwin ] && [ "$MACHINE" = x86_64 ] && command -v sysctl >/dev/null 2>&1; then
  if [ "$(sysctl -in sysctl.proc_translated 2>/dev/null || true)" = 1 ]; then
    MACHINE=arm64
    rosetta=true
  fi
fi

case "$MACHINE" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) arch=unknown ;;
esac

SHELL_NAME="$(basename "${SHELL:-unknown}")"

# --- PVM install target per environment ---
case "$os:$arch" in
  darwin:amd64|darwin:arm64|linux:amd64|linux:arm64)
    supported=true
    asset="pvm-$os-$arch"
    installer=install-pvm.sh
    ;;
  windows-gitbash:*)
    supported=true
    asset="pvm-windows-$arch.exe"
    installer=install-pvm.ps1
    ;;
  *)
    supported=false
    asset=""
    installer=""
    ;;
esac
if [ "$arch" = unknown ]; then
  supported=false
  asset=""
  installer=""
fi

# --- existing PVM installation ---
PVM_HOME_RESOLVED="${PVM_HOME:-$HOME/.pvm}"
pvm_path=""
if command -v pvm >/dev/null 2>&1; then
  pvm_path="$(command -v pvm)"
elif [ -x "$PVM_HOME_RESOLVED/bin/pvm" ]; then
  pvm_path="$PVM_HOME_RESOLVED/bin/pvm"
fi

pvm_version=""
if [ -n "$pvm_path" ]; then
  pvm_version="$("$pvm_path" version 2>/dev/null | awk '$1 == "pvm" {print $2; exit}')"
  pvm_version="${pvm_version#v}"
fi

# --- report ---
printf 'os=%s\n' "$os"
printf 'arch=%s\n' "$arch"
printf 'arch_raw=%s\n' "${MACHINE:-unknown}"
printf 'rosetta=%s\n' "$rosetta"
printf 'shell=%s\n' "$SHELL_NAME"
printf 'supported=%s\n' "$supported"
if [ -n "$pvm_path" ]; then
  printf 'pvm_installed=true\n'
  printf 'pvm_path=%s\n' "$pvm_path"
else
  printf 'pvm_installed=false\n'
fi
if [ -n "$pvm_version" ]; then
  printf 'pvm_version=%s\n' "$pvm_version"
fi
if [ -n "$asset" ]; then
  printf 'asset=%s\n' "$asset"
fi
if [ -n "$installer" ]; then
  printf 'installer=%s\n' "$installer"
fi
printf '\n'

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

if [ "$supported" = false ]; then
  printf 'No PVM release is available for %s/%s.\n' "${OS_NAME:-unknown}" "${MACHINE:-unknown}" >&2
  exit 1
fi

if [ -n "$pvm_version" ]; then
  printf 'PVM %s is already installed at:\n  %s\n' "$pvm_version" "$pvm_path"
  printf 'No reinstallation is needed. To upgrade, run:\n  pvm self-update\n'
else
  printf 'Detected %s/%s%s. Recommended install:\n' \
    "$os" "$arch" "$([ "$rosetta" = true ] && printf ' (native Apple Silicon under Rosetta)')"
  if [ "$os" = windows-gitbash ]; then
    printf '  powershell -ExecutionPolicy Bypass -File "%s\\install-pvm.ps1"\n' "$SCRIPT_DIR"
  else
    printf '  bash "%s/install-pvm.sh"\n' "$SCRIPT_DIR"
  fi
  printf 'Or the one-line remote installer:\n'
  if [ "$os" = windows-gitbash ]; then
    printf '  irm https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex\n'
  else
    printf '  curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash\n'
  fi
fi
