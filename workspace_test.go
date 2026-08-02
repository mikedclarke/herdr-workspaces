package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// withConfigDir points the config root at a fresh temp directory for one test.
func withConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", dir)
	return dir
}

func writeWorkspaceFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "workspaces"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workspaces", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadWorkspacesSortsAndDefaultsName(t *testing.T) {
	dir := withConfigDir(t)
	writeWorkspaceFile(t, dir, "b.toml", "name = \"zeta\"\ndir = \"~/z\"\n")
	writeWorkspaceFile(t, dir, "a.toml", "dir = \"~/code/alpha\"\n")

	ws, err := loadWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Fatalf("got %d workspaces, want 2", len(ws))
	}
	if ws[0].Name != "alpha" {
		t.Errorf("defaulted name = %q, want %q (basename of dir)", ws[0].Name, "alpha")
	}
	if ws[1].Name != "zeta" {
		t.Errorf("sort order: got %q last, want zeta", ws[1].Name)
	}
}

func TestLoadWorkspacesEmptyDir(t *testing.T) {
	withConfigDir(t)
	ws, err := loadWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 0 {
		t.Fatalf("got %d workspaces, want 0", len(ws))
	}
}

func TestLoadWorkspacesReportsBadFiles(t *testing.T) {
	dir := withConfigDir(t)
	writeWorkspaceFile(t, dir, "good.toml", "dir = \"~/ok\"\n")
	writeWorkspaceFile(t, dir, "broken.toml", "dir = \n")
	writeWorkspaceFile(t, dir, "nodir.toml", "name = \"x\"\n")

	_, err := loadWorkspaces()
	if err == nil {
		t.Fatal("want error for bad files, got nil")
	}
	for _, frag := range []string{"broken.toml", "nodir.toml"} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("error does not name %s: %v", frag, err)
		}
	}
	if strings.Contains(err.Error(), "good.toml") {
		t.Errorf("error wrongly names the valid file: %v", err)
	}
}

func TestExpandedDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HW_TEST_VAR", "/opt/x")

	cases := []struct{ in, want string }{
		{"~", home},
		{"~/code/a", filepath.Join(home, "code", "a")},
		{"/abs/path", "/abs/path"},
		{"/abs/path/", "/abs/path"},
		{"$HW_TEST_VAR/y", "/opt/x/y"},
	}
	for _, c := range cases {
		got, err := Workspace{Dir: c.in}.expandedDir()
		if err != nil {
			t.Errorf("expandedDir(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("expandedDir(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"engineer", "engineer"},
		{"My Repo", "my-repo"},
		{"a  b--c", "a-b-c"},
		{"  Side Projects!  ", "side-projects"},
		{"···", ""},
	}
	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAddWorkspaceRoundTrip(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()

	path, err := addWorkspace(Workspace{Dir: target, Group: "Work", Command: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	var got Workspace
	if _, err := toml.DecodeFile(path, &got); err != nil {
		t.Fatal(err)
	}
	wantName := filepath.Base(target)
	if got.Name != wantName || got.Dir != target || got.Group != "Work" || got.Command != "claude" {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if got.Description != "" {
		t.Errorf("empty description was written: %+v", got)
	}

	ws, err := loadWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 1 || ws[0].Name != wantName {
		t.Errorf("loadWorkspaces after add = %+v", ws)
	}
}

func TestAddWorkspaceRejects(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()

	if _, err := addWorkspace(Workspace{Dir: ""}); err == nil {
		t.Error("empty dir: want error")
	}
	if _, err := addWorkspace(Workspace{Dir: filepath.Join(target, "missing")}); err == nil {
		t.Error("nonexistent dir: want error")
	}
	if _, err := addWorkspace(Workspace{Name: "dupe", Dir: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := addWorkspace(Workspace{Name: "dupe", Dir: target}); err == nil {
		t.Error("duplicate slug: want error")
	}
	if _, err := addWorkspace(Workspace{Name: "···", Dir: target}); err == nil {
		t.Error("unsluggable name: want error")
	}
}

func TestUpdateWorkspaceInPlace(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()
	if _, err := addWorkspace(Workspace{Name: "thing", Dir: target}); err != nil {
		t.Fatal(err)
	}

	path, err := updateWorkspace(Workspace{Name: "thing", Dir: target, Command: "claude"}, "thing.toml")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "thing.toml" {
		t.Fatalf("path = %s, want same file", path)
	}
	ws, err := loadWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 1 || ws[0].Command != "claude" {
		t.Fatalf("after update: %+v", ws)
	}
}

func TestUpdateWorkspaceRenameMovesFile(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()
	if _, err := addWorkspace(Workspace{Name: "old", Dir: target}); err != nil {
		t.Fatal(err)
	}

	path, err := updateWorkspace(Workspace{Name: "new name", Dir: target}, "old.toml")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "new-name.toml" {
		t.Fatalf("path = %s, want new-name.toml", path)
	}
	ws, err := loadWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 1 || ws[0].Name != "new name" || ws[0].source != "new-name.toml" {
		t.Fatalf("after rename: %+v", ws)
	}
}

func TestUpdateWorkspaceRenameCollision(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()
	if _, err := addWorkspace(Workspace{Name: "one", Dir: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := addWorkspace(Workspace{Name: "two", Dir: target}); err != nil {
		t.Fatal(err)
	}

	if _, err := updateWorkspace(Workspace{Name: "two", Dir: target}, "one.toml"); err == nil {
		t.Fatal("renaming over another entry: want error")
	}
	// Both entries must survive the refused rename.
	ws, err := loadWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Fatalf("after refused rename: %+v", ws)
	}
}

func TestRemoveWorkspace(t *testing.T) {
	withConfigDir(t)
	target := t.TempDir()
	if _, err := addWorkspace(Workspace{Name: "gone", Dir: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := removeWorkspace("nope"); err == nil {
		t.Error("unknown name: want error")
	}
	if _, err := removeWorkspace("gone"); err != nil {
		t.Fatal(err)
	}
	if ws, _ := loadWorkspaces(); len(ws) != 0 {
		t.Fatalf("after remove: %+v", ws)
	}
}
