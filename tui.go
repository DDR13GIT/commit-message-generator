package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

type keyScreen int

const (
	keysList keyScreen = iota
	chooseProvider
	enterName
	enterToken
	confirmDelete
)

type keysModel struct {
	path     string
	store    keyStore
	screen   keyScreen
	cursor   int
	provider int
	name     string
	token    string
	status   string
}

func runKeyUI(ctx context.Context) error {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return errors.New("cmsg keys needs a terminal. Use cmsg keys list, add, use, or delete instead")
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	store, err := readKeys(path)
	if err != nil {
		return err
	}
	model := &keysModel{path: path, store: store, cursor: max(0, store.index(store.Active))}
	_, err = tea.NewProgram(model, tea.WithContext(ctx), tea.WithAltScreen()).Run()
	return err
}

func (m *keysModel) Init() tea.Cmd { return nil }

func (m *keysModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if key.Type == tea.KeyEsc {
		if m.screen == keysList {
			return m, tea.Quit
		}
		m.screen, m.status, m.name, m.token = keysList, "", "", ""
		return m, nil
	}

	switch m.screen {
	case keysList:
		switch key.String() {
		case "q":
			return m, tea.Quit
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(m.cursor+1, max(0, len(m.store.Keys)-1))
		case "a":
			m.screen, m.status, m.provider = chooseProvider, "", 0
		case "enter":
			if len(m.store.Keys) > 0 {
				name := m.store.Keys[m.cursor].Name
				m.save("Selected "+name+".", func(s *keyStore) error { return s.use(name) })
			}
		case "d":
			if len(m.store.Keys) > 0 {
				m.name, m.screen, m.status = m.store.Keys[m.cursor].Name, confirmDelete, ""
			}
		}
	case chooseProvider:
		switch key.String() {
		case "up", "k", "left":
			m.provider = max(0, m.provider-1)
		case "down", "j", "right":
			m.provider = min(m.provider+1, len(providers)-1)
		case "enter":
			m.screen = enterName
		}
	case enterName:
		if key.Type == tea.KeyEnter {
			if !keyName.MatchString(m.name) {
				m.status = "Enter a name of up to 40 letters, numbers, dots, underscores or hyphens."
			} else if m.store.index(m.name) >= 0 {
				m.status = "That name is already in use."
			} else {
				m.screen, m.status = enterToken, ""
			}
		} else {
			m.name = appendInput(m.name, key, 40)
		}
	case enterToken:
		if key.Type == tea.KeyEnter {
			p, name := providers[m.provider], m.name
			if m.save("Saved "+name+".", func(s *keyStore) error { return s.add(p, name, m.token, false) }) {
				m.cursor = m.store.index(name)
			}
		} else {
			m.token = appendInput(m.token, key, 4096)
		}
	case confirmDelete:
		switch key.String() {
		case "y":
			name := m.name
			m.save("Deleted "+name+".", func(s *keyStore) error { return s.delete(name) })
		case "n":
			m.screen, m.name = keysList, ""
		}
	}
	return m, nil
}

func (m *keysModel) save(status string, change func(*keyStore) error) bool {
	store, err := updateKeys(m.path, change)
	if err != nil {
		m.status = err.Error()
		return false
	}
	m.store, m.screen, m.status, m.name, m.token = store, keysList, status, "", ""
	m.cursor = min(m.cursor, max(0, len(store.Keys)-1))
	return true
}

func appendInput(value string, key tea.KeyMsg, limit int) string {
	switch key.Type {
	case tea.KeyBackspace:
		if len(value) != 0 {
			return value[:len(value)-1]
		}
	case tea.KeyRunes:
		for _, r := range key.Runes {
			if len(value) == limit {
				break
			}
			if r >= '!' && r <= '~' {
				value += string(r)
			}
		}
	}
	return value
}

func (m *keysModel) View() string {
	var out strings.Builder
	fmt.Fprintln(&out, "cmsg API keys")
	fmt.Fprintln(&out)
	switch m.screen {
	case keysList:
		if len(m.store.Keys) == 0 {
			fmt.Fprintln(&out, "No saved keys.")
		}
		for i, k := range m.store.Keys {
			cursor, active := " ", ""
			if i == m.cursor {
				cursor = ">"
			}
			if k.Name == m.store.Active {
				active = " [active]"
			}
			fmt.Fprintf(&out, "%s %s (%s)%s\n", cursor, k.Name, k.Provider, active)
		}
		fmt.Fprintln(&out, "\n↑/↓ move  Enter activate  a add  d delete  q quit")
	case chooseProvider:
		fmt.Fprintln(&out, "Choose a provider")
		for i, p := range providers {
			cursor := " "
			if i == m.provider {
				cursor = ">"
			}
			fmt.Fprintf(&out, "%s %s\n", cursor, p.label)
		}
		fmt.Fprintln(&out, "\n↑/↓ move  Enter continue  Esc cancel")
	case enterName:
		fmt.Fprintf(&out, "Name  %s_\n\nEnter continue  Esc cancel\n", m.name)
	case enterToken:
		fmt.Fprintf(&out, "%s API key  %s_\n\nEnter save  Esc cancel\n", providers[m.provider].label, strings.Repeat("*", min(24, len(m.token))))
	case confirmDelete:
		fmt.Fprintf(&out, "Delete %s from this device?\n\ny delete  n cancel\n", m.name)
	}
	if m.status != "" {
		fmt.Fprintf(&out, "\n%s\n", m.status)
	}
	return out.String()
}
