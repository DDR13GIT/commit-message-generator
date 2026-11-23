package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const usage = `Usage:
  cmsg                          Draft from staged changes
  cmsg keys                     Open the interactive key manager
  cmsg keys list
  cmsg keys add <groq|openrouter> <name>
  cmsg keys use <name>
  cmsg keys delete <name>
  cmsg login [groq|openrouter]  Save and select a default key
  cmsg --license                Show the license

Press Enter to commit, e to edit in your Git editor, or q to cancel.
Set GROQ_MODEL or OPENROUTER_MODEL to override the provider model.`

//go:embed LICENSE
var license string

func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer,
	generate func(context.Context, provider, string, string, string) (string, error),
) error {
	if len(args) > 0 {
		switch args[0] {
		case "keys":
			return keysCommand(args[1:], in, out)
		case "login":
			return login(args[1:], in, out)
		}
		if len(args) == 1 {
			switch args[0] {
			case "-h", "--help", "help":
				fmt.Fprintln(out, usage)
				return nil
			case "--license":
				_, err := fmt.Fprint(out, license)
				return err
			}
		}
		return errors.New("see cmsg -h")
	}

	tree, err := stagedTree(ctx)
	if err != nil {
		return err
	}
	diff := diffBuffer{}
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--stat", "--patch", "--no-color",
		"--no-ext-diff", "--no-textconv", "--no-relative", "--ignore-submodules=none", "--submodule=short", "--")
	if err := captureGit(cmd, &diff); err != nil {
		return err
	}
	if diff.buf.Len() == 0 {
		return errors.New("no staged changes. Stage the files you want to commit with git add")
	}

	p, key, err := activeKey()
	if err != nil {
		return err
	}
	model := strings.TrimSpace(os.Getenv(p.modelEnv))
	if model == "" {
		model = p.defaultModel
	}
	patch := diff.buf.String()
	if diff.truncated {
		fmt.Fprintf(errOut, "The staged diff exceeds 48 KiB. %s will receive the first 48 KiB, starting with the diff stat.\n", p.label)
		patch += "\n[Diff truncated]\n"
	}
	fmt.Fprintf(errOut, "Drafting a commit message from the staged diff with %s...\n", p.label)
	subject, err := generate(ctx, p, key, model, patch)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n%s\n\n", subject)
	reader := bufio.NewReader(in)
	for {
		fmt.Fprint(out, "Enter to commit, e to edit, q to cancel: ")
		reply, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(out, "\nCommit canceled.")
			return nil
		}
		if err != nil {
			return err
		}

		choice := strings.ToLower(strings.TrimSpace(reply))
		switch choice {
		case "q":
			fmt.Fprintln(out, "Commit canceled.")
			return nil
		case "", "e":
			currentTree, err := stagedTree(ctx)
			if err != nil {
				return err
			}
			if currentTree != tree {
				return errors.New("staged changes changed while drafting. Run cmsg again for the new diff")
			}
			commitArgs := []string{"commit", "-m", subject}
			if choice == "e" {
				commitArgs = append(commitArgs, "--edit")
			}
			cmd := exec.CommandContext(ctx, "git", commitArgs...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errOut
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("git commit: %w", err)
			}
			return nil
		}
	}
}

func stagedTree(ctx context.Context) (string, error) {
	var tree bytes.Buffer
	err := captureGit(exec.CommandContext(ctx, "git", "write-tree"), &tree)
	return strings.TrimSpace(tree.String()), err
}

func captureGit(cmd *exec.Cmd, out io.Writer) error {
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = out, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w\n%s", cmd.Args[1], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

type diffBuffer struct {
	buf       bytes.Buffer
	truncated bool
}

func (b *diffBuffer) Write(p []byte) (int, error) {
	const maxDiffBytes = 48 * 1024
	remaining := maxDiffBytes - b.buf.Len()
	if len(p) > remaining {
		b.truncated = true
	}
	b.buf.Write(p[:min(len(p), remaining)])
	return len(p), nil
}
