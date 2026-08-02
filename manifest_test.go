package main

import (
	"testing"

	"github.com/BurntSushi/toml"
)

// The manifest and the binary must agree on identity and version; herdr trusts
// the manifest, the code trusts its constants.
func TestManifestMatchesBinary(t *testing.T) {
	var m struct {
		ID      string `toml:"id"`
		Version string `toml:"version"`
		Actions []struct {
			ID      string   `toml:"id"`
			Command []string `toml:"command"`
		} `toml:"actions"`
		Panes []struct {
			ID      string   `toml:"id"`
			Command []string `toml:"command"`
		} `toml:"panes"`
	}
	if _, err := toml.DecodeFile("herdr-plugin.toml", &m); err != nil {
		t.Fatal(err)
	}
	if m.ID != pluginID {
		t.Errorf("manifest id %q != pluginID %q", m.ID, pluginID)
	}
	if m.Version != version {
		t.Errorf("manifest version %q != main.version %q", m.Version, version)
	}
	if len(m.Actions) != 1 || m.Actions[0].ID != "workspaces" {
		t.Errorf("actions = %+v, want the single workspaces action", m.Actions)
	}
	if len(m.Panes) != 1 || m.Panes[0].ID != "picker" {
		t.Errorf("panes = %+v, want the single picker pane", m.Panes)
	}
}
