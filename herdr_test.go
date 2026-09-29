package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type stubCall struct {
	Method string
	Params map[string]any
}

// stubHerdr is a fake herdr server on a unix socket: one JSON request per
// connection, one canned response, every call recorded.
type stubHerdr struct {
	path string
	ln   net.Listener

	mu    sync.Mutex
	calls []stubCall

	handle func(method string, params map[string]any) (any, *herdrError)
}

func newStubHerdr(t *testing.T, handle func(method string, params map[string]any) (any, *herdrError)) *stubHerdr {
	t.Helper()
	// Not t.TempDir(): its test-name-derived path can overflow the ~104-byte
	// unix socket path limit on macOS.
	dir, err := os.MkdirTemp("", "hw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	s := &stubHerdr{path: path, ln: ln, handle: handle}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *stubHerdr) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			defer conn.Close()
			var req struct {
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
				return
			}
			s.mu.Lock()
			s.calls = append(s.calls, stubCall{Method: req.Method, Params: req.Params})
			s.mu.Unlock()

			result, herr := s.handle(req.Method, req.Params)
			resp := map[string]any{}
			if herr != nil {
				resp["error"] = herr
			} else {
				resp["result"] = result
			}
			json.NewEncoder(conn).Encode(resp)
		}(conn)
	}
}

func (s *stubHerdr) recorded() []stubCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubCall(nil), s.calls...)
}

func TestWorkspaceCreateParams(t *testing.T) {
	s := newStubHerdr(t, func(method string, params map[string]any) (any, *herdrError) {
		if method != "workspace.create" {
			return nil, &herdrError{Code: "unexpected", Message: method}
		}
		return map[string]any{"root_pane": map[string]any{"pane_id": "p1"}}, nil
	})
	c := &herdrClient{socketPath: s.path}

	paneID, err := c.workspaceCreate("/tmp/x", "myrepo", true)
	if err != nil {
		t.Fatal(err)
	}
	if paneID != "p1" {
		t.Fatalf("paneID = %q, want p1", paneID)
	}
	calls := s.recorded()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	p := calls[0].Params
	if p["cwd"] != "/tmp/x" || p["label"] != "myrepo" || p["focus"] != true {
		t.Fatalf("params = %+v", p)
	}
}

func TestCallSurfacesHerdrError(t *testing.T) {
	s := newStubHerdr(t, func(method string, params map[string]any) (any, *herdrError) {
		return nil, &herdrError{Code: "pane_not_found", Message: "gone"}
	})
	c := &herdrClient{socketPath: s.path}

	_, err := c.workspaceCreate("/tmp/x", "x", false)
	var herr *herdrError
	if !errors.As(err, &herr) || herr.Code != "pane_not_found" {
		t.Fatalf("err = %v, want herdrError pane_not_found", err)
	}
}

func TestRunCommandPacesTypeThenEnter(t *testing.T) {
	s := newStubHerdr(t, func(method string, params map[string]any) (any, *herdrError) {
		switch method {
		case "pane.read":
			return map[string]any{"read": map[string]any{"text": "❯ claude"}}, nil
		case "pane.send_input":
			return map[string]any{}, nil
		}
		return nil, &herdrError{Code: "unexpected", Message: method}
	})
	c := &herdrClient{socketPath: s.path}

	if err := c.runCommand("p1", "claude"); err != nil {
		t.Fatal(err)
	}

	var sends []stubCall
	for _, call := range s.recorded() {
		if call.Method == "pane.send_input" {
			sends = append(sends, call)
		}
	}
	if len(sends) != 2 {
		t.Fatalf("got %d send_input calls, want 2 (type, then Enter)", len(sends))
	}
	if sends[0].Params["text"] != "claude" {
		t.Errorf("first send text = %v", sends[0].Params["text"])
	}
	if _, hasKeys := sends[0].Params["keys"]; hasKeys {
		t.Error("typing must not press keys; Enter is a separate send")
	}
	keys, _ := sends[1].Params["keys"].([]any)
	if len(keys) != 1 || keys[0] != "Enter" {
		t.Errorf("second send keys = %v, want [Enter]", sends[1].Params["keys"])
	}
	if sends[1].Params["text"] != "" {
		t.Errorf("second send text = %v, want empty", sends[1].Params["text"])
	}
}

func TestCommandEchoProbe(t *testing.T) {
	cases := []struct{ in, want string }{
		{"claude", "claude"},
		{"a very long command line here", "a very long"},
		{"first\nsecond", "first"},
	}
	for _, c := range cases {
		if got := commandEchoProbe(c.in); got != c.want {
			t.Errorf("commandEchoProbe(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNewHerdrClientRequiresSocket(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "")
	if _, err := newHerdrClient(); err == nil || !strings.Contains(err.Error(), "HERDR_SOCKET_PATH") {
		t.Fatalf("err = %v, want HERDR_SOCKET_PATH message", err)
	}
}

func openStub(t *testing.T, open []map[string]any) *stubHerdr {
	return newStubHerdr(t, func(method string, params map[string]any) (any, *herdrError) {
		switch method {
		case "workspace.list":
			return map[string]any{"type": "workspace_list", "workspaces": open}, nil
		case "workspace.focus":
			return map[string]any{"type": "ok"}, nil
		case "workspace.create":
			return map[string]any{"root_pane": map[string]any{"pane_id": "p9"}}, nil
		}
		return nil, &herdrError{Code: "unexpected", Message: method}
	})
}

func methods(calls []stubCall) []string {
	var out []string
	for _, c := range calls {
		out = append(out, c.Method)
	}
	return out
}

func TestOpenWorkspaceFocusesTheOneAlreadyOpen(t *testing.T) {
	s := openStub(t, []map[string]any{
		{"workspace_id": "w7", "number": 7, "label": "beta"},
		{"workspace_id": "w3", "number": 3, "label": "beta"},
		{"workspace_id": "w1", "number": 1, "label": "beta-2"},
	})
	c := &herdrClient{socketPath: s.path}
	w := Workspace{Name: "beta", Dir: t.TempDir()}
	if err := openWorkspace(c, w, "", false); err != nil {
		t.Fatal(err)
	}
	calls := s.recorded()
	if got := strings.Join(methods(calls), ","); got != "workspace.list,workspace.focus" {
		t.Fatalf("calls = %s, want list then focus", got)
	}
	if calls[1].Params["workspace_id"] != "w3" {
		t.Fatalf("focused %v, want the lowest-numbered w3", calls[1].Params["workspace_id"])
	}
}

func TestOpenWorkspaceCreatesWhenNoneOpenOrANewSessionIsAsked(t *testing.T) {
	s := openStub(t, []map[string]any{{"workspace_id": "w1", "number": 1, "label": "beta-2"}})
	c := &herdrClient{socketPath: s.path}
	w := Workspace{Name: "beta", Dir: t.TempDir()}
	if err := openWorkspace(c, w, "", false); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(methods(s.recorded()), ","); got != "workspace.list,workspace.create" {
		t.Fatalf("calls = %s, want list then create", got)
	}

	s = openStub(t, []map[string]any{{"workspace_id": "w3", "number": 3, "label": "beta"}})
	c = &herdrClient{socketPath: s.path}
	if err := openWorkspace(c, w, "beta", true); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(methods(s.recorded()), ","); got != "workspace.create" {
		t.Fatalf("calls = %s, want a new workspace without a lookup", got)
	}
}
