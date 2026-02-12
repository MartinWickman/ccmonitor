#!/bin/sh
# install.sh — Install ccmonitor from GitHub Releases
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/MartinWickman/ccmonitor/main/install.sh | sh
#   curl -sSfL https://raw.githubusercontent.com/MartinWickman/ccmonitor/main/install.sh | sh -s -- -b ~/.local/bin
#   curl -sSfL https://raw.githubusercontent.com/MartinWickman/ccmonitor/main/install.sh | sh -s -- v0.8.0
set -e

OWNER="martinwickman"
REPO="ccmonitor"
BINARY="ccmonitor"
GITHUB_DOWNLOAD="https://github.com/${OWNER}/${REPO}/releases/download"

usage() {
  cat <<EOF
Usage: $0 [-b bindir] [version]
  -b      Installation directory (default: /usr/local/bin, or ~/.local/bin if no write access)
  version Version tag to install (default: latest)
EOF
  exit 2
}

# --- Platform detection ---------------------------------------------------

uname_os() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    msys*|mingw*|cygwin*) os="windows" ;;
  esac
  echo "$os"
}

uname_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64)        arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
  esac
  echo "$arch"
}

check_platform() {
  case "${OS}/${ARCH}" in
    linux/amd64|linux/arm64) return 0 ;;
    darwin/amd64|darwin/arm64) return 0 ;;
    windows/amd64|windows/arm64) return 0 ;;
  esac
  echo "Error: unsupported platform ${OS}/${ARCH}" >&2
  echo "Check https://github.com/${OWNER}/${REPO}/releases for available binaries." >&2
  exit 1
}

# --- Utilities -------------------------------------------------------------

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

http_download() {
  local_file=$1
  source_url=$2
  if has_cmd curl; then
    curl --fail -sSL -o "$local_file" "$source_url"
  elif has_cmd wget; then
    wget -q -O "$local_file" "$source_url"
  else
    echo "Error: need curl or wget to download files" >&2
    exit 1
  fi
}

latest_version() {
  giturl="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
  header=""
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    header="Authorization: token $GITHUB_TOKEN"
  fi

  if has_cmd curl; then
    if [ -n "$header" ]; then
      json=$(curl --fail -sSL -H "$header" "$giturl")
    else
      json=$(curl --fail -sSL "$giturl")
    fi
  elif has_cmd wget; then
    if [ -n "$header" ]; then
      json=$(wget -q --header "$header" -O - "$giturl")
    else
      json=$(wget -q -O - "$giturl")
    fi
  else
    echo "Error: need curl or wget" >&2
    exit 1
  fi

  # Parse tag_name without jq
  version=$(echo "$json" | tr ',' '\n' | grep '"tag_name"' | head -1 | cut -f4 -d'"')
  if [ -z "$version" ]; then
    echo "Error: could not determine latest version from GitHub" >&2
    echo "Set GITHUB_TOKEN if you are being rate-limited." >&2
    exit 1
  fi
  echo "$version"
}

untar() {
  tarball=$1
  case "${tarball}" in
    *.tar.gz|*.tgz) tar --no-same-owner -xzf "${tarball}" ;;
    *.zip) unzip -o "${tarball}" ;;
    *)
      echo "Error: unknown archive format: ${tarball}" >&2
      return 1
      ;;
  esac
}

hash_sha256() {
  target=$1
  if has_cmd sha256sum; then
    sha256sum "$target" | cut -d ' ' -f 1
  elif has_cmd shasum; then
    shasum -a 256 "$target" | cut -d ' ' -f 1
  elif has_cmd openssl; then
    openssl dgst -sha256 "$target" | cut -d ' ' -f 2
  else
    echo "Warning: no sha256 command found, skipping checksum verification" >&2
    return 1
  fi
}

verify_checksum() {
  tarball_path=$1
  checksum_path=$2
  basename_file=$(basename "$tarball_path")

  want=$(grep "${basename_file}" "${checksum_path}" | tr '\t' ' ' | cut -d ' ' -f 1)
  if [ -z "$want" ]; then
    echo "Warning: checksum not found for ${basename_file}, skipping verification" >&2
    return 0
  fi
  got=$(hash_sha256 "$tarball_path") || return 0
  if [ "$want" != "$got" ]; then
    echo "Error: checksum mismatch for ${basename_file}" >&2
    echo "  expected: ${want}" >&2
    echo "  got:      ${got}" >&2
    return 1
  fi
}

# --- Argument parsing ------------------------------------------------------

parse_args() {
  BINDIR=""
  VERSION=""
  while getopts "b:h?" arg; do
    case "$arg" in
      b) BINDIR="$OPTARG" ;;
      h|?) usage ;;
    esac
  done
  shift $((OPTIND - 1))
  VERSION="${1:-}"
}

resolve_bindir() {
  if [ -n "$BINDIR" ]; then
    return
  fi
  if [ -w "/usr/local/bin" ]; then
    BINDIR="/usr/local/bin"
  else
    BINDIR="${HOME}/.local/bin"
  fi
}

# --- Main installation (wrapped for curl|sh safety) ------------------------

execute() {
  tmpdir=$(mktemp -d)
  trap 'rm -rf "${tmpdir}"' EXIT

  if [ -z "$VERSION" ]; then
    echo "Finding latest version..."
    VERSION=$(latest_version)
  fi

  tag="${VERSION}"
  version_num="${VERSION#v}"

  echo "Installing ${BINARY} ${tag} (${OS}/${ARCH})"

  case "$OS" in
    windows) ext="zip" ;;
    *)       ext="tar.gz" ;;
  esac

  archive="${BINARY}_${version_num}_${OS}_${ARCH}.${ext}"
  archive_url="${GITHUB_DOWNLOAD}/${tag}/${archive}"
  checksum_file="${BINARY}_${version_num}_checksums.txt"
  checksum_url="${GITHUB_DOWNLOAD}/${tag}/${checksum_file}"

  echo "Downloading ${archive}..."
  http_download "${tmpdir}/${archive}" "${archive_url}"

  echo "Verifying checksum..."
  http_download "${tmpdir}/${checksum_file}" "${checksum_url}" 2>/dev/null && \
    verify_checksum "${tmpdir}/${archive}" "${tmpdir}/${checksum_file}" || \
    echo "Skipping checksum verification."

  echo "Extracting..."
  (cd "${tmpdir}" && untar "${archive}")

  bin_name="${BINARY}"
  if [ "$OS" = "windows" ]; then
    bin_name="${BINARY}.exe"
  fi

  install -d "${BINDIR}"
  install "${tmpdir}/${bin_name}" "${BINDIR}/"

  echo ""
  echo "Installed ${BINDIR}/${bin_name}"

  case ":${PATH}:" in
    *":${BINDIR}:"*) ;;
    *)
      echo ""
      echo "NOTE: ${BINDIR} is not in your PATH."
      echo "Add it with:  export PATH=\"${BINDIR}:\$PATH\""
      ;;
  esac

  echo ""
  echo "Next, register the hooks in any Claude Code session:"
  echo "  /plugin marketplace add MartinWickman/ccmonitor"
  echo "  /plugin install ccmonitor"
}

# --- Entry point -----------------------------------------------------------

OS=$(uname_os)
ARCH=$(uname_arch)

check_platform
parse_args "$@"
resolve_bindir
execute
