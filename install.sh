#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'cmsg installer: %s\n' "$*" >&2
  exit 1
}

add_shell_path() {
  local profile=$1
  local marker='# Added by the cmsg installer'
  mkdir -p "$(dirname "$profile")"
  if [[ -f "$profile" ]] && grep -Fxq "$marker" "$profile"; then
    return
  fi
  printf '\n%s\n%s\n' "$marker" 'case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac' >> "$profile"
}

save_key() {
  local binary=$1 key
  if [[ -n ${GROQ_API_KEY:-} ]]; then
    "$binary" login
    return
  fi
  if [[ -t 0 ]]; then
    exec 3<&0
  elif ! { exec 3</dev/tty; } 2>/dev/null; then
    return
  fi
  if IFS= read -r -s -u 3 -p 'Groq API key (Enter to keep existing setup): ' key; then
    printf '\n' >&2
    if [[ -n "$key" ]]; then
      printf '%s\n' "$key" | "$binary" login
    fi
  fi
  exec 3<&-
}

main() {
  local os arch asset expected actual login_profile
  local shell=${SHELL:-}
  local base_url=https://github.com/DDR13GIT/commit-message-generator/releases/latest/download
  local bin_dir="$HOME/.local/bin"

  command -v git >/dev/null || fail 'Install Git first, then retry.'
  command -v curl >/dev/null || fail 'Install curl first, then retry.'
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) fail 'Supported operating systems are macOS and Linux.' ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch=arm64 ;;
    x86_64|amd64) arch=amd64 ;;
    *) fail 'Supported architectures are arm64 and amd64.' ;;
  esac

  asset="cmsg-$os-$arch"
  mkdir -p "$bin_dir"
  [[ ! -d "$bin_dir/cmsg" ]] || fail "$bin_dir/cmsg is a directory."
  tmp=$(mktemp -d "$bin_dir/.cmsg-install.XXXXXX")
  trap 'rm -rf "$tmp"' EXIT
  curl -fsSL "$base_url/$asset" -o "$tmp/$asset"
  curl -fsSL "$base_url/SHA256SUMS" -o "$tmp/SHA256SUMS"
  expected=$(awk -v asset="$asset" '$2 == asset {print $1}' "$tmp/SHA256SUMS")
  if command -v sha256sum >/dev/null; then
    actual=$(sha256sum "$tmp/$asset")
  else
    actual=$(shasum -a 256 "$tmp/$asset")
  fi
  [[ -n "$expected" && "$expected" == "${actual%% *}" ]] || fail 'Checksum verification failed.'
  chmod 0755 "$tmp/$asset"
  "$tmp/$asset" --help >/dev/null
  mv -f "$tmp/$asset" "$bin_dir/cmsg"

  case "${shell##*/}" in
    zsh) add_shell_path "${ZDOTDIR:-$HOME}/.zshrc" ;;
    bash|'')
      add_shell_path "$HOME/.bashrc"
      login_profile="$HOME/.profile"
      if [[ -f "$HOME/.bash_profile" ]]; then
        login_profile="$HOME/.bash_profile"
      elif [[ -f "$HOME/.bash_login" ]]; then
        login_profile="$HOME/.bash_login"
      fi
      add_shell_path "$login_profile"
      ;;
  esac

  printf 'Installed cmsg at %s\n' "$bin_dir/cmsg"
  save_key "$bin_dir/cmsg"
  if [[ -t 0 ]] && git rev-parse --is-inside-work-tree >/dev/null 2>&1 &&
    [[ -n $(git diff --cached --name-only --) ]]; then
    "$bin_dir/cmsg"
  else
    printf '%s\n' 'Run cmsg after staging your changes.'
  fi
}

main "$@"
