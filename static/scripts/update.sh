#!/usr/bin/env bash
#
# Agenvoy updater - always overwrite to the latest release.
# Source clone is staged under /tmp and removed on exit / interrupt.
#
# Usage:
#   curl -fsSL https://cloud.agenvoy.com/update.sh -o /tmp/agenvoy-update.sh \
#     && bash /tmp/agenvoy-update.sh; rm -f /tmp/agenvoy-update.sh
#   agen update
#
set -euo pipefail

REPO_URL="https://github.com/agenvoy/agenvoy.git"
REPO_API="https://api.github.com/repos/agenvoy/agenvoy/releases/latest"
GO_INSTALL_DIR="${HOME}/.local/go"
REQUIRED_GO_MAJOR=1
REQUIRED_GO_MINOR=26

if [ -t 1 ]; then
  C_RED=$'\033[0;31m'; C_GRN=$'\033[0;32m'; C_YLW=$'\033[0;33m'
  C_BLU=$'\033[0;34m'; C_RST=$'\033[0m'
else
  C_RED=''; C_GRN=''; C_YLW=''; C_BLU=''; C_RST=''
fi

log()  { printf "%s==>%s %s\n" "$C_BLU" "$C_RST" "$*"; }
ok()   { printf "%s ok%s %s\n" "$C_GRN" "$C_RST" "$*"; }
warn() { printf "%s !!%s %s\n" "$C_YLW" "$C_RST" "$*"; }
die()  { printf "%s xx%s %s\n" "$C_RED" "$C_RST" "$*" >&2; exit 1; }

print_done() {
  local tag="$1"
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

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1${2:+ ($2)}"
}

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

persist_go_path() {
  local rc="" os
  os="$(uname -s)"
  case "${SHELL##*/}" in
    zsh)  rc="${HOME}/.zshrc" ;;
    bash) [ "$os" = "Darwin" ] && rc="${HOME}/.bash_profile" || rc="${HOME}/.bashrc" ;;
    *)    rc="${HOME}/.profile" ;;
  esac

  local marker_begin="# >>> agenvoy go path >>>"
  local marker_end="# <<< agenvoy go path <<<"
  local export_line="export PATH=\"${GO_INSTALL_DIR}/bin:\$PATH\""

  if [ -f "$rc" ] && grep -Fq "$marker_begin" "$rc"; then
    return 0
  fi

  mkdir -p "$(dirname "$rc")"
  {
    printf '\n%s\n' "$marker_begin"
    printf '%s\n' "$export_line"
    printf '%s\n' "$marker_end"
  } >> "$rc"
  ok "Persisted Go PATH to $rc"
  warn "Open a new shell or run: source $rc"
}

GO_TMP_DIR=""
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
  persist_go_path
}

ensure_go() {
  local platform; platform="$(detect_platform)"
  local current

  # Probe canonical install dir if go isn't on PATH (install.sh writes here
  # but the caller's shell rc may not have been sourced in this subprocess).
  if ! command -v go >/dev/null 2>&1 && [ -x "${GO_INSTALL_DIR}/bin/go" ]; then
    export PATH="${GO_INSTALL_DIR}/bin:${PATH}"
  fi

  if current="$(current_go_version)" && [ -n "$current" ]; then
    if go_version_ok "$current"; then
      ok "Go $current already meets >= ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR}"
      if [ "$(command -v go)" = "${GO_INSTALL_DIR}/bin/go" ]; then
        persist_go_path
      fi
      return 0
    fi
    warn "Go $current < ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR}, upgrading"
  else
    warn "Go not found, bootstrapping"
  fi
  install_go "$platform"
}

SRC_DIR=""
cleanup() {
  if [ -n "$SRC_DIR" ] && [ -d "$SRC_DIR" ]; then
    rm -rf "$SRC_DIR"
  fi
  if [ -n "$GO_TMP_DIR" ] && [ -d "$GO_TMP_DIR" ]; then
    rm -rf "$GO_TMP_DIR"
  fi
}
trap cleanup EXIT INT TERM

latest_tag() {
  curl -fsSL "$REPO_API" \
    | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' \
    | head -n 1 \
    | sed 's/.*"\([^"]*\)"$/\1/'
}

main() {
  log "Agenvoy updater"

  require_cmd curl
  require_cmd git
  require_cmd make
  ensure_go

  log "Resolving latest release tag..."
  local tag
  tag="$(latest_tag)"
  [ -n "$tag" ] || die "Failed to resolve latest tag from $REPO_API"
  log "Latest: $tag"

  SRC_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agenvoy-update.XXXXXX")"
  log "Cloning agenvoy@${tag} -> ${SRC_DIR}"
  git clone --depth 1 --branch "$tag" "$REPO_URL" "$SRC_DIR"

  log "Building (sudo prompt expected for /usr/local/bin install)"
  ( cd "$SRC_DIR" && make build )

  command -v agen >/dev/null 2>&1 \
    || die "agen not found on PATH after build (expected /usr/local/bin/agen)"
  ok "Updated to $tag at $(command -v agen)"

  log "Stopping old daemon (if any) so the new binary takes effect"
  agen stop || true

  print_done "$tag"
}

main "$@"
