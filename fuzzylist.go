package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

// listItem is one row in a fuzzyList. A selectable row shows a name (with
// fuzzy matches highlighted) plus an optional dim description, and carries ref
// (the caller's index for the row). A row with selectable=false is a group
// heading: skipped during navigation and hidden while filtering.
type listItem struct {
	name       string
	desc       string
	selectable bool
	ref        int
}

// scoredItem is a listItem that survived the current query, with the
// name-character positions that matched, for highlighting.
type scoredItem struct {
	item    listItem
	matched []int
}

// fuzzyList is a fuzzy-filtered, keyboard- and mouse-navigable list with a
// query box.
type fuzzyList struct {
	input     textinput.Model
	items     []listItem
	filtered  []scoredItem
	cursor    int
	lastQuery string
}

func newFuzzyList(placeholder string, items []listItem) fuzzyList {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = ""
	ti.Focus()

	l := fuzzyList{input: ti, items: items}
	l.filter()
	return l
}

// filter recomputes the visible rows from the current query. An empty query
// shows every item, headings included, in natural order. A non-empty query
// fuzzy-matches the selectable items against name and description together,
// highlighting only the matches that land inside the name.
func (l *fuzzyList) filter() {
	q := strings.TrimSpace(l.input.Value())
	if q != l.lastQuery {
		l.cursor = 0
		l.lastQuery = q
	}
	l.filtered = l.filtered[:0]

	if q == "" {
		for _, it := range l.items {
			l.filtered = append(l.filtered, scoredItem{item: it})
		}
		l.clampCursor()
		return
	}

	var sel []listItem
	for _, it := range l.items {
		if it.selectable {
			sel = append(sel, it)
		}
	}
	haystacks := make([]string, len(sel))
	for i, it := range sel {
		haystacks[i] = it.name + "  " + it.desc
	}
	for _, mt := range fuzzy.Find(q, haystacks) {
		var inName []int
		for _, idx := range mt.MatchedIndexes {
			if idx < len(sel[mt.Index].name) {
				inName = append(inName, idx)
			}
		}
		l.filtered = append(l.filtered, scoredItem{item: sel[mt.Index], matched: inName})
	}
	l.clampCursor()
}

// clampCursor keeps the cursor in range and parked on a selectable row,
// searching down then up when it lands on a heading.
func (l *fuzzyList) clampCursor() {
	if len(l.filtered) == 0 {
		l.cursor = 0
		return
	}
	if l.cursor >= len(l.filtered) {
		l.cursor = len(l.filtered) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.filtered[l.cursor].item.selectable {
		return
	}
	for i := l.cursor; i < len(l.filtered); i++ {
		if l.filtered[i].item.selectable {
			l.cursor = i
			return
		}
	}
	for i := l.cursor; i >= 0; i-- {
		if l.filtered[i].item.selectable {
			l.cursor = i
			return
		}
	}
}

func (l *fuzzyList) moveUp() {
	for i := l.cursor - 1; i >= 0; i-- {
		if l.filtered[i].item.selectable {
			l.cursor = i
			return
		}
	}
}

func (l *fuzzyList) moveDown() {
	for i := l.cursor + 1; i < len(l.filtered); i++ {
		if l.filtered[i].item.selectable {
			l.cursor = i
			return
		}
	}
}

// selectedRef returns the ref of the highlighted row, or -1 when nothing is
// selectable (empty list, or every match filtered away).
func (l *fuzzyList) selectedRef() int {
	if len(l.filtered) == 0 {
		return -1
	}
	it := l.filtered[l.cursor].item
	if !it.selectable {
		return -1
	}
	return it.ref
}

// listPromptLines is how many lines view() renders before the first result
// row: the query line and the blank beneath it. rowIndexAt and view() share
// this accounting and must change together.
const listPromptLines = 2

// rowIndexAt maps a view-local line (0 is the query line) to the index into
// filtered of the selectable row drawn there, or -1 for the prompt, blanks,
// headings, or past the end.
func (l *fuzzyList) rowIndexAt(y int) int {
	line := listPromptLines
	for i, s := range l.filtered {
		if !s.item.selectable {
			line += 2 // a heading renders as a blank line plus its label
			continue
		}
		if line == y {
			return i
		}
		line++
	}
	return -1
}

// clickRow moves the highlight to the selectable row at view-local line y,
// reporting whether y landed on one.
func (l *fuzzyList) clickRow(y int) bool {
	idx := l.rowIndexAt(y)
	if idx < 0 {
		return false
	}
	l.cursor = idx
	return true
}

// editQuery feeds a message to the query box and re-filters. Non-key messages
// (the cursor blink tick) pass through harmlessly.
func (l *fuzzyList) editQuery(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	l.input, cmd = l.input.Update(msg)
	l.filter()
	return cmd
}

// view renders the query line, the match count, and the result rows. Headings
// render as a blank line plus a dim label; emptyMsg shows when no row matches.
func (l fuzzyList) view(emptyMsg string) string {
	var b strings.Builder

	matched, total := 0, 0
	for _, it := range l.items {
		if it.selectable {
			total++
		}
	}
	for _, s := range l.filtered {
		if s.item.selectable {
			matched++
		}
	}

	b.WriteString(promptStyle.Render("❯ "))
	b.WriteString(l.input.View())
	b.WriteString("   ")
	b.WriteString(countStyle.Render(fmt.Sprintf("%d/%d", matched, total)))
	b.WriteString("\n\n")

	if matched == 0 {
		b.WriteString(descStyle.Render("  " + emptyMsg))
		b.WriteString("\n")
	}
	for i, s := range l.filtered {
		it := s.item
		if !it.selectable {
			b.WriteString("\n")
			b.WriteString(headingStyle.Render(it.name))
			b.WriteString("\n")
			continue
		}
		if i == l.cursor {
			b.WriteString(barStyle.Render("▌ "))
		} else {
			b.WriteString("  ")
		}
		b.WriteString(highlightName(it.name, s.matched, i == l.cursor))
		if it.desc != "" {
			b.WriteString("  ")
			b.WriteString(descStyle.Render(it.desc))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// highlightName renders a row's name with the fuzzy-matched characters
// emphasized. matched holds byte indexes into name (names are effectively
// ASCII for matching, so byte and rune indexes coincide).
func highlightName(name string, matched []int, selected bool) string {
	base := nameStyle
	if selected {
		base = nameSelStyle
	}
	if len(matched) == 0 {
		return base.Render(name)
	}
	set := make(map[int]bool, len(matched))
	for _, idx := range matched {
		set[idx] = true
	}
	var b strings.Builder
	for i, r := range name {
		if set[i] {
			b.WriteString(matchStyle.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}
