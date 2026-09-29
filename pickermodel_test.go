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
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
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

func twoWorkspaces() []Workspace {
	return []Workspace{
		{Name: "alpha", Dir: "~/a", source: "alpha.toml"},
		{Name: "beta", Dir: "~/b", Description: "the b one", source: "beta.toml"},
	}
}

func TestPickerEnterChooses(t *testing.T) {
	m := newPickerModel(twoWorkspaces())
	m = update(t, m, key("j"), key("enter"))
	if m.chosen == nil || m.chosen.Name != "beta" {
		t.Fatalf("chosen = %+v, want beta", m.chosen)
	}
	if m.newSession {
		t.Fatal("enter asked for a new session; it should reuse an open one")
	}
}

func TestPickerQuitKeys(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		m := newPickerModel(twoWorkspaces())
		m = update(t, m, key(k))
		if m.chosen != nil {
			t.Fatalf("%s: chosen = %+v, want nil", k, m.chosen)
		}
	}
}

func TestPickerRightArrowNamesSession(t *testing.T) {
	// Right arrow on beta opens the naming prompt prefilled with the name;
	// a typed suffix rides along as the session label, the entry unchanged.
	m := newPickerModel(twoWorkspaces())
	m = update(t, m, key("j"), key("right"))
	if m.mode != modeLabel || m.labelRef != 1 {
		t.Fatalf("right: mode=%v ref=%d, want modeLabel on beta", m.mode, m.labelRef)
	}
	if got := m.labelInput.Value(); got != "beta" {
		t.Fatalf("prompt prefill = %q, want beta", got)
	}
	m = update(t, m, key("-2"), key("enter"))
	if m.chosen == nil || m.chosen.Name != "beta" {
		t.Fatalf("chosen = %+v, want beta", m.chosen)
	}
	if m.chosenLabel != "beta-2" {
		t.Fatalf("chosenLabel = %q, want beta-2", m.chosenLabel)
	}
	if !m.newSession {
		t.Fatal("naming a session should always open a new one")
	}
}

func TestPickerLabelEscBacksOut(t *testing.T) {
	m := newPickerModel(twoWorkspaces())
	m = update(t, m, key("right"), key("esc"))
	if m.mode != modeList || m.chosen != nil {
		t.Fatalf("esc from naming: mode=%v chosen=%+v", m.mode, m.chosen)
	}
}

func TestPickerLabelClearedFallsBack(t *testing.T) {
	// Clearing the prompt entirely opens with an empty override, so open
	// falls back to the entry's own name.
	m := newPickerModel(twoWorkspaces())
	m = update(t, m, key("right"),
		key("backspace"), key("backspace"), key("backspace"), key("backspace"), key("backspace"),
		key("enter"))
	if m.chosen == nil || m.chosen.Name != "alpha" {
		t.Fatalf("chosen = %+v, want alpha", m.chosen)
	}
	if m.chosenLabel != "" {
		t.Fatalf("chosenLabel = %q, want empty (fallback to name)", m.chosenLabel)
	}
}

func TestPickerNavKeysDoNotFilter(t *testing.T) {
	m := newPickerModel(twoWorkspaces())
	m = update(t, m, key("e"), key("esc"), key("a"), key("esc"))
	if got := m.list.input.Value(); got != "" {
		t.Fatalf("action keys leaked into the query: %q", got)
	}
}

func TestPickerFilterThenChoose(t *testing.T) {
	m := newPickerModel(twoWorkspaces())
	m = update(t, m, key("/"), key("bet"), key("enter"))
	if m.chosen == nil || m.chosen.Name != "beta" {
		t.Fatalf("chosen = %+v, want beta", m.chosen)
	}
}

func TestPickerFilterEscKeepsQueryThenEdit(t *testing.T) {
	m := newPickerModel(twoWorkspaces())
	m = update(t, m, key("/"), key("bet"), key("esc"))
	if m.filtering {
		t.Fatal("esc should leave filter mode")
	}
	if m.list.input.Value() != "bet" {
		t.Fatalf("query = %q, want kept", m.list.input.Value())
	}
	m = update(t, m, key("e"))
	if m.mode != modeForm || m.form.source != "beta.toml" {
		t.Fatalf("e after filtering: mode=%v source=%q", m.mode, m.form.source)
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
	m = update(t, m, key("a"))
	if m.mode != modeForm {
		t.Fatal("a should enter the add form")
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
	m = update(t, m, key("a"), key(target), key("ctrl+s"))
	if m.mode != modeForm || m.form.errMsg == "" {
		t.Fatalf("duplicate add: mode = %v, err = %q", m.mode, m.form.errMsg)
	}
	m = update(t, m, key("esc"))
	if m.mode != modeList {
		t.Fatal("esc should leave the form")
	}
}

func TestPickerEditFlow(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()
	if _, err := addWorkspace(Workspace{Name: "thing", Dir: target}); err != nil {
		t.Fatal(err)
	}
	workspaces, err := loadWorkspaces()
	if err != nil {
		t.Fatal(err)
	}

	m := newPickerModel(workspaces)
	m = update(t, m, key("e"))
	if m.mode != modeForm || m.form.source != "thing.toml" {
		t.Fatalf("edit form: mode=%v source=%q", m.mode, m.form.source)
	}
	if got := m.form.inputs[fieldDir].Value(); got != target {
		t.Fatalf("dir not prefilled: %q", got)
	}

	// Move to the command field and set it.
	m = update(t, m, key("tab"), key("tab"), key("tab"), key("tab"), key("claude"), key("enter"))
	if m.mode != modeList {
		t.Fatalf("after save mode = %v, err = %q", m.mode, m.form.errMsg)
	}
	if len(m.workspaces) != 1 || m.workspaces[0].Command != "claude" {
		t.Fatalf("edited entry = %+v", m.workspaces)
	}
}

func TestPickerDeleteFlow(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()
	if _, err := addWorkspace(Workspace{Name: "gone", Dir: target}); err != nil {
		t.Fatal(err)
	}
	workspaces, err := loadWorkspaces()
	if err != nil {
		t.Fatal(err)
	}

	// Any key but y cancels.
	m := newPickerModel(workspaces)
	m = update(t, m, key("d"), key("n"))
	if m.mode != modeList || len(m.workspaces) != 1 {
		t.Fatalf("cancelled delete: mode=%v n=%d", m.mode, len(m.workspaces))
	}

	m = update(t, m, key("d"), key("y"))
	if m.mode != modeList || len(m.workspaces) != 0 {
		t.Fatalf("confirmed delete: mode=%v n=%d", m.mode, len(m.workspaces))
	}
	if ws, _ := loadWorkspaces(); len(ws) != 0 {
		t.Fatalf("file not removed: %+v", ws)
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
