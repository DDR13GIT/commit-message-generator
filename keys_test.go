package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolateKeys(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("GROQ_MODEL", "")
	t.Setenv("OPENROUTER_MODEL", "")
}

func TestManageKeysAndMigrateGroq(t *testing.T) {
	setupRepo(t)
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(filepath.Dir(path), "groq-key")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy-groq-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if p, key, err := activeKey(); err != nil || p.id != "groq" || key != "legacy-groq-secret" {
		t.Fatal("legacy key was not resolved")
	}

	var out bytes.Buffer
	keys := func(input string, args ...string) {
		t.Helper()
		if err := run(context.Background(), append([]string{"keys"}, args...), strings.NewReader(input), &out, &out, nil); err != nil {
			t.Fatal(err)
		}
	}
	keys("work-router-secret\n", "add", "openrouter", "work")
	keys("", "list")
	if !strings.Contains(out.String(), "default") || !strings.Contains(out.String(), "work") ||
		strings.Contains(out.String(), "-secret") {
		t.Fatal("unexpected key list")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy file was not removed after saving the profile store")
	}
	keys("", "use", "work")
	t.Setenv("GROQ_API_KEY", "environment-groq-secret")
	t.Setenv("OPENROUTER_MODEL", "custom/model")
	err = run(context.Background(), nil, strings.NewReader("q\n"), io.Discard, io.Discard,
		func(_ context.Context, p provider, key, model, diff string) (string, error) {
			if p.id != "openrouter" || key != "work-router-secret" || model != "custom/model" {
				t.Fatal("draft did not use selected OpenRouter profile")
			}
			return "feat: add example", nil
		})
	if err != nil {
		t.Fatal(err)
	}
	keys("", "delete", "work")
	t.Setenv("GROQ_API_KEY", "")
	if _, _, err := activeKey(); err == nil {
		t.Fatal("deleting the active key should require a new selection")
	}
	keys("", "use", "default")
	if p, _, err := activeKey(); err != nil || p.id != "groq" {
		t.Fatal("could not reselect the migrated Groq key")
	}
}

func TestOpenRouterRequest(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://openrouter.ai/api/v1/chat/completions" ||
			req.Header.Get("Authorization") != "Bearer fake-router-token" {
			t.Fatal("wrong OpenRouter endpoint or key")
		}
		var body struct {
			Model               string `json:"model"`
			MaxTokens           int    `json:"max_tokens"`
			MaxCompletionTokens int    `json:"max_completion_tokens"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "openrouter/auto" || body.MaxTokens == 0 || body.MaxCompletionTokens != 0 {
			t.Fatal("incorrect OpenRouter request parameters")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"choices":[{"message":{"content":"feat: add router"}}]}`,
		))}, nil
	})}
	if _, err := generateSubject(context.Background(), client, openRouter, "fake-router-token", openRouter.defaultModel, "diff"); err != nil {
		t.Fatal(err)
	}
}
