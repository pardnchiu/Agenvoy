#!/usr/bin/env bash
#
# Agenvoy installer
# Usage:  curl -fsSL https://cloud.agenvoy.com/install.sh | bash
#
set -euo pipefail

REPO_URL="https://github.com/agenvoy/agenvoy.git"
REPO_API="https://api.github.com/repos/agenvoy/agenvoy/releases/latest"
GO_INSTALL_DIR="${HOME}/.local/go"
REQUIRED_GO_MAJOR=1
REQUIRED_GO_MINOR=26

PKG_MGR=""
SUDO=""

SRC_DIR=""
GO_TMP_DIR=""
INSTALLED_TAG=""
cleanup() {
  if [ -n "$SRC_DIR" ] && [ -d "$SRC_DIR" ]; then
    rm -rf "$SRC_DIR"
  fi
  if [ -n "$GO_TMP_DIR" ] && [ -d "$GO_TMP_DIR" ]; then
    rm -rf "$GO_TMP_DIR"
  fi
}
trap cleanup EXIT INT TERM

if [ -t 1 ]; then
  C_RED=$'\033[0;31m'; C_GRN=$'\033[0;32m'; C_YLW=$'\033[0;33m'
  C_BLU=$'\033[0;34m'; C_RST=$'\033[0m'
else
  C_RED=''; C_GRN=''; C_YLW=''; C_BLU=''; C_RST=''
fi

log()  { printf "%s==>%s %s\n" "$C_BLU" "$C_RST" "$*"; }
ok()   { printf "%s ok%s %s\n"  "$C_GRN" "$C_RST" "$*"; }
warn() { printf "%s !!%s %s\n"  "$C_YLW" "$C_RST" "$*"; }
die()  { printf "%s xx%s %s\n"  "$C_RED" "$C_RST" "$*" >&2; exit 1; }

detect_platform() {
  local os arch
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux)  os=linux  ;;
    *) die "Unsupported OS: $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) die "Unsupported arch: $(uname -m)" ;;
  esac
  printf "%s-%s" "$os" "$arch"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1${2:+ ($2)}"
}

detect_pkg_mgr() {
  if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  fi
  if   command -v apt-get >/dev/null 2>&1; then PKG_MGR=apt
  elif command -v dnf     >/dev/null 2>&1; then PKG_MGR=dnf
  elif command -v yum     >/dev/null 2>&1; then PKG_MGR=yum
  elif command -v pacman  >/dev/null 2>&1; then PKG_MGR=pacman
  elif command -v apk     >/dev/null 2>&1; then PKG_MGR=apk
  elif command -v brew    >/dev/null 2>&1; then PKG_MGR=brew
  fi
  [ -n "$PKG_MGR" ] && log "Package manager: $PKG_MGR"
}

# Map logical package name -> distro-specific package
resolve_pkg() {
  case "$1:$PKG_MGR" in
    poppler:pacman|poppler:brew) printf "poppler" ;;
    poppler:*)                   printf "poppler-utils" ;;
    *)                           printf "%s" "$1" ;;
  esac
}

pkg_install() {
  local logical pkgs=()
  for logical in "$@"; do pkgs+=("$(resolve_pkg "$logical")"); done
  log "Installing: ${pkgs[*]} (via $PKG_MGR)"
  case "$PKG_MGR" in
    apt)
      $SUDO apt-get update -y || warn "apt-get update failed, continuing"
      $SUDO DEBIAN_FRONTEND=noninteractive apt-get install -y "${pkgs[@]}"
      ;;
    dnf)    $SUDO dnf install -y "${pkgs[@]}" ;;
    yum)    $SUDO yum install -y "${pkgs[@]}" ;;
    pacman) $SUDO pacman -Sy --noconfirm "${pkgs[@]}" ;;
    apk)    $SUDO apk add --no-cache "${pkgs[@]}" ;;
    brew)   brew install "${pkgs[@]}" ;;
    *) return 1 ;;
  esac
}

confirm_overwrite_agen() {
  command -v agen >/dev/null 2>&1 || return 0

  local existing
  existing="$(command -v agen)"
  log "agen already installed at: $existing"

  if [ ! -e /dev/tty ] || [ ! -r /dev/tty ]; then
    log "Non-interactive shell; keeping existing agen"
    exit 0
  fi

  printf "Overwrite existing agen? [y/N] " >/dev/tty
  local ans=""
  IFS= read -r ans </dev/tty || ans=""
  case "$ans" in
    y|Y|yes|YES|Yes) ok "Proceeding with reinstall" ;;
    *)
      ok "Keeping existing agen"
      exit 0
      ;;
  esac
}

ensure_cmd() {
  local cmd="$1" logical="${2:-$1}"
  command -v "$cmd" >/dev/null 2>&1 && return 0

  if [ "$(uname -s)" = "Darwin" ]; then
    case "$cmd" in
      make|git|cc|clang|gcc)
        die "$cmd not found on macOS. Run 'xcode-select --install' then re-run this installer."
        ;;
    esac
  fi

  [ -n "$PKG_MGR" ] || die "$cmd not found and no supported package manager detected. Install '$logical' manually."

  warn "$cmd missing, installing $logical"
  pkg_install "$logical" || die "Failed to install $logical"
  command -v "$cmd" >/dev/null 2>&1 || die "$cmd still missing after installing $logical"
}

# returns 0 if "$1" (e.g. 1.26.0) >= REQUIRED
go_version_ok() {
  local v="$1"
  local major minor
  major="${v%%.*}"; v="${v#*.}"; minor="${v%%.*}"
  case "$minor" in *[!0-9]*) minor="${minor%%[!0-9]*}" ;; esac
  [ -n "$major" ] && [ -n "$minor" ] || return 1
  if [ "$major" -gt "$REQUIRED_GO_MAJOR" ]; then return 0; fi
  if [ "$major" -eq "$REQUIRED_GO_MAJOR" ] && [ "$minor" -ge "$REQUIRED_GO_MINOR" ]; then return 0; fi
  return 1
}

current_go_version() {
  command -v go >/dev/null 2>&1 || return 1
  go version 2>/dev/null | awk '{print $3}' | sed 's/^go//'
}

install_go() {
  local platform="$1"
  local version url tarball
  log "Resolving latest Go release..."
  version="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n 1)"
  case "$version" in go*) ;; *) die "Unexpected Go version response: $version" ;; esac

  tarball="${version}.${platform}.tar.gz"
  url="https://go.dev/dl/${tarball}"
  GO_TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agenvoy-go.XXXXXX")"

  log "Downloading $url"
  curl -fSL --progress-bar "$url" -o "${GO_TMP_DIR}/${tarball}"

  log "Installing to ${GO_INSTALL_DIR}"
  rm -rf "$GO_INSTALL_DIR"
  mkdir -p "$(dirname "$GO_INSTALL_DIR")"
  tar -C "$(dirname "$GO_INSTALL_DIR")" -xzf "${GO_TMP_DIR}/${tarball}"

  export PATH="${GO_INSTALL_DIR}/bin:${PATH}"
  ok "Installed $(go version 2>/dev/null || echo "$version")"
  warn "Add this to your shell rc to keep Go on PATH:"
  printf '       export PATH="%s/bin:$PATH"\n' "$GO_INSTALL_DIR"
}

ensure_go() {
  local platform="$1"
  local current
  if current="$(current_go_version)" && [ -n "$current" ]; then
    if go_version_ok "$current"; then
      ok "Go $current already meets >= ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR}"
      return 0
    fi
    warn "Go $current < ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR}, upgrading"
  else
    warn "Go not found, installing"
  fi
  install_go "$platform"
}

latest_tag() {
  curl -fsSL "$REPO_API" \
    | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' \
    | head -n 1 \
    | sed 's/.*"\([^"]*\)"$/\1/'
}

clone_repo() {
  local tag
  log "Resolving latest Agenvoy release tag..."
  tag="$(latest_tag)"
  [ -n "$tag" ] || die "Failed to resolve latest release tag from $REPO_API"

  SRC_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agenvoy-install.XXXXXX")"
  log "Cloning agenvoy@${tag} -> ${SRC_DIR}"
  git clone --depth 1 --branch "$tag" "$REPO_URL" "$SRC_DIR"
  INSTALLED_TAG="$tag"
  ok "Cloned $tag"
}

build_and_install() {
  log "Building (sudo prompt expected for /usr/local/bin install)"
  ( cd "$SRC_DIR" && make build )
  command -v agen >/dev/null 2>&1 \
    || die "agen not found on PATH after build (expected /usr/local/bin/agen)"
  ok "agen installed at $(command -v agen)"
}

stop_daemon() {
  log "Stopping existing daemon (if any) so the new binary takes effect"
  agen stop || true
}

print_done() {
  local tag="${1:-installed}"
  local lines=(
    "Agenvoy ${tag} installed"
    ""
    "Next: run 'agen' to attach the new build"
  )

  local max=0 line len
  for line in "${lines[@]}"; do
    len=${#line}
    [ "$len" -gt "$max" ] && max=$len
  done

  local pad_each=2
  local inner=$((max + pad_each * 2))

  local border="" rpad=""
  local i=0
  while [ $i -lt $inner ]; do
    border="${border}─"
    i=$((i + 1))
  done

  printf '\n%s╭%s╮%s\n' "$C_GRN" "$border" "$C_RST"
  for line in "${lines[@]}"; do
    rpad=""
    i=0
    while [ $i -lt $((max - ${#line})) ]; do
      rpad="${rpad} "
      i=$((i + 1))
    done
    printf '%s│%s  %s%s  %s│%s\n' "$C_GRN" "$C_RST" "$line" "$rpad" "$C_GRN" "$C_RST"
  done
  printf '%s╰%s╯%s\n\n' "$C_GRN" "$border" "$C_RST"
}

main() {
  log "Agenvoy installer"
  local platform; platform="$(detect_platform)"
  log "Platform: $platform"

  confirm_overwrite_agen

  require_cmd curl
  require_cmd uname
  require_cmd tar

  detect_pkg_mgr
  ensure_cmd git
  ensure_cmd make
  ensure_cmd pdftotext poppler

  ensure_go "$platform"
  clone_repo
  build_and_install
  stop_daemon
  print_done "$INSTALLED_TAG"
}

main "$@"
