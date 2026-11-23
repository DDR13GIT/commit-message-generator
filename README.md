# Commit Message Generator

Draft a one-line [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) message from your staged Git diff using [Groq](https://console.groq.com/docs/text-chat).

## Set up

Build `cmsg` with Go 1.24 or newer. From this directory, run:

```zsh
go build -o bin/cmsg .
export PATH="$PWD/bin:$PATH"
```

To make the command available in new terminals, add the absolute path to this `bin` directory to `PATH` in your zsh configuration.

Create an [API key in the Groq console](https://console.groq.com/keys). In zsh, paste the key at the hidden prompt:

```zsh
(
  read -rs 'groq_key?Groq API key: ' &&
    printf '\n' >&2 &&
    GROQ_API_KEY="$groq_key" cmsg login
)
```

If `GROQ_API_KEY` is already exported, run `cmsg login` to save it. The command also accepts a key piped through standard input. It saves the key to a user-only plaintext file in your OS user config directory and prints the path. An exported `GROQ_API_KEY` overrides the saved key.

## Draft a commit

```zsh
git add path/to/file
cmsg
```

- **Enter** commits with the suggested subject.
- **e**, then Enter, opens the subject in your Git editor. Save and close the editor to commit. Clear the message to abort from the editor.
- **q**, then Enter, cancels the commit.

The default model is [`openai/gpt-oss-20b`](https://console.groq.com/docs/model/openai/gpt-oss-20b). Set `GROQ_MODEL` to use another Groq chat model.

Running `cmsg` sends staged diff content to Groq before the review prompt. Unstaged and untracked file contents stay local. For large diffs, the CLI warns you and sends the first 48 KiB, starting with the file-change summary.

## Use it with gcam

Replace your existing `gcam` alias with this function in your zsh configuration:

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

This example assumes your current manual alias expands to `git commit -am`. If it uses other flags, keep those in the manual branch. With no argument, the AI flow commits what you staged with `git add`.
