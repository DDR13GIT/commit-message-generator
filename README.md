# Commit Message Generator

`cmsg` drafts a [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) subject from your staged Git changes using Groq.

## Install

Use Bash or zsh on macOS or Linux (arm64 or x86-64), with Git and curl installed. Get a [Groq API key](https://console.groq.com/keys). Run the installer from a Git repo with staged changes to draft your first message immediately. This sends the staged diff to Groq.

```sh
cmsg_installer=$(curl -fsSL https://github.com/DDR13GIT/commit-message-generator/releases/latest/download/install.sh) && export PATH="$HOME/.local/bin:$PATH" && bash -c "$cmsg_installer"
```

Paste your key at the hidden prompt. The installer verifies the binary's SHA-256 checksum, installs it to `~/.local/bin/cmsg`, and adds that directory to your Bash or zsh PATH. It runs without Go or sudo. Run the same command to update, and press Enter at the prompt to keep your saved key. If `GROQ_API_KEY` is already exported, the installer saves it automatically.

## Draft a commit

```sh
git add path/to/file
cmsg
```

- **Enter** commits with the suggested subject.
- **e**, then Enter, opens the draft in your Git editor. Save and close the editor to commit. Clear the message to abort.
- **q**, then Enter, cancels the commit.

`cmsg` sends the staged diff to Groq before it shows the draft. Unstaged and untracked file contents stay local. For diffs over 48 KiB, the CLI warns you and sends the first 48 KiB, starting with the file-change summary.

## Configure Groq

The default model is [`openai/gpt-oss-20b`](https://console.groq.com/docs/model/openai/gpt-oss-20b). Set `GROQ_MODEL` to use a different Groq chat model.

To save or replace your key without rerunning the installer, use this hidden prompt in Bash or zsh:

```sh
(
  printf 'Groq API key: ' >&2
  IFS= read -rs groq_key && printf '\n' >&2 && GROQ_API_KEY="$groq_key" cmsg login
)
```

If `GROQ_API_KEY` is exported, `cmsg login` saves that value. The key is stored as plaintext in a file readable and writable only by your user. The path is `~/Library/Application Support/git-cmsg/groq-key` on macOS, and `${XDG_CONFIG_HOME:-$HOME/.config}/git-cmsg/groq-key` on Linux. An exported `GROQ_API_KEY` takes precedence over the file.

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

With Git and Go 1.24 or newer, run from the project directory:

```sh
go build -o bin/cmsg .
export PATH="$PWD/bin:$PATH"
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
