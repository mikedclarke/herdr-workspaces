package main

import "testing"

func sampleList() fuzzyList {
	return newFuzzyList("filter", []listItem{
		{name: "Agents"},
		{name: "engineer", desc: "tooling", selectable: true, ref: 0},
		{name: "growth", selectable: true, ref: 1},
		{name: "Sites"},
		{name: "blog", desc: "the website", selectable: true, ref: 2},
	})
}

func TestFuzzyListNavigationSkipsHeadings(t *testing.T) {
	l := sampleList()
	if got := l.selectedRef(); got != 0 {
		t.Fatalf("initial selection ref = %d, want 0", got)
	}
	l.moveDown()
	if got := l.selectedRef(); got != 1 {
		t.Fatalf("after moveDown ref = %d, want 1", got)
	}
	l.moveDown()
	if got := l.selectedRef(); got != 2 {
		t.Fatalf("moveDown should skip the Sites heading, ref = %d, want 2", got)
	}
	l.moveDown()
	if got := l.selectedRef(); got != 2 {
		t.Fatalf("moveDown at end should stay, ref = %d", got)
	}
	l.moveUp()
	l.moveUp()
	if got := l.selectedRef(); got != 0 {
		t.Fatalf("after moving back up ref = %d, want 0", got)
	}
}

func TestFuzzyListFilterMatchesNameAndDescription(t *testing.T) {
	l := sampleList()

	l.input.SetValue("webs")
	l.filter()
	if got := l.selectedRef(); got != 2 {
		t.Fatalf("description match ref = %d, want 2", got)
	}
	for _, s := range l.filtered {
		if !s.item.selectable {
			t.Fatal("headings should be hidden while filtering")
		}
	}

	l.input.SetValue("zzz")
	l.filter()
	if got := l.selectedRef(); got != -1 {
		t.Fatalf("no-match ref = %d, want -1", got)
	}

	l.input.SetValue("")
	l.filter()
	if len(l.filtered) != 5 {
		t.Fatalf("cleared filter shows %d rows, want 5", len(l.filtered))
	}
}

func TestFuzzyListClickRow(t *testing.T) {
	l := sampleList()
	// Layout: line 0 prompt, 1 blank, 2 blank (heading lead), 3 "Agents",
	// 4 engineer, 5 growth, 6 blank, 7 "Sites", 8 blog.
	if l.clickRow(3) {
		t.Error("clicking a heading should not select")
	}
	if !l.clickRow(5) {
		t.Fatal("clicking the growth row should select")
	}
	if got := l.selectedRef(); got != 1 {
		t.Fatalf("clicked ref = %d, want 1", got)
	}
	if !l.clickRow(8) {
		t.Fatal("clicking the blog row should select")
	}
	if got := l.selectedRef(); got != 2 {
		t.Fatalf("clicked ref = %d, want 2", got)
	}
	if l.clickRow(40) {
		t.Error("clicking past the end should not select")
	}
}
