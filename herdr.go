package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// herdrClient talks to the running herdr server over its unix socket. The
// protocol is newline-delimited JSON: one request object per line, one
// response per line, over a short-lived connection per call. herdr injects
// HERDR_SOCKET_PATH into every plugin command, so this works whenever herdr
// runs us.
type herdrClient struct {
	socketPath string
}

func newHerdrClient() (*herdrClient, error) {
	path := os.Getenv("HERDR_SOCKET_PATH")
	if path == "" {
		return nil, errors.New("HERDR_SOCKET_PATH is not set; are you running inside herdr?")
	}
	return &herdrClient{socketPath: path}, nil
}

type herdrError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *herdrError) Error() string { return fmt.Sprintf("herdr: %s: %s", e.Code, e.Message) }

const callDeadline = 30 * time.Second

func (c *herdrClient) call(method string, params map[string]any, out any) error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("connect herdr socket: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(callDeadline)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	req := map[string]any{"id": "herdr-workspaces", "method": method, "params": params}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("write request: %w", err)
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *herdrError     `json:"error"`
	}
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&resp); err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.Error != nil {
		return resp.Error
	}
	if out != nil {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil
}

// workspaceCreate makes a new workspace rooted at cwd with the given label and
// returns the id of its root pane. When focus is true the user is switched to
// the new workspace.
func (c *herdrClient) workspaceCreate(cwd, label string, focus bool) (paneID string, err error) {
	var out struct {
		RootPane struct {
			PaneID string `json:"pane_id"`
		} `json:"root_pane"`
	}
	err = c.call("workspace.create", map[string]any{
		"cwd":   cwd,
		"label": label,
		"focus": focus,
	}, &out)
	if err != nil {
		return "", err
	}
	return out.RootPane.PaneID, nil
}

// sendInput types text into a pane and then presses the given keys, as if at
// the keyboard. To run a shell command, pass the command as text and "Enter"
// as the sole key — herdr pastes text, and once the shell's line editor is
// active an embedded "\n" is inserted literally instead of submitting.
func (c *herdrClient) sendInput(paneID, text string, keys ...string) error {
	params := map[string]any{
		"pane_id": paneID,
		"text":    text,
	}
	if len(keys) > 0 {
		params["keys"] = keys
	}
	return c.call("pane.send_input", params, nil)
}

// paneRead returns the text currently shown in a pane: the trailing `lines`
// rows of its visible screen.
func (c *herdrClient) paneRead(paneID string, lines int) (string, error) {
	var out struct {
		Read struct {
			Text string `json:"text"`
		} `json:"read"`
	}
	err := c.call("pane.read", map[string]any{
		"pane_id": paneID,
		"source":  "visible",
		"lines":   lines,
	}, &out)
	if err != nil {
		return "", err
	}
	return out.Read.Text, nil
}

// runCommand types command into a freshly created pane and submits it, pacing
// itself to the shell's startup so the command actually runs instead of
// sitting unsubmitted at the prompt. Two startup races are dodged: typing
// before the shell exists (keystrokes dropped), and pressing Enter before the
// line editor holds the text (line lost). Every wait is best effort — on
// timeout we proceed anyway, so a slow shell degrades to blind typing rather
// than hanging.
func (c *herdrClient) runCommand(paneID, command string) error {
	c.waitForPane(paneID, func(text string) bool {
		return strings.TrimSpace(text) != ""
	})
	if err := c.sendInput(paneID, command); err != nil {
		return err
	}
	probe := commandEchoProbe(command)
	c.waitForPane(paneID, func(text string) bool {
		return strings.Contains(text, probe)
	})
	return c.sendInput(paneID, "", "Enter")
}

const paneWaitTimeout = 5 * time.Second

// waitForPane polls the pane's visible text until ready reports true or the
// timeout elapses.
func (c *herdrClient) waitForPane(paneID string, ready func(string) bool) {
	deadline := time.Now().Add(paneWaitTimeout)
	for time.Now().Before(deadline) {
		if text, err := c.paneRead(paneID, 20); err == nil && ready(text) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// commandEchoProbe returns a short, stable fragment of a command to look for
// when confirming it was typed at the prompt: the first line, capped so it
// stays on a single terminal row even in a narrow pane.
func commandEchoProbe(command string) string {
	probe := command
	if i := strings.IndexByte(probe, '\n'); i >= 0 {
		probe = probe[:i]
	}
	if len(probe) > 12 {
		probe = probe[:12]
	}
	return strings.TrimSpace(probe)
}
