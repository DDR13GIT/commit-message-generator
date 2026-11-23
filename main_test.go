package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitFlow(t *testing.T) {
	for _, tc := range []struct {
		name, input, subject string
	}{
		{"accept", "\n", "feat: add example"},
		{"edit", "e\n", "fix: use edited subject"},
		{"cancel", "q\n", ""},
		{"closed-input", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupRepo(t)
			t.Setenv("GIT_EDITOR", "sed '1c\\\nfix: use edited subject\n' \"$1\" > \"$1.edited\" && mv \"$1.edited\" \"$1\"; :")
			t.Setenv("GROQ_API_KEY", "fake-test-token")
			var output bytes.Buffer
			err := run(context.Background(), nil, strings.NewReader(tc.input), &output, &output,
				func(_ context.Context, key, model, diff string) (string, error) {
					if key != "fake-test-token" || model != defaultModel {
						t.Fatal("unexpected Groq configuration")
					}
					if !strings.Contains(diff, "+staged content") || strings.Contains(diff, "unstaged content") {
						t.Fatalf("unexpected diff: %q", diff)
					}
					return "feat: add example", nil
				})
			if err != nil {
				t.Fatalf("%v\n%s", err, &output)
			}
			if tc.subject == "" {
				if err := exec.Command("git", "rev-parse", "--verify", "HEAD").Run(); err == nil {
					t.Fatal("commit created without confirmation")
				}
				return
			}
			if got := gitTest(t, "log", "-1", "--format=%s"); got != tc.subject {
				t.Fatalf("commit subject = %q, want %q", got, tc.subject)
			}
			if got := gitTest(t, "show", "HEAD:example.txt"); got != "staged content" {
				t.Fatalf("commit content = %q", got)
			}
		})
	}
}

func TestBlocksChangedIndex(t *testing.T) {
	setupRepo(t)
	t.Setenv("GROQ_API_KEY", "fake-test-token")
	err := run(context.Background(), nil, strings.NewReader("\n"), io.Discard, io.Discard,
		func(_ context.Context, key, model, diff string) (string, error) {
			gitTest(t, "add", "example.txt")
			return "feat: add example", nil
		})
	if err == nil || !strings.Contains(err.Error(), "staged changes changed") {
		t.Fatalf("expected a stale-diff error, got %v", err)
	}
}

func TestSaveKey(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	err := run(context.Background(), []string{"login"}, strings.NewReader("fake-test-token\n"), io.Discard, io.Discard, nil)
	if err != nil {
		t.Fatal(err)
	}
	key, err := loadKey()
	if err != nil || key != "fake-test-token" {
		t.Fatalf("stored key lookup failed: %v", err)
	}
	path, err := keyPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("key file permissions = %o", info.Mode().Perm())
	}
	t.Setenv("GROQ_API_KEY", "override-token")
	key, err = loadKey()
	if err != nil || key != "override-token" {
		t.Fatalf("environment override failed: %v", err)
	}
}

func TestGroqRequest(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != groqEndpoint ||
			req.Header.Get("Authorization") != "Bearer fake-test-token" {
			t.Fatal("unexpected Groq request")
		}
		var body struct {
			Model    string
			Messages []struct{ Content string }
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "custom-model" || len(body.Messages) != 2 || body.Messages[1].Content != "sample diff" {
			t.Fatal("unexpected Groq request body")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"choices":[{"message":{"content":"feat(cli): add generator"}}]}`,
		))}, nil
	})}
	subject, err := generateSubject(context.Background(), client, "fake-test-token", "custom-model", "sample diff")
	if err != nil || subject != "feat(cli): add generator" {
		t.Fatalf("subject = %q, error = %v", subject, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func setupRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Chdir(t.TempDir())
	gitTest(t, "init", "--quiet")
	gitTest(t, "config", "user.name", "CLI Test")
	gitTest(t, "config", "user.email", "test@example.com")
	path := filepath.Join(".", "example.txt")
	if err := os.WriteFile(path, []byte("staged content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, "add", "example.txt")
	if err := os.WriteFile(path, []byte("unstaged content\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func gitTest(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestLargeDiffIsCapped(t *testing.T) {
	setupRepo(t)
	if err := os.WriteFile("large.txt", []byte(strings.Repeat("added line\n", 5000)), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, "add", "large.txt")
	t.Setenv("GROQ_API_KEY", "fake-test-token")
	var warnings bytes.Buffer
	err := run(context.Background(), nil, strings.NewReader("q\n"), io.Discard, &warnings,
		func(_ context.Context, key, model, diff string) (string, error) {
			const marker = "\n[Diff truncated]\n"
			if len(diff) != 48*1024+len(marker) || !strings.HasSuffix(diff, marker) {
				t.Fatalf("unexpected patch size %d or missing truncation marker", len(diff))
			}
			return "feat: add example", nil
		})
	if err != nil || !strings.Contains(warnings.String(), "48 KiB") {
		t.Fatalf("missing truncation warning or unexpected error: %q, %v", &warnings, err)
	}
}
