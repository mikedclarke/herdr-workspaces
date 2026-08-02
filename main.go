package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const version = "0.2.0"

func usage(w *os.File) {
	fmt.Fprintln(w, `usage: herdr-workspaces <command>

  picker         Open the workspace picker (opens as a herdr pane when
                 invoked without a terminal)
  list           List registered workspaces
  add <dir>      Register a directory as a workspace
    --name         workspace name (default: the directory's basename)
    --description  shown dimmed in the picker
    --group        groups entries under a heading in the picker
    --command      run in the root pane when the workspace opens
  edit <name>    Change a registered workspace; only the flags you pass
                 change (--name --dir --description --group --command),
                 and an empty value clears the field
  remove <name>  Delete a workspace's config file
  open <name>    Open a registered workspace now (requires herdr)
  version        Print the version`)
}

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "picker":
		err = cmdPicker()
	case "picker-ui":
		// Internal: the manifest's pane entrypoint (herdr hosts the TUI).
		err = runPickerUI()
	case "list":
		err = cmdList()
	case "add":
		err = cmdAdd(os.Args[2:])
	case "edit":
		err = cmdEdit(os.Args[2:])
	case "remove":
		if len(os.Args) < 3 {
			err = fmt.Errorf("remove: workspace name required")
		} else {
			var path string
			if path, err = removeWorkspace(os.Args[2]); err == nil {
				fmt.Println("removed:", path)
			}
		}
	case "open":
		if len(os.Args) < 3 {
			err = fmt.Errorf("open: workspace name required")
		} else {
			err = cmdOpen(os.Args[2])
		}
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "herdr-workspaces: unknown command %q\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "herdr-workspaces:", err)
		os.Exit(1)
	}
}

func cmdList() error {
	workspaces, err := loadWorkspaces()
	if err != nil {
		return err
	}
	if len(workspaces) == 0 {
		dir, _ := workspacesConfigDir()
		fmt.Printf("no workspaces registered; add one with `herdr-workspaces add <dir>` or drop a .toml into %s\n", dir)
		return nil
	}
	for _, w := range workspaces {
		line := w.Name
		if w.Group != "" {
			line += "  [" + w.Group + "]"
		}
		line += "  " + w.displayDir()
		if w.Command != "" {
			line += "  → " + w.Command
		}
		fmt.Println(line)
	}
	return nil
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	name := fs.String("name", "", "workspace name")
	description := fs.String("description", "", "description")
	group := fs.String("group", "", "picker group")
	command := fs.String("command", "", "startup command")
	// flag.Parse stops at the first non-flag argument, so peel a leading
	// directory off first, so both `add <dir> --group X` and `add --group X
	// <dir>` work.
	var dir string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		dir, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case dir == "" && fs.NArg() == 1:
		dir = fs.Arg(0)
	case dir == "" || fs.NArg() != 0:
		return fmt.Errorf("add: exactly one directory required")
	}
	w := Workspace{
		Name:        *name,
		Description: *description,
		Group:       *group,
		Dir:         dir,
		Command:     *command,
	}
	path, err := addWorkspace(w)
	if err != nil {
		return err
	}
	fmt.Println("added:", path)
	return nil
}

func cmdEdit(args []string) error {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	name := fs.String("name", "", "workspace name")
	dir := fs.String("dir", "", "directory")
	description := fs.String("description", "", "description")
	group := fs.String("group", "", "picker group")
	command := fs.String("command", "", "startup command")

	var target string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		target, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case target == "" && fs.NArg() == 1:
		target = fs.Arg(0)
	case target == "" || fs.NArg() != 0:
		return fmt.Errorf("edit: exactly one workspace name required")
	}

	workspaces, err := loadWorkspaces()
	if err != nil {
		return err
	}
	for _, w := range workspaces {
		if w.Name != target {
			continue
		}
		// Only flags the user actually passed change the entry, so an
		// explicit empty value can clear a field.
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "name":
				w.Name = *name
			case "dir":
				w.Dir = *dir
			case "description":
				w.Description = *description
			case "group":
				w.Group = *group
			case "command":
				w.Command = *command
			}
		})
		path, err := updateWorkspace(w, w.source)
		if err != nil {
			return err
		}
		fmt.Println("updated:", path)
		return nil
	}
	return fmt.Errorf("no workspace named %q; see `herdr-workspaces list`", target)
}

func cmdOpen(name string) error {
	workspaces, err := loadWorkspaces()
	if err != nil {
		return err
	}
	for _, w := range workspaces {
		if w.Name == name {
			client, err := newHerdrClient()
			if err != nil {
				return err
			}
			return openWorkspace(client, w)
		}
	}
	return fmt.Errorf("no workspace named %q; see `herdr-workspaces list`", name)
}
