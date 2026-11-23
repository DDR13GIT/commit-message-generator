#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/assets" "$tmp/tools" "$tmp/home"

asset="cmsg-$(go env GOOS)-$(go env GOARCH)"
go build -o "$tmp/assets/$asset" .
(
  cd "$tmp/assets"
  if command -v sha256sum >/dev/null; then
    sha256sum "$asset"
  else
    shasum -a 256 "$asset"
  fi > SHA256SUMS
)

cat > "$tmp/tools/curl" <<'MOCK'
#!/usr/bin/env bash
set -eu
[[ $1 == -fsSL && $2 == https://github.com/DDR13GIT/commit-message-generator/releases/latest/download/* && $3 == -o ]]
cp "$CMSG_TEST_ASSETS/${2##*/}" "$4"
MOCK
chmod +x "$tmp/tools/curl"

export HOME="$tmp/home" ZDOTDIR="$tmp/home" XDG_CONFIG_HOME="$tmp/home/config"
export PATH="$tmp/tools:/usr/bin:/bin" CMSG_TEST_ASSETS="$tmp/assets" SHELL=/bin/zsh
export GROQ_API_KEY=fake-test-token

output=$(bash ./install.sh)
[[ "$output" == *"Groq API key saved"* && "$output" == *"Run cmsg after staging your changes."* ]]
cmp "$HOME/.local/bin/cmsg" "$tmp/assets/$asset"
"$HOME/.local/bin/cmsg" --help >/dev/null
"$HOME/.local/bin/cmsg" --license > "$tmp/license"
cmp LICENSE "$tmp/license"
key_file=$(find "$HOME" -type f -name groq-key)
[[ $(cat "$key_file") == "$GROQ_API_KEY" ]]
bash ./install.sh >/dev/null
[[ $(grep -Fc '# Added by the cmsg installer' "$HOME/.zshrc") == 1 ]]

printf '%s\n' corrupt >> "$tmp/assets/$asset"
if bash ./install.sh >"$tmp/failure" 2>&1; then
  printf '%s\n' 'Installer accepted a corrupt download.' >&2
  exit 1
fi
grep -Fq 'Checksum verification failed' "$tmp/failure"
"$HOME/.local/bin/cmsg" --help >/dev/null
printf '%s\n' 'Installer checks passed.'
