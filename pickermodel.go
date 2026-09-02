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
	modeForm
	modeConfirmDelete
	modeLabel
)

// pickerHeaderLines is how many lines view() renders above the fuzzy list:
// the title and the blank beneath it. Mouse handling subtracts it before
// asking the list which row was clicked.
const pickerHeaderLines = 2

// pickerModel is the picker TUI. The list is navigation-first: single keys
// act (open, add, edit, delete), and `/` focuses the fuzzy filter; leaving
// the filter with esc keeps it applied so a filtered entry can be acted on.
// When the program finishes, chosen holds the workspace to open, or nil.
type pickerModel struct {
	workspaces []Workspace // display order; list refs index into this
	list       fuzzyList
	filtering  bool
	mode       pickerMode
	form       addForm
	deleteRef  int // pending delete, index into workspaces
	labelInput textinput.Model
	labelRef   int // entry being named, index into workspaces
	chosen     *Workspace
	// chosenLabel overrides the herdr workspace label for this open only,
	// leaving the entry's stored name untouched. Empty means use the name.
	chosenLabel string
	width       int
	height      int
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
	m.list = newFuzzyList("/ to filter", items)
	m.list.input.Blur()
	m.filtering = false
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
	switch m.mode {
	case modeForm:
		return m.updateForm(msg)
	case modeConfirmDelete:
		return m.updateConfirmDelete(msg)
	case modeLabel:
		return m.updateLabel(msg)
	}
	return m.updateList(msg)
}

func (m pickerModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// With no workspaces the screen is an onboarding card: add still
		// works, any exit key closes.
		if len(m.workspaces) == 0 {
			switch msg.String() {
			case "a", "ctrl+a":
				return m.enterAdd()
			case "ctrl+c", "esc", "q", "enter":
				return m, tea.Quit
			}
			return m, nil
		}
		if m.filtering {
			return m.updateListFiltering(msg)
		}

		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "up", "k", "ctrl+p":
			m.list.moveUp()
			return m, nil
		case "down", "j", "ctrl+n":
			m.list.moveDown()
			return m, nil
		case "enter":
			return m.activate()
		case "right":
			return m.enterLabel()
		case "/":
			m.filtering = true
			m.list.input.Focus()
			return m, textinput.Blink
		case "a", "ctrl+a":
			return m.enterAdd()
		case "e", "ctrl+e":
			return m.enterEdit()
		case "d":
			return m.enterConfirmDelete()
		}
		return m, nil

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

// updateListFiltering handles keys while the query box is focused. Esc leaves
// filter mode with the query still applied; enter opens the highlighted match
// directly, so `/name enter` stays one fluid motion.
func (m pickerModel) updateListFiltering(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.filtering = false
		m.list.input.Blur()
		return m, nil
	case "up", "ctrl+p":
		m.list.moveUp()
		return m, nil
	case "down", "ctrl+n":
		m.list.moveDown()
		return m, nil
	case "enter":
		return m.activate()
	}
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

// enterLabel opens the one-field prompt for naming this session, prefilled
// with the entry's own name and the cursor at the end so tacking on a suffix
// (a second session in the same directory) is the quick path. With nothing
// selectable it is a no-op.
func (m pickerModel) enterLabel() (tea.Model, tea.Cmd) {
	ref := m.list.selectedRef()
	if ref < 0 {
		return m, nil
	}
	m.mode = modeLabel
	m.labelRef = ref
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "session name"
	ti.SetValue(m.workspaces[ref].Name)
	ti.CursorEnd()
	ti.Focus()
	m.labelInput = ti
	return m, textinput.Blink
}

// updateLabel handles the naming prompt: enter opens the entry with the typed
// label (blank falls back to the entry's name at open time), esc backs out.
func (m pickerModel) updateLabel(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		if _, isMouse := msg.(tea.MouseMsg); isMouse {
			return m, nil
		}
		var cmd tea.Cmd
		m.labelInput, cmd = m.labelInput.Update(msg)
		return m, cmd
	}
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeList
		return m, nil
	case "enter":
		w := m.workspaces[m.labelRef]
		m.chosen = &w
		m.chosenLabel = strings.TrimSpace(m.labelInput.Value())
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.labelInput, cmd = m.labelInput.Update(msg)
	return m, cmd
}

func (m pickerModel) enterAdd() (tea.Model, tea.Cmd) {
	m.mode = modeForm
	m.form = newAddForm()
	return m, textinput.Blink
}

// enterEdit opens the form prefilled with the highlighted workspace. With
// nothing selectable it is a no-op.
func (m pickerModel) enterEdit() (tea.Model, tea.Cmd) {
	ref := m.list.selectedRef()
	if ref < 0 {
		return m, nil
	}
	m.mode = modeForm
	m.form = newEditForm(m.workspaces[ref])
	return m, textinput.Blink
}

func (m pickerModel) enterConfirmDelete() (tea.Model, tea.Cmd) {
	ref := m.list.selectedRef()
	if ref < 0 {
		return m, nil
	}
	m.mode = modeConfirmDelete
	m.deleteRef = ref
	return m, nil
}

func (m pickerModel) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "y":
		if err := removeWorkspaceFile(m.workspaces[m.deleteRef].source); err != nil {
			m.mode = modeList
			return m, nil
		}
		workspaces, err := loadWorkspaces()
		if err == nil {
			m.setWorkspaces(workspaces)
		}
		m.mode = modeList
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	default:
		// Anything but an explicit yes backs out.
		m.mode = modeList
		return m, nil
	}
}

func (m pickerModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		return m.saveForm()
	case "ctrl+s":
		return m.saveForm()
	}
	cmd := m.form.route(msg)
	return m, cmd
}

// saveForm validates the form and writes the workspace file (a new entry, or
// the edited entry's own file), returning to a refreshed list on success and
// surfacing the error in place on failure.
func (m pickerModel) saveForm() (tea.Model, tea.Cmd) {
	var err error
	if m.form.source != "" {
		_, err = updateWorkspace(m.form.workspace(), m.form.source)
	} else {
		_, err = addWorkspace(m.form.workspace())
	}
	if err != nil {
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
	return m, nil
}

func (m pickerModel) View() string {
	var b strings.Builder
	switch m.mode {
	case modeForm:
		title := "Workspaces · add"
		if m.form.source != "" {
			title = "Workspaces · edit"
		}
		b.WriteString(titleStyle.Render(title))
		b.WriteString("\n\n")
		b.WriteString(m.form.view())
		b.WriteString("\n")
		b.WriteString(footerStyle.Render("enter next · ctrl+s save · esc back"))
	case modeConfirmDelete:
		w := m.workspaces[m.deleteRef]
		b.WriteString(titleStyle.Render("Workspaces · delete"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Delete %q? Its file %s will be removed.\n\n", w.Name, w.source))
		b.WriteString(footerStyle.Render("y delete · any other key cancels"))
	case modeLabel:
		w := m.workspaces[m.labelRef]
		b.WriteString(titleStyle.Render("Workspaces · name"))
		b.WriteString("\n\n")
		b.WriteString("  " + descStyle.Render(w.displayDir()) + "\n\n")
		b.WriteString("  ")
		b.WriteString(labelSel.Render(fmt.Sprintf("%-12s", "name")))
		b.WriteString(m.labelInput.View())
		b.WriteString("\n\n")
		b.WriteString(footerStyle.Render("enter open · esc back"))
	default:
		b.WriteString(titleStyle.Render("Workspaces"))
		b.WriteString("\n\n")
		if len(m.workspaces) == 0 {
			dir, _ := workspacesConfigDir()
			b.WriteString("  No workspaces registered yet.\n\n")
			b.WriteString(descStyle.Render("  Press a to add a directory, or drop a .toml into\n  " + dir))
			b.WriteString("\n\n")
			b.WriteString(footerStyle.Render("a add · esc close"))
		} else {
			b.WriteString(m.list.view("no match"))
			b.WriteString("\n")
			if m.filtering {
				b.WriteString(footerStyle.Render("enter open · esc done filtering"))
			} else {
				b.WriteString(footerStyle.Render("enter open · → name · / filter · a add · e edit · d delete · q close"))
			}
		}
	}
	b.WriteString("\n")
	return b.String()
}

// addForm is the in-picker form for registering or editing a directory: five
// text fields, one focused at a time. Leaving the directory field prefills
// the name with the directory's basename when the name is still empty. A
// non-empty source means the form edits that existing file instead of adding.
type addForm struct {
	inputs [5]textinput.Model
	focus  int
	errMsg string
	source string
}

const (
	fieldDir = iota
	fieldName
	fieldDescription
	fieldGroup
	fieldCommand
)

var fieldLabels = [5]string{"directory", "name", "description", "group", "command"}
var fieldHints = [5]string{"~/code/myrepo (required)", "defaults to the directory name", "optional, shown in the picker", "optional picker heading", "optional, runs on open"}

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

func newEditForm(w Workspace) addForm {
	f := newAddForm()
	f.source = w.source
	f.inputs[fieldDir].SetValue(w.Dir)
	f.inputs[fieldName].SetValue(w.Name)
	f.inputs[fieldDescription].SetValue(w.Description)
	f.inputs[fieldGroup].SetValue(w.Group)
	f.inputs[fieldCommand].SetValue(w.Command)
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
		Name:        strings.TrimSpace(f.inputs[fieldName].Value()),
		Description: strings.TrimSpace(f.inputs[fieldDescription].Value()),
		Group:       strings.TrimSpace(f.inputs[fieldGroup].Value()),
		Dir:         strings.TrimSpace(f.inputs[fieldDir].Value()),
		Command:     strings.TrimSpace(f.inputs[fieldCommand].Value()),
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
		b.WriteString(label.Render(fmt.Sprintf("%-12s", fieldLabels[i])))
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
