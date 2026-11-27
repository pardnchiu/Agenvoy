#!/usr/bin/env bash
#
# Agenvoy updater - always overwrite to the latest release.
#
# Usage:
#   curl -fsSL https://agenvoy.com/scripts/update.sh -o /tmp/agenvoy-update.sh \
#     && bash /tmp/agenvoy-update.sh; rm -f /tmp/agenvoy-update.sh
#   agen update
#
# Downloads the prebuilt binary from the latest GitHub release. No Go toolchain
# and no compilation: only the runtime dependencies agen shells out to.
#
set -euo pipefail

REPO="pardnchiu/Agenvoy"
BIN_DIR="/usr/local/bin"
AGENVOY_DIR="${HOME}/.config/agenvoy"
SKILLS_DIR="${AGENVOY_DIR}/skills/.system"
TOOLS_DIR="${AGENVOY_DIR}/tools/.system"
VENDOR_DIR="${AGENVOY_DIR}/vendor"

PKG_MGR=""
SUDO=""
TMP_DIR=""

cleanup() {
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
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

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1${2:+ ($2)}"
}

# A controlling terminal exists and is usable for prompting, even when stdin is
# a pipe (the `curl … | bash` case).
have_tty() {
  [ -e /dev/tty ] && [ -r /dev/tty ] && [ -w /dev/tty ]
}

source_build_hint() {
  printf '%s' "git clone https://github.com/${REPO}.git && cd Agenvoy && make build"
}

# ---------------------------------------------------------------- 1. platform

detect_platform() {
  local os arch
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux)  os=linux  ;;
    *) die "Unsupported OS: $(uname -s)" ;;
  esac
  if [ "$(uname -o 2>/dev/null)" = "Android" ]; then
    die "Android (Termux) cannot run the Linux binary directly. Install Ubuntu with proot-distro and re-run inside it: pkg install proot-distro && proot-distro install ubuntu && proot-distro login ubuntu, then apt update && apt install -y curl"
  fi
  case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) die "Unsupported arch: $(uname -m)" ;;
  esac

  # release.yml builds linux/amd64, linux/arm64, darwin/arm64 only.
  # Reject here, before any dependency install or sudo prompt.
  if [ "$os" = "darwin" ] && [ "$arch" = "amd64" ]; then
    die "Intel Mac has no prebuilt binary. Build from source instead: $(source_build_hint)"
  fi

  printf "%s-%s" "$os" "$arch"
}

# ---------------------------------------------------------- 2. package manager

is_admin_user() {
  case "$(uname -s)" in
    Darwin)
      # Group membership is the actual precondition Homebrew claims to check.
      id -Gn 2>/dev/null | tr ' ' '\n' | grep -qx admin
      ;;
    *)
      id -Gn 2>/dev/null | tr ' ' '\n' | grep -qxE 'sudo|wheel|admin'
      ;;
  esac
}

# Homebrew collapses three distinct failures into "needs to be an Administrator".
# Report which one actually applies so the user can act on it.
sudo_unavailable_reason() {
  if ! command -v sudo >/dev/null 2>&1; then
    printf '%s' "sudo is not installed. Run this installer as root, or install sudo first."
    return 0
  fi
  if ! is_admin_user; then
    printf '%s' "User '$(whoami)' is not in the admin group, so root access cannot be obtained. Re-run as an administrator account."
    return 0
  fi
  printf '%s' "No terminal is available to prompt for your sudo password, and the sudo timestamp is cold. This is NOT a permissions problem — '$(whoami)' is an administrator. Either run 'sudo -v' first in this same terminal (macOS ties the timestamp to the tty), or download the script and run it directly: curl -fsSL https://agenvoy.com/scripts/update.sh -o update.sh && bash update.sh"
}

ensure_sudo() {
  [ "$(id -u)" -eq 0 ] && return 0
  command -v sudo >/dev/null 2>&1 || return 1
  sudo -n true 2>/dev/null && return 0
  if have_tty; then
    warn "Need sudo to install into ${BIN_DIR}"
    sudo -v </dev/tty >/dev/tty 2>/dev/tty && return 0
  fi
  return 1
}

ensure_homebrew_darwin() {
  [ "$(uname -s)" = "Darwin" ] || return 0
  command -v brew >/dev/null 2>&1 && { ok "Homebrew already installed"; return 0; }

  warn "Homebrew not found, installing"

  # Homebrew's installer needs root. Under `curl … | bash` our stdin is the curl
  # pipe, so a plain `/bin/bash -c "$(…)"` child inherits a non-TTY stdin,
  # Homebrew switches to NONINTERACTIVE, its have_sudo_access() falls back to
  # `sudo -n`, and a cold sudo timestamp aborts with a misleading
  # "needs to be an Administrator". Warm the timestamp ourselves and hand the
  # child the real terminal.
  local brew_script
  brew_script="$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" \
    || die "Failed to download the Homebrew installer. Check network access to raw.githubusercontent.com, then re-run."
  [ -n "$brew_script" ] || die "Downloaded Homebrew installer was empty; refusing to execute."

  ensure_sudo || true

  if have_tty; then
    if ! /bin/bash -c "$brew_script" </dev/tty; then
      die "Homebrew installation failed (see output above). Install it manually from https://brew.sh, then re-run this updater."
    fi
  else
    # No controlling terminal at all (CI, MDM, remote provisioning).
    # Homebrew cannot prompt, so root access must already be available.
    if [ "$(id -u)" -ne 0 ] && ! sudo -n true 2>/dev/null; then
      die "$(sudo_unavailable_reason)"
    fi
    if ! NONINTERACTIVE=1 /bin/bash -c "$brew_script"; then
      die "Homebrew installation failed in non-interactive mode (see output above)."
    fi
  fi

  [ -x /opt/homebrew/bin/brew ] || die "Homebrew install completed but brew binary not found in /opt/homebrew/bin"
  eval "$(/opt/homebrew/bin/brew shellenv)"
  command -v brew >/dev/null 2>&1 || die "brew still not on PATH after eval shellenv"
  ok "Homebrew installed: $(brew --version | head -n 1)"
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
  # An AND-list as the last statement returns 1 under `set -e` when no package
  # manager is found, which would kill the updater with no message.
  if [ -n "$PKG_MGR" ]; then
    log "Package manager: $PKG_MGR"
  else
    warn "No supported package manager detected; runtime dependencies must be installed manually"
  fi
}

# Map logical package name -> distro-specific package
resolve_pkg() {
  case "$1:$PKG_MGR" in
    poppler:pacman|poppler:brew) printf "poppler" ;;
    poppler:*)                   printf "poppler-utils" ;;
    python3:pacman)              printf "python" ;;
    python3:*)                   printf "python3" ;;
    nodejs:brew)                 printf "node" ;;
    nodejs:*)                    printf "nodejs" ;;
    bubblewrap:*)                printf "bubblewrap" ;;
    libsecret:apt)               printf "libsecret-tools" ;;
    libsecret:*)                 printf "libsecret" ;;
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
      $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y "${pkgs[@]}"
      ;;
    dnf)    $SUDO dnf install -y "${pkgs[@]}" ;;
    yum)    $SUDO yum install -y "${pkgs[@]}" ;;
    pacman) $SUDO pacman -Sy --noconfirm "${pkgs[@]}" ;;
    apk)    $SUDO apk add --no-cache "${pkgs[@]}" ;;
    brew)
      local prefix; prefix="$(brew --prefix)"
      if [ ! -w "$prefix" ]; then
        warn "$prefix not writable, fixing ownership (sudo prompt expected)"
        sudo chown -R "$(whoami)" "$prefix"
      fi
      brew install "${pkgs[@]}"
      ;;
    *) return 1 ;;
  esac
}

ensure_cmd() {
  local cmd="$1" logical="${2:-$1}"
  command -v "$cmd" >/dev/null 2>&1 && return 0

  [ -n "$PKG_MGR" ] || die "$cmd not found and no supported package manager detected. Install '$logical' manually."

  warn "$cmd missing, installing $logical"
  pkg_install "$logical" || die "Failed to install $logical"
  command -v "$cmd" >/dev/null 2>&1 || die "$cmd still missing after installing $logical"
}

# go-rod launches ~/.cache/rod/browser/.../chrome directly (not apt's chromium
# package), so apt won't pull its shared-lib deps automatically. libasound2 is
# the one rod's chromium needs that a bare Debian/Ubuntu host lacks; minimal
# WSL images additionally ship no CJK fonts, so Chinese renders as tofu boxes.
ensure_chrome_deps() {
  [ "$PKG_MGR" = "apt" ] || return 0

  log "Ensuring libasound2 for go-rod (apt)"
  $SUDO apt-get update -y || warn "apt-get update failed, continuing"
  if $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y libasound2 2>/dev/null; then
    ok "libasound2 installed"
  else
    # Newer Ubuntu (24.04+) renamed libasound2 -> libasound2t64; retry with that name.
    warn "libasound2 install failed, retrying with libasound2t64 (newer Ubuntu package name)"
    if $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y libasound2t64; then
      ok "libasound2t64 installed"
    else
      warn "Failed to install libasound2/libasound2t64; go-rod browser automation may not launch"
    fi
  fi

  log "Ensuring CJK fonts for go-rod (apt)"
  if $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y fonts-noto-cjk; then
    ok "fonts-noto-cjk installed"
  else
    warn "Failed to install fonts-noto-cjk; Chinese/Japanese/Korean text may render as tofu boxes"
  fi
}

install_dependencies() {
  ensure_cmd tar
  ensure_cmd pdftotext poppler
  ensure_cmd python3
  ensure_cmd node nodejs

  if [ "$(uname -s)" = "Linux" ]; then
    # sandbox: linux uses bubblewrap, macOS uses the built-in sandbox-exec
    ensure_cmd bwrap bubblewrap
    # keychain: go-pkg/filesystem/keychain shells out to secret-tool on linux
    # and, when it fails, silently writes secrets to a plaintext .secrets file
    ensure_cmd secret-tool libsecret
  fi
  ensure_chrome_deps
}

# --------------------------------------------------------------- 3. download

# The 302 from /releases/latest/download/ carries the resolved tag and is
# served Cache-Control: no-cache. Avoids api.github.com's 60 req/hr/IP limit.
resolve_tag() {
  local loc dir
  loc="$(curl -fsSI -o /dev/null -w '%{redirect_url}' \
    "https://github.com/${REPO}/releases/latest/download/x")" \
    || die "Failed to resolve latest release tag"
  dir="${loc%/*}"
  [ -n "${dir##*/}" ] || die "Unexpected redirect target: $loc"
  printf "%s" "${dir##*/}"
}

verify_checksum() {
  local dir="$1" asset="$2"
  (
    cd "$dir"
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum -c "${asset}.sha256"
    else
      shasum -a 256 -c "${asset}.sha256"
    fi
  ) >/dev/null 2>&1
}

download_release() {
  local platform="$1" tag="$2"
  local asset url
  asset="agen-${tag}-${platform}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

  TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agenvoy-install.XXXXXX")"

  log "Downloading $url"
  curl -fSL --retry 3 --retry-delay 2 --progress-bar "$url" -o "${TMP_DIR}/${asset}" \
    || die "Download failed: $asset"

  curl -fsSL --retry 3 -o "${TMP_DIR}/${asset}.sha256" "${url}.sha256" \
    || die "No .sha256 published for ${asset}; refusing to install without an integrity check."
  verify_checksum "$TMP_DIR" "$asset" || die "Checksum mismatch for ${asset}; refusing to install."
  ok "Checksum verified"

  tar -C "$TMP_DIR" -xzf "${TMP_DIR}/${asset}"
  [ -f "${TMP_DIR}/agen" ]   || die "Archive did not contain 'agen'"
  [ -d "${TMP_DIR}/skills" ] || die "Archive did not contain 'skills/'"
  chmod +x "${TMP_DIR}/agen"
  ok "Extracted $tag"
}

# ---------------------------------------------------------------- 4. install

# Doubles as the load test: a prebuilt binary that cannot run here (musl host,
# glibc older than 2.34) fails at exec, not at first use. Running the new
# binary rather than the installed one is what makes this a load test.
stop_daemon_and_verify() {
  log "Stopping existing daemon (if any) and checking the binary runs here"
  local out rc
  set +e
  out="$("${TMP_DIR}/agen" stop 2>&1)"
  rc=$?
  set -e
  if [ "$rc" -ne 0 ]; then
    case "$out" in
      *GLIBC*|*"cannot execute"*|*"No such file or directory"*|*"not found"*|*Killed*|*"Exec format error"*)
        printf '%s\n' "$out" >&2
        die "The prebuilt binary does not run on this system. Build from source instead: $(source_build_hint)"
        ;;
    esac
    warn "agen stop exited ${rc}: ${out}"
  fi
}

# `command -v agen` succeeding proves nothing: a stale copy earlier on PATH
# (~/.local/bin, brew) answers the same way and the user keeps running the old
# binary while the updater reports success.
verify_path_resolution() {
  hash -r 2>/dev/null || true
  local resolved
  resolved="$(command -v agen 2>/dev/null || true)"
  if [ -z "$resolved" ]; then
    warn "${BIN_DIR} is not on PATH; run ${BIN_DIR}/agen directly or add it to PATH"
  elif [ "$resolved" != "${BIN_DIR}/agen" ]; then
    warn "Another agen shadows the new install: $resolved (new binary is at ${BIN_DIR}/agen)"
  fi
}

install_release() {
  ensure_sudo || die "$(sudo_unavailable_reason)"
  log "Installing binary to ${BIN_DIR}"
  $SUDO mkdir -p "$BIN_DIR"
  $SUDO install -m 0755 "${TMP_DIR}/agen" "${BIN_DIR}/agen"
  [ -x "${BIN_DIR}/agen" ] || die "Install failed: ${BIN_DIR}/agen is missing"

  # Wipe before copy: cp -R only overwrites same-named entries, so a skill
  # removed or renamed in this release would linger and still be registered
  # by SkillScanner. Only .system and vendor are touched; sibling user skills survive.
  # tools/.system stays empty because the default api tools live in
  # extensions/apis, which is go:embed'd into the binary.
  log "Installing bundled skills to ${SKILLS_DIR}"
  rm -rf "$SKILLS_DIR" "$TOOLS_DIR" "$VENDOR_DIR"
  mkdir -p "$SKILLS_DIR" "$TOOLS_DIR"
  cp -R "${TMP_DIR}/skills/." "${SKILLS_DIR}/"
  find "$SKILLS_DIR" -name __pycache__ -prune -exec rm -rf {} + 2>/dev/null || true

  verify_path_resolution
  ok "agen installed at ${BIN_DIR}/agen"
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

# ------------------------------------------------------------------- main

main() {
  log "Agenvoy updater"

  require_cmd uname
  require_cmd curl

  local platform; platform="$(detect_platform)"
  log "Platform: $platform"

  ensure_homebrew_darwin
  detect_pkg_mgr
  install_dependencies

  local tag; tag="$(resolve_tag)"
  log "Latest release: $tag"

  download_release "$platform" "$tag"
  stop_daemon_and_verify
  install_release
  print_done "$tag"

  sleep 2
  exec "${BIN_DIR}/agen"
}

main "$@"
