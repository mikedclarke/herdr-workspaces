package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.Msg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func update(t *testing.T, m pickerModel, msgs ...tea.Msg) pickerModel {
	t.Helper()
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		var ok bool
		m, ok = next.(pickerModel)
		if !ok {
			t.Fatalf("Update returned %T, want pickerModel", next)
		}
	}
	return m
}

func TestPickerEnterChooses(t *testing.T) {
	m := newPickerModel([]Workspace{
		{Name: "alpha", Dir: "~/a"},
		{Name: "beta", Dir: "~/b"},
	})
	m = update(t, m, key("down"), key("enter"))
	if m.chosen == nil || m.chosen.Name != "beta" {
		t.Fatalf("chosen = %+v, want beta", m.chosen)
	}
}

func TestPickerEscCancels(t *testing.T) {
	m := newPickerModel([]Workspace{{Name: "alpha", Dir: "~/a"}})
	m = update(t, m, key("esc"))
	if m.chosen != nil {
		t.Fatalf("chosen = %+v, want nil after esc", m.chosen)
	}
}

func TestPickerFilterThenChoose(t *testing.T) {
	m := newPickerModel([]Workspace{
		{Name: "alpha", Dir: "~/a"},
		{Name: "beta", Dir: "~/b", Description: "the b one"},
	})
	m = update(t, m, key("bet"), key("enter"))
	if m.chosen == nil || m.chosen.Name != "beta" {
		t.Fatalf("chosen = %+v, want beta", m.chosen)
	}
}

func TestPickerGroupHeadings(t *testing.T) {
	m := newPickerModel([]Workspace{
		{Name: "alpha", Dir: "~/a", Group: "Work"},
		{Name: "beta", Dir: "~/b"},
	})
	// Grouped entries first under their heading, ungrouped trail.
	if len(m.workspaces) != 2 || m.workspaces[0].Name != "alpha" || m.workspaces[1].Name != "beta" {
		t.Fatalf("display order = %+v", m.workspaces)
	}
	headings := 0
	for _, it := range m.list.items {
		if !it.selectable {
			headings++
		}
	}
	if headings != 2 {
		t.Fatalf("got %d headings, want Work and Ungrouped", headings)
	}
}

func TestPickerAddFlow(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()

	m := newPickerModel(nil)
	m = update(t, m, key("ctrl+a"))
	if m.mode != modeAdd {
		t.Fatal("ctrl+a should enter add mode")
	}

	// Type the directory, tab away (prefills the name), save.
	m = update(t, m, key(target), key("tab"), key("ctrl+s"))
	if m.mode != modeList {
		t.Fatalf("after save mode = %v, err = %q", m.mode, m.form.errMsg)
	}
	if len(m.workspaces) != 1 {
		t.Fatalf("list not refreshed after add: %+v", m.workspaces)
	}

	// A second add of the same directory collides on the slug and stays in
	// the form with the error shown.
	m = update(t, m, key("ctrl+a"), key(target), key("ctrl+s"))
	if m.mode != modeAdd || m.form.errMsg == "" {
		t.Fatalf("duplicate add: mode = %v, err = %q", m.mode, m.form.errMsg)
	}
	m = update(t, m, key("esc"))
	if m.mode != modeList {
		t.Fatal("esc should leave the add form")
	}
}

func TestAddFormPrefillsName(t *testing.T) {
	f := newAddForm()
	f.inputs[fieldDir].SetValue("~/code/myrepo/")
	f.next()
	if got := f.inputs[fieldName].Value(); got != "myrepo" {
		t.Fatalf("prefilled name = %q, want myrepo", got)
	}

	// A name the user already typed is never overwritten by the prefill.
	f = newAddForm()
	f.inputs[fieldDir].SetValue("~/code/myrepo")
	f.inputs[fieldName].SetValue("custom")
	f.next()
	if got := f.inputs[fieldName].Value(); got != "custom" {
		t.Fatalf("name = %q, want custom kept", got)
	}
}
