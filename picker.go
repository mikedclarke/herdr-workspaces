package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	isatty "github.com/mattn/go-isatty"
)

const pluginID = "mikedclarke.herdr-workspaces"

// cmdPicker opens the workspace picker. From a shell (both stdin and stdout
// are a terminal) it runs the TUI right here; invoked as the manifest action
// (server-side, no terminal) it asks herdr to host the `picker` pane
// entrypoint instead, so herdr creates and tears down the pane.
func cmdPicker() error {
	if isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) {
		return runPicker(false)
	}
	herdr := os.Getenv("HERDR_BIN_PATH")
	if herdr == "" {
		herdr = "herdr"
	}
	cmd := exec.Command(herdr, "plugin", "pane", "open",
		"--plugin", pluginID,
		"--entrypoint", "picker",
		"--placement", "zoomed",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open picker pane: %w", err)
	}
	return nil
}

// runPickerUI is the manifest's pane entrypoint: the picker hosted by herdr
// in a zoomed plugin pane.
func runPickerUI() error {
	return runPicker(true)
}

// runPicker renders the full-screen picker. When a workspace is chosen it
// opens it; on cancel it simply exits. hosted marks the herdr-hosted pane,
// which is torn down the moment this process exits; there an error must wait
// for a keypress or it vanishes before it can be read.
func runPicker(hosted bool) error {
	workspaces, err := loadWorkspaces()
	if err != nil {
		return holdError(hosted, err)
	}

	// Mouse cell motion: herdr forwards clicks and wheel events to the pane
	// once we ask for them, so the picker works by touch as well as keys.
	p := tea.NewProgram(newPickerModel(workspaces), tea.WithAltScreen(), tea.WithMouseCellMotion())
	result, err := p.Run()
	if err != nil {
		return holdError(hosted, err)
	}

	m, ok := result.(pickerModel)
	if !ok || m.chosen == nil {
		return nil
	}
	client, err := newHerdrClient()
	if err != nil {
		return holdError(hosted, err)
	}
	if err := openWorkspace(client, *m.chosen, m.chosenLabel); err != nil {
		return holdError(hosted, fmt.Errorf("open workspace %q: %w", m.chosen.Name, err))
	}
	return nil
}

// holdError keeps a hosted pane alive until Enter so the error is readable;
// herdr closes the pane when the process exits, which would otherwise reduce
// the message to a flash. Outside a hosted pane the shell keeps the output on
// screen, so the error passes straight through.
func holdError(hosted bool, err error) error {
	if !hosted {
		return err
	}
	fmt.Fprintln(os.Stderr, "herdr-workspaces:", err)
	fmt.Fprint(os.Stderr, "\npress enter to close ")
	bufio.NewReader(os.Stdin).ReadString('\n')
	return err
}

// openWorkspace turns a registered directory into a live herdr workspace: a
// focused workspace rooted there, with the optional startup command run in its
// root pane. Creating the focused workspace switches the user to it. label is
// this session's workspace label; empty falls back to the entry's name, so a
// second session in the same directory can be named apart from the first.
func openWorkspace(client *herdrClient, w Workspace, label string) error {
	dir, err := w.expandedDir()
	if err != nil {
		return err
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("directory does not exist: %s", dir)
	}
	if label == "" {
		label = w.Name
	}
	paneID, err := client.workspaceCreate(dir, label, true)
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	if w.Command != "" {
		if err := client.runCommand(paneID, w.Command); err != nil {
			return fmt.Errorf("run startup command: %w", err)
		}
	}
	return nil
}
