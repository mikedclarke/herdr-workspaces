# herdr-workspaces

[![herdr](https://img.shields.io/badge/herdr-%E2%89%A5%200.7.0-blue)](https://herdr.dev)
[![license](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Directories as workspaces for [herdr](https://herdr.dev). Register the places you work — repos, agent directories, client folders — and open any of them as a named herdr workspace from one fuzzy picker, on one keybinding.

## What it does

- **Register a directory** from inside the picker (ctrl+a), from the CLI (`herdr-workspaces add ~/code/myrepo`), or by dropping a small TOML file into the config directory.
- **Pick one** from a fuzzy-filtered list (grouped under headings if you use groups), by keyboard or mouse.
- **Get a workspace**: a new focused herdr workspace rooted in that directory, labeled with the entry's name, optionally auto-running a startup command (your agent, an editor, a dev server) in its root pane.

That's the whole plugin. It doesn't template tabs and panes, run scheduled jobs, or manage worktrees — it opens directories as workspaces, quickly.

## Install

Requires herdr ≥ 0.7.0 on Linux or macOS, and a Go 1.24+ toolchain to build.

```bash
herdr plugin install mikedclarke/herdr-workspaces
```

Or from a checkout:

```bash
sh scripts/build.sh
herdr plugin link /path/to/herdr-workspaces
```

Installing registers one plugin action, `Workspaces: Open`. herdr has no built-in menu for plugin actions, so bind it to a key in `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+p"
type = "plugin_action"
command = "mikedclarke.herdr-workspaces.workspaces"
description = "workspaces: open picker"
```

(Any key works; `prefix+p` is a natural choice if it's free.)

## Registering workspaces

Each workspace is one TOML file in the plugin's config directory — `~/.config/herdr/plugins/config/mikedclarke.herdr-workspaces/workspaces/` when installed under herdr:

```toml
name = "myrepo"                    # workspace label; defaults to the directory basename
dir = "~/code/myrepo"              # required; ~ and $VARS are expanded when opening
description = "The main product"   # optional; shown dimmed in the picker
group = "Work"                     # optional; clusters entries under a heading
command = "claude"                 # optional; runs in the root pane once the shell is up
```

You rarely write these by hand: press **ctrl+a** in the picker, or:

```bash
herdr-workspaces add ~/code/myrepo --group Work --command claude
```

A malformed file fails the picker loudly, naming the file, instead of silently dropping the entry.

## Using the picker

Press your keybinding. Type to filter (fuzzy, across names and descriptions), arrows or ctrl+p/ctrl+n to move, Enter or click to open, ctrl+a to add the next directory without leaving the picker, Esc to close.

The startup command is typed into the new workspace's root pane the way you would type it: the plugin waits for the shell prompt, types the command, waits for it to echo, and submits it with a real Enter — so it runs instead of sitting at the prompt.

## CLI

The binary lives in the plugin directory (`bin/herdr-workspaces`); symlink it onto your `$PATH` if you want it from any shell.

```
herdr-workspaces picker      # the picker (opens as a herdr pane when not on a TTY)
herdr-workspaces list        # registered workspaces
herdr-workspaces add <dir>   # register a directory (--name --description --group --command)
herdr-workspaces open <name> # open one now (needs to run inside herdr)
herdr-workspaces version
```

## Development

```bash
sh scripts/build.sh   # build bin/herdr-workspaces
sh scripts/check.sh   # gofmt, go vet, go test -race — the pre-release gate
```

## License

MIT
