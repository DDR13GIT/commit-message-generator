# Commit Message Generator

Turn staged Git changes into a [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) message with **Groq or OpenRouter**. `cmsg` is a Go CLI that uses your own API key.

- **Use your preferred provider.** Run Groq directly or choose a model through OpenRouter.
- **Switch keys from your terminal.** Save named keys for both providers and select the one you want with `cmsg keys`.
- **Review before committing.** Press **Enter** to accept the draft, **e** to open your Git editor, or **q** to cancel.

```sh
cmsg keys   # Add an API key and select your provider
cmsg        # Draft a message after staging changes with git add
```

## Install

Use Bash or zsh on macOS or Linux (arm64 or x86-64), with Git and curl installed. Get a [Groq key](https://console.groq.com/keys) or an [OpenRouter key](https://openrouter.ai/keys). Run the installer from a Git repo with staged changes to draft your first message immediately. This sends the staged diff to your selected provider.

```sh
cmsg_installer=$(curl -fsSL https://github.com/DDR13GIT/commit-message-generator/releases/latest/download/install.sh) && export PATH="$HOME/.local/bin:$PATH" && bash -c "$cmsg_installer"
```

Paste your key at the hidden prompt and choose its provider. The installer verifies the binary's SHA-256 checksum, installs it to `~/.local/bin/cmsg`, and adds that directory to your Bash or zsh PATH. It runs without Go or sudo. Run the same command to update, and press Enter at the key prompt to keep your saved selection. If `GROQ_API_KEY` or `OPENROUTER_API_KEY` is already exported, the installer saves and selects that provider's key, preferring Groq when both are set. To try changes ahead of the latest release, [build from source](#build-from-source).

## Draft a commit

```sh
git add path/to/file
cmsg
```

- **Enter** commits with the suggested subject.
- **e**, then Enter, opens the draft in your Git editor. Save and close the editor to commit. Clear the message to abort.
- **q**, then Enter, cancels the commit.

`cmsg` sends the staged diff to the active key's provider before it shows the draft. Unstaged and untracked file contents stay local. For diffs over 48 KiB, the CLI warns you and sends the first 48 KiB, starting with the file-change summary.

## Providers and API keys

| Provider | Get a key | Default model | Model override |
| --- | --- | --- | --- |
| **Groq** | [Groq Console](https://console.groq.com/keys) | [`openai/gpt-oss-20b`](https://console.groq.com/docs/model/openai/gpt-oss-20b) | `GROQ_MODEL` |
| **OpenRouter** | [OpenRouter Keys](https://openrouter.ai/keys) | [`openrouter/auto`](https://openrouter.ai/openrouter/auto) | `OPENROUTER_MODEL` |

Run `cmsg keys` to open the key manager. Press **a** to add a named key. Use the arrow keys to highlight a key, **Enter** to activate it, or **d** to delete it from this device. Key entry is masked, and the key list shows names and providers.

The first saved key becomes active. Adding more keys keeps your current selection. Deleting the active key clears that selection. Existing Groq keys appear as `default` and move into the new store on the first key change.

For scripting, `keys add` accepts the key on standard input or from the provider's environment variable. This example uses an exported `OPENROUTER_API_KEY`:

```sh
cmsg keys add openrouter work </dev/null
cmsg keys list
cmsg keys use work
cmsg keys delete work
```

`cmsg login` saves and activates the Groq `default` key. `cmsg login openrouter` does the same for the `openrouter` key. Both accept a key from the provider's environment variable or standard input.

Saved keys are stored as plaintext in a user-only `keys.json` file under `~/Library/Application Support/git-cmsg/` on macOS or `${XDG_CONFIG_HOME:-$HOME/.config}/git-cmsg/` on Linux. An active key in this file takes precedence over environment variables. With no saved selection, `cmsg` uses `GROQ_API_KEY` or `OPENROUTER_API_KEY`, preferring Groq if both are set. The old `groq-key` file still respects `GROQ_API_KEY` until it is migrated.

## Use it with gcam

If your existing `gcam` alias runs `git commit -am`, replace it with this function in your zsh configuration:

```zsh
unalias gcam 2>/dev/null
gcam() {
  if (( $# )); then
    git commit -am "$@"
  else
    cmsg
  fi
}
```

Run `gcam` after staging changes to draft a message, or keep using `gcam "my message"` for manual commits. If your existing alias uses other flags, carry them over to the manual branch.

## Build from source

With Git and Go 1.24 or newer:

```sh
git clone https://github.com/DDR13GIT/commit-message-generator.git
cd commit-message-generator
go build -o bin/cmsg .
export PATH="$PWD/bin:$PATH"
```

To run the tests:

```sh
go test ./...
bash scripts/test-install.sh
```

Maintainers can build the four platform binaries and their checksum manifest locally with `bash scripts/build-release.sh`. To publish, push `main` to the public GitHub repository, then create and push a version tag such as `v0.1.0`:

```sh
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

The release workflow builds and publishes the binaries, checksum manifest, and installer. The one-command installer becomes available after the first release is published.

## License

[MIT](LICENSE). Run `cmsg --license` to display the license from an installed binary.
