package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Workspace is one registered directory, loaded from a TOML file in the
// workspaces config directory. Opening it creates a new herdr workspace rooted
// at Dir, labeled Name, optionally running Command in the root pane.
type Workspace struct {
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	Group       string `toml:"group,omitempty"`
	Dir         string `toml:"dir"`
	Command     string `toml:"command,omitempty"`

	// source is the file the workspace was loaded from, used only for error
	// messages. It is not part of the on-disk format.
	source string `toml:"-"`
}

// configBaseDir returns the plugin's config root. When herdr runs us it sets
// HERDR_PLUGIN_CONFIG_DIR to the herdr-managed per-plugin directory — the
// canonical home, provisioned and isolated by herdr. Outside herdr (dev,
// tests, a bare shell) fall back to ~/.config/herdr-workspaces, honoring
// $XDG_CONFIG_HOME.
func configBaseDir() (string, error) {
	if d := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); d != "" {
		return d, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "herdr-workspaces"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "herdr-workspaces"), nil
}

func workspacesConfigDir() (string, error) {
	base, err := configBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "workspaces"), nil
}

// ensureWorkspacesDir makes sure the workspaces directory exists and returns
// its path. It is never seeded: an empty directory is meaningful — it triggers
// the picker's onboarding empty state.
func ensureWorkspacesDir() (string, error) {
	dir, err := workspacesConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// loadWorkspaces reads, parses, and validates every *.toml workspace in the
// config directory, returning them sorted by name. A malformed or invalid file
// fails the whole load with a message naming the offending files, so config
// mistakes surface loudly instead of an entry silently going missing. An empty
// directory returns an empty slice so the caller can show the empty state.
func loadWorkspaces() ([]Workspace, error) {
	dir, err := ensureWorkspacesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var workspaces []Workspace
	var problems []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		var w Workspace
		if _, err := toml.DecodeFile(filepath.Join(dir, e.Name()), &w); err != nil {
			problems = append(problems, fmt.Sprintf("  %s: %v", e.Name(), err))
			continue
		}
		w.source = e.Name()
		if err := w.normalize(); err != nil {
			problems = append(problems, "  "+err.Error())
			continue
		}
		workspaces = append(workspaces, w)
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid workspace files in %s:\n%s", dir, strings.Join(problems, "\n"))
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].Name < workspaces[j].Name })
	return workspaces, nil
}

// normalize validates the workspace and fills the name default (the
// directory's basename). The directory is not checked for existence here —
// that is a per-open concern (it might exist on one machine but not another).
func (w *Workspace) normalize() error {
	w.Dir = strings.TrimSpace(w.Dir)
	if w.Dir == "" {
		return fmt.Errorf("workspace %s: dir is required", w.source)
	}
	w.Name = strings.TrimSpace(w.Name)
	if w.Name == "" {
		w.Name = filepath.Base(strings.TrimRight(w.Dir, "/"))
	}
	return nil
}

// expandedDir resolves Dir to an absolute path, expanding a leading ~ to the
// home directory and $VAR / ${VAR} references.
func (w Workspace) expandedDir() (string, error) {
	dir := w.Dir
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory for dir %q: %w", w.Dir, err)
		}
		if dir == "~" {
			return home, nil
		}
		dir = filepath.Join(home, dir[2:])
	}
	return filepath.Clean(os.ExpandEnv(dir)), nil
}

// displayDir resolves the directory for UI contexts that have no way to
// surface an error, falling back to the raw dir so something still renders.
func (w Workspace) displayDir() string {
	dir, err := w.expandedDir()
	if err != nil {
		return w.Dir
	}
	return dir
}

// slugify turns a workspace name into its config filename stem: lowercase,
// runs of non-alphanumerics collapsed to single hyphens.
func slugify(name string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(name) {
		alnum := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if !alnum {
			pendingHyphen = b.Len() > 0
			continue
		}
		if pendingHyphen {
			b.WriteByte('-')
			pendingHyphen = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// addWorkspace validates a new entry and writes it as <slug>.toml in the
// config directory, returning the file's path. Unlike loading, adding checks
// the directory exists — catching a typo at entry time — and refuses to
// overwrite an existing file.
func addWorkspace(w Workspace) (string, error) {
	if err := w.normalize(); err != nil {
		return "", fmt.Errorf("dir is required")
	}
	expanded, err := w.expandedDir()
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(expanded); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("not a directory: %s", expanded)
	}

	slug := slugify(w.Name)
	if slug == "" {
		return "", fmt.Errorf("name %q has no usable characters for a filename", w.Name)
	}
	dir, err := ensureWorkspacesDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, slug+".toml")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("workspace file already exists: %s", path)
	}

	var buf strings.Builder
	if err := toml.NewEncoder(&buf).Encode(w); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
