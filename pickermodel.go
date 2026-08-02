package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type pickerMode int

const (
	modeList pickerMode = iota
	modeAdd
)

// pickerHeaderLines is how many lines view() renders above the fuzzy list:
// the title and the blank beneath it. Mouse handling subtracts it before
// asking the list which row was clicked.
const pickerHeaderLines = 2

// pickerModel is the picker TUI: a fuzzy list of registered workspaces, plus
// an add form (ctrl+a) for registering a new directory in place. When the
// program finishes, chosen holds the workspace to open, or nil on cancel.
type pickerModel struct {
	workspaces []Workspace // display order; list refs index into this
	list       fuzzyList
	mode       pickerMode
	form       addForm
	chosen     *Workspace
	width      int
	height     int
}

func newPickerModel(workspaces []Workspace) pickerModel {
	m := pickerModel{}
	m.setWorkspaces(workspaces)
	return m
}

// setWorkspaces (re)builds the list rows: workspaces clustered under group
// headings when any entry has a group, a plain flat list otherwise. Groups
// appear in name order with the catch-all "Ungrouped" last.
func (m *pickerModel) setWorkspaces(workspaces []Workspace) {
	var ordered []Workspace
	var items []listItem

	byGroup := map[string][]Workspace{}
	for _, w := range workspaces {
		byGroup[w.Group] = append(byGroup[w.Group], w)
	}
	var groups []string
	for g := range byGroup {
		if g != "" {
			groups = append(groups, g)
		}
	}
	sort.Strings(groups)

	appendGroup := func(heading string, ws []Workspace) {
		if heading != "" {
			items = append(items, listItem{name: heading})
		}
		for _, w := range ws {
			items = append(items, listItem{name: w.Name, desc: w.Description, selectable: true, ref: len(ordered)})
			ordered = append(ordered, w)
		}
	}

	if len(groups) == 0 {
		appendGroup("", workspaces)
	} else {
		for _, g := range groups {
			appendGroup(g, byGroup[g])
		}
		if ws := byGroup[""]; len(ws) > 0 {
			appendGroup("Ungrouped", ws)
		}
	}

	m.workspaces = ordered
	m.list = newFuzzyList("type to filter…", items)
}

func (m pickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wm, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wm.Width
		m.height = wm.Height
		return m, nil
	}
	if m.mode == modeAdd {
		return m.updateAdd(msg)
	}
	return m.updateList(msg)
}

func (m pickerModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// With no workspaces the screen is an onboarding card: ctrl+a still
		// opens the add form, any exit key closes.
		if len(m.workspaces) == 0 {
			switch msg.String() {
			case "ctrl+a":
				return m.enterAdd()
			case "ctrl+c", "esc", "q", "enter":
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "ctrl+p":
			m.list.moveUp()
			return m, nil
		case "down", "ctrl+n":
			m.list.moveDown()
			return m, nil
		case "enter":
			return m.activate()
		case "ctrl+a":
			return m.enterAdd()
		}
		cmd := m.list.editQuery(msg)
		return m, cmd

	case tea.MouseMsg:
		if len(m.workspaces) == 0 {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.list.moveUp()
		case tea.MouseButtonWheelDown:
			m.list.moveDown()
		case tea.MouseButtonLeft:
			if m.list.clickRow(msg.Y-pickerHeaderLines) && msg.Action == tea.MouseActionRelease {
				return m.activate()
			}
		}
		return m, nil
	}

	// Non-key messages (the blink tick) keep the query input alive.
	cmd := m.list.editQuery(msg)
	return m, cmd
}

// activate records the highlighted workspace as chosen and quits so the
// caller opens it. Activating with nothing selectable is a no-op.
func (m pickerModel) activate() (tea.Model, tea.Cmd) {
	ref := m.list.selectedRef()
	if ref < 0 {
		return m, nil
	}
	w := m.workspaces[ref]
	m.chosen = &w
	return m, tea.Quit
}

func (m pickerModel) enterAdd() (tea.Model, tea.Cmd) {
	m.mode = modeAdd
	m.form = newAddForm()
	return m, textinput.Blink
}

func (m pickerModel) updateAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		if _, isMouse := msg.(tea.MouseMsg); isMouse {
			return m, nil
		}
		cmd := m.form.route(msg)
		return m, cmd
	}

	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeList
		return m, nil
	case "tab", "down":
		m.form.next()
		return m, nil
	case "shift+tab", "up":
		m.form.prev()
		return m, nil
	case "enter":
		if !m.form.atLast() {
			m.form.next()
			return m, nil
		}
		return m.saveAdd()
	case "ctrl+s":
		return m.saveAdd()
	}
	cmd := m.form.route(msg)
	return m, cmd
}

// saveAdd validates the form and writes the new workspace file, returning to
// a refreshed list on success and surfacing the error in place on failure.
func (m pickerModel) saveAdd() (tea.Model, tea.Cmd) {
	if _, err := addWorkspace(m.form.workspace()); err != nil {
		m.form.errMsg = err.Error()
		return m, nil
	}
	workspaces, err := loadWorkspaces()
	if err != nil {
		m.form.errMsg = err.Error()
		return m, nil
	}
	m.setWorkspaces(workspaces)
	m.mode = modeList
	return m, textinput.Blink
}

func (m pickerModel) View() string {
	var b strings.Builder
	switch m.mode {
	case modeAdd:
		b.WriteString(titleStyle.Render("Workspaces · add"))
		b.WriteString("\n\n")
		b.WriteString(m.form.view())
		b.WriteString("\n")
		b.WriteString(footerStyle.Render("enter next · ctrl+s save · esc back"))
	default:
		b.WriteString(titleStyle.Render("Workspaces"))
		b.WriteString("\n\n")
		if len(m.workspaces) == 0 {
			dir, _ := workspacesConfigDir()
			b.WriteString("  No workspaces registered yet.\n\n")
			b.WriteString(descStyle.Render("  Press ctrl+a to add a directory, or drop a .toml into\n  " + dir))
			b.WriteString("\n\n")
			b.WriteString(footerStyle.Render("ctrl+a add · esc close"))
		} else {
			b.WriteString(m.list.view("no match"))
			b.WriteString("\n")
			b.WriteString(footerStyle.Render("enter open · ctrl+a add · esc close"))
		}
	}
	b.WriteString("\n")
	return b.String()
}

// addForm is the in-picker form for registering a directory: four text
// fields, one focused at a time. Leaving the directory field prefills the
// name with the directory's basename when the name is still empty.
type addForm struct {
	inputs [4]textinput.Model
	focus  int
	errMsg string
}

const (
	fieldDir = iota
	fieldName
	fieldGroup
	fieldCommand
)

var fieldLabels = [4]string{"directory", "name", "group", "command"}
var fieldHints = [4]string{"~/code/myrepo (required)", "defaults to the directory name", "optional picker heading", "optional, runs on open"}

func newAddForm() addForm {
	var f addForm
	for i := range f.inputs {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = fieldHints[i]
		f.inputs[i] = ti
	}
	f.inputs[fieldDir].Focus()
	return f
}

func (f *addForm) atLast() bool { return f.focus == len(f.inputs)-1 }

func (f *addForm) next() { f.setFocus(f.focus + 1) }
func (f *addForm) prev() { f.setFocus(f.focus - 1) }

func (f *addForm) setFocus(idx int) {
	if idx < 0 || idx >= len(f.inputs) {
		return
	}
	if f.focus == fieldDir && idx != fieldDir && strings.TrimSpace(f.inputs[fieldName].Value()) == "" {
		if dir := strings.TrimSpace(f.inputs[fieldDir].Value()); dir != "" {
			f.inputs[fieldName].SetValue(filepath.Base(strings.TrimRight(dir, "/")))
		}
	}
	f.inputs[f.focus].Blur()
	f.focus = idx
	f.inputs[f.focus].Focus()
}

// route feeds a message to the focused input.
func (f *addForm) route(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return cmd
}

func (f *addForm) workspace() Workspace {
	return Workspace{
		Name:    strings.TrimSpace(f.inputs[fieldName].Value()),
		Group:   strings.TrimSpace(f.inputs[fieldGroup].Value()),
		Dir:     strings.TrimSpace(f.inputs[fieldDir].Value()),
		Command: strings.TrimSpace(f.inputs[fieldCommand].Value()),
	}
}

func (f addForm) view() string {
	var b strings.Builder
	for i, in := range f.inputs {
		label := labelStyle
		if i == f.focus {
			label = labelSel
		}
		b.WriteString("  ")
		b.WriteString(label.Render(fmt.Sprintf("%-10s", fieldLabels[i])))
		b.WriteString(in.View())
		b.WriteString("\n")
	}
	if f.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("  " + f.errMsg))
		b.WriteString("\n")
	}
	return b.String()
}
