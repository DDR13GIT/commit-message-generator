package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

type savedKey struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Token    string `json:"token"`
}

type keyStore struct {
	Active string     `json:"active"`
	Keys   []savedKey `json:"keys"`
	legacy bool
}

var keyName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,39}$`)

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "git-cmsg", "keys.json"), nil
}

func readKeys(path string) (keyStore, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		legacy, err := os.ReadFile(filepath.Join(filepath.Dir(path), "groq-key"))
		if errors.Is(err, os.ErrNotExist) {
			return keyStore{}, nil
		}
		if err != nil {
			return keyStore{}, err
		}
		token, err := parseKey(string(legacy))
		if err != nil {
			return keyStore{}, err
		}
		return keyStore{Active: "default", Keys: []savedKey{{Name: "default", Provider: groq.id, Token: token}}, legacy: true}, nil
	}
	if err != nil {
		return keyStore{}, err
	}
	var store keyStore
	if err := json.Unmarshal(data, &store); err != nil {
		return keyStore{}, errors.New("invalid cmsg key configuration")
	}
	for _, k := range store.Keys {
		if _, ok := lookupProvider(k.Provider); !ok || !keyName.MatchString(k.Name) {
			return keyStore{}, errors.New("invalid provider or name in cmsg key configuration")
		}
		if token, err := parseKey(k.Token); err != nil || token != k.Token {
			return keyStore{}, errors.New("invalid token in cmsg key configuration")
		}
	}
	return store, nil
}

func activeKey() (provider, string, error) {
	path, err := configPath()
	if err != nil {
		return provider{}, "", err
	}
	store, err := readKeys(path)
	if err != nil {
		return provider{}, "", err
	}
	if i := store.index(store.Active); i >= 0 {
		selected := store.Keys[i]
		p, _ := lookupProvider(selected.Provider)
		if store.legacy && os.Getenv(p.keyEnv) != "" {
			token, err := parseKey(os.Getenv(p.keyEnv))
			return p, token, err
		}
		return p, selected.Token, nil
	}
	for _, p := range providers {
		if value := os.Getenv(p.keyEnv); value != "" {
			token, err := parseKey(value)
			return p, token, err
		}
	}
	return provider{}, "", errors.New("no active API key. Run cmsg keys to add or select one")
}

func (s *keyStore) index(name string) int {
	for i, k := range s.Keys {
		if k.Name == name {
			return i
		}
	}
	return -1
}

func (s *keyStore) add(p provider, name, value string, replace bool) error {
	if !keyName.MatchString(name) {
		return errors.New("use 1–40 letters, numbers, dots, underscores or hyphens for the name, starting with a letter or number")
	}
	token, err := parseKey(value)
	if err != nil {
		return err
	}
	k := savedKey{Name: name, Provider: p.id, Token: token}
	if i := s.index(name); i >= 0 {
		if !replace || s.Keys[i].Provider != p.id {
			return fmt.Errorf("a key named %q already exists", name)
		}
		s.Keys[i] = k
	} else {
		s.Keys = append(s.Keys, k)
	}
	if replace || len(s.Keys) == 1 {
		s.Active = name
	}
	return nil
}

func (s *keyStore) use(name string) error {
	if s.index(name) < 0 {
		return errors.New("key name not found")
	}
	s.Active = name
	return nil
}

func (s *keyStore) delete(name string) error {
	i := s.index(name)
	if i < 0 {
		return errors.New("key name not found")
	}
	s.Keys = append(s.Keys[:i], s.Keys[i+1:]...)
	if s.Active == name {
		s.Active = ""
	}
	return nil
}

func updateKeys(path string, change func(*keyStore) error) (keyStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return keyStore{}, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "keys.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return keyStore{}, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return keyStore{}, err
	}

	store, err := readKeys(path)
	if err != nil {
		return keyStore{}, err
	}
	if err := change(&store); err != nil {
		return keyStore{}, err
	}
	file, err := os.CreateTemp(dir, ".keys-*")
	if err != nil {
		return keyStore{}, err
	}
	defer os.Remove(file.Name())
	writeErr := json.NewEncoder(file).Encode(store)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return keyStore{}, err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return keyStore{}, err
	}
	// The new file takes precedence as soon as it is published. Remove the legacy
	// copy only afterward so a failed save cannot lose the existing credential.
	if err := os.Remove(filepath.Join(dir, "groq-key")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return store, fmt.Errorf("keys saved, but could not remove the old Groq key file: %w", err)
	}
	store.legacy = false
	return store, nil
}

func parseKey(value string) (string, error) {
	key := strings.TrimSpace(value)
	if key == "" || len(key) > 4096 || strings.ContainsFunc(key, func(r rune) bool {
		return r < '!' || r > '~'
	}) {
		return "", errors.New("expected a single API key")
	}
	return key, nil
}

func keyInput(p provider, in io.Reader) (string, error) {
	if file, ok := in.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			if value := os.Getenv(p.keyEnv); value != "" {
				return parseKey(value)
			}
			return "", fmt.Errorf("set %s, pipe the key on standard input, or run cmsg keys for a hidden prompt", p.keyEnv)
		}
	}
	data, err := io.ReadAll(io.LimitReader(in, 4098))
	if err != nil {
		return "", err
	}
	if len(data) == 4098 {
		return "", errors.New("API key input is too long")
	}
	if len(data) == 0 {
		return parseKey(os.Getenv(p.keyEnv))
	}
	return parseKey(string(data))
}

func login(args []string, in io.Reader, out io.Writer) error {
	p, name := groq, "default"
	if len(args) > 1 {
		return errors.New("usage: cmsg login [groq|openrouter]")
	}
	if len(args) == 1 {
		var ok bool
		p, ok = lookupProvider(args[0])
		if !ok {
			return errors.New("usage: cmsg login [groq|openrouter]")
		}
		if p.id != groq.id {
			name = p.id
		}
	}
	value := os.Getenv(p.keyEnv)
	if value == "" {
		var err error
		value, err = keyInput(p, in)
		if err != nil {
			return err
		}
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	if _, err := updateKeys(path, func(s *keyStore) error {
		return s.add(p, name, value, true)
	}); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s API key saved to %s\n", p.label, path)
	return nil
}

func keysCommand(args []string, in io.Reader, out io.Writer) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	switch {
	case len(args) == 1 && args[0] == "list":
		store, err := readKeys(path)
		if err != nil {
			return err
		}
		if len(store.Keys) == 0 {
			fmt.Fprintln(out, "No saved keys.")
		}
		for _, k := range store.Keys {
			marker := " "
			if k.Name == store.Active {
				marker = "*"
			}
			fmt.Fprintf(out, "%s %s (%s)\n", marker, k.Name, k.Provider)
		}
		return nil
	case len(args) == 3 && args[0] == "add":
		p, ok := lookupProvider(args[1])
		if !ok {
			return errors.New("choose groq or openrouter")
		}
		value, err := keyInput(p, in)
		if err != nil {
			return err
		}
		_, err = updateKeys(path, func(s *keyStore) error { return s.add(p, args[2], value, false) })
		if err == nil {
			fmt.Fprintf(out, "Saved %s (%s).\n", args[2], p.id)
		}
		return err
	case len(args) == 2 && args[0] == "use":
		_, err := updateKeys(path, func(s *keyStore) error { return s.use(args[1]) })
		if err == nil {
			fmt.Fprintf(out, "Selected %s.\n", args[1])
		}
		return err
	case len(args) == 2 && args[0] == "delete":
		_, err := updateKeys(path, func(s *keyStore) error { return s.delete(args[1]) })
		if err == nil {
			fmt.Fprintf(out, "Deleted %s.\n", args[1])
		}
		return err
	}
	return errors.New("see cmsg -h for key commands")
}
