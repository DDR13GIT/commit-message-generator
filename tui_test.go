package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyManagerAddSelectDelete(t *testing.T) {
	isolateKeys(t)
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	model := &keysModel{path: path}
	press := func(kind tea.KeyType) { model.Update(tea.KeyMsg{Type: kind}) }
	typeText := func(s string) { model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}) }

	typeText("a")
	press(tea.KeyDown)
	press(tea.KeyEnter)
	typeText("router")
	press(tea.KeyEnter)
	typeText("fake-router-key")
	if !strings.Contains(model.View(), "***************") || strings.Contains(model.View(), "fake-router-key") {
		t.Fatal("token field is not masked")
	}
	press(tea.KeyEnter)
	if model.store.Active != "router" || model.store.Keys[0].Provider != "openrouter" {
		t.Fatal("OpenRouter key was not saved and activated")
	}

	typeText("a")
	press(tea.KeyEnter)
	typeText("groq")
	press(tea.KeyEnter)
	typeText("fake-groq-key")
	press(tea.KeyEnter)
	if model.store.Active != "router" {
		t.Fatal("adding a second key silently changed the selection")
	}
	press(tea.KeyEnter)
	if model.store.Active != "groq" {
		t.Fatal("could not activate the highlighted key")
	}
	typeText("d")
	typeText("n")
	if len(model.store.Keys) != 2 {
		t.Fatal("cancel deleted a key")
	}
	typeText("d")
	typeText("y")
	if len(model.store.Keys) != 1 || model.store.Active != "" {
		t.Fatal("deleting the active key did not clear the selection")
	}
}
