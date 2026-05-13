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

if [ -t 1 ]; then
  C_RED=$'\033[0;31m'; C_GRN=$'\033[0;32m'
  C_BLU=$'\033[0;34m'; C_RST=$'\033[0m'
else
  C_RED=''; C_GRN=''; C_BLU=''; C_RST=''
fi

log() { printf "%s==>%s %s\n" "$C_BLU" "$C_RST" "$*"; }
ok()  { printf "%s ok%s %s\n" "$C_GRN" "$C_RST" "$*"; }
die() { printf "%s xx%s %s\n" "$C_RED" "$C_RST" "$*" >&2; exit 1; }

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

SRC_DIR=""
cleanup() {
  if [ -n "$SRC_DIR" ] && [ -d "$SRC_DIR" ]; then
    rm -rf "$SRC_DIR"
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
  require_cmd go "run install.sh first to bootstrap toolchain"

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
