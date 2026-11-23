package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const usage = `Usage:
  cmsg            Draft a Conventional Commit message from staged changes
  cmsg login      Save the Groq API key from GROQ_API_KEY or standard input
  cmsg --license  Show the license

Press Enter to commit, e to edit in your Git editor, or q to cancel.
Set GROQ_MODEL to override the default model.`

//go:embed LICENSE
var license string

func main() {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		func(ctx context.Context, key, model, diff string) (string, error) {
			return generateSubject(ctx, client, key, model, diff)
		})
	if err != nil {
		fmt.Fprintln(os.Stderr, "cmsg:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer,
	generate func(context.Context, string, string, string) (string, error),
) error {
	if len(args) > 0 {
		if len(args) == 1 {
			switch args[0] {
			case "-h", "--help", "help":
				fmt.Fprintln(out, usage)
				return nil
			case "login":
				return login(in, out)
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

	key, err := loadKey()
	if err != nil {
		return err
	}
	model := strings.TrimSpace(os.Getenv("GROQ_MODEL"))
	if model == "" {
		model = defaultModel
	}
	patch := diff.buf.String()
	if diff.truncated {
		fmt.Fprintln(errOut, "The staged diff exceeds 48 KiB. Groq will receive the first 48 KiB, starting with the diff stat.")
		patch += "\n[Diff truncated]\n"
	}
	fmt.Fprintln(errOut, "Drafting a commit message from the staged diff with Groq...")
	subject, err := generate(ctx, key, model, patch)
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
