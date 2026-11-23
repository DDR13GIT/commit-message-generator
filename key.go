package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func keyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	// Keep the original directory so existing logins still work after the rename.
	return filepath.Join(dir, "git-cmsg", "groq-key"), nil
}

func loadKey() (string, error) {
	value := os.Getenv("GROQ_API_KEY")
	if value == "" {
		path, err := keyPath()
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("no Groq API key. Set GROQ_API_KEY or save it with cmsg login")
		}
		if err != nil {
			return "", err
		}
		value = string(data)
	}
	return parseKey(value)
}

func login(in io.Reader, out io.Writer) error {
	value := os.Getenv("GROQ_API_KEY")
	if value == "" {
		if file, ok := in.(*os.File); ok {
			info, err := file.Stat()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeCharDevice != 0 {
				return errors.New("set GROQ_API_KEY or pipe the key into cmsg login. See the README for a hidden-input example")
			}
		}
		data, err := io.ReadAll(io.LimitReader(in, 4097))
		if err != nil {
			return err
		}
		if len(data) > 4096 {
			return errors.New("Groq key input is too long")
		}
		value = string(data)
	}
	key, err := parseKey(value)
	if err != nil {
		return err
	}

	path, err := keyPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".groq-key-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := io.WriteString(file, key+"\n")
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	fmt.Fprintf(out, "Groq API key saved to %s\n", path)
	return nil
}

func parseKey(value string) (string, error) {
	key := strings.TrimSpace(value)
	if key == "" || strings.ContainsFunc(key, func(r rune) bool {
		return r < '!' || r > '~'
	}) {
		return "", errors.New("expected a single Groq API key")
	}
	return key, nil
}
