# herdr-workspaces

[![release](https://img.shields.io/github/v/release/mikedclarke/herdr-workspaces)](https://github.com/mikedclarke/herdr-workspaces/releases)
[![herdr](https://img.shields.io/badge/herdr-%E2%89%A5%200.7.0-blue)](https://herdr.dev)
[![license](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Directories as workspaces for [herdr](https://herdr.dev). Register the places you work (repos, agent directories, client folders) and open any of them as a named herdr workspace from one picker, on one keybinding.

## What it does

- **Register a directory** from inside the picker, from the CLI, or by dropping a small TOML file into the config directory. Edit and delete entries the same way.
- **Pick one** from a grouped list, by keyboard or mouse, with fuzzy filtering.
- **Get a workspace**: a new focused herdr workspace rooted in that directory, labeled with the entry's name, optionally auto-running a startup command (your agent, an editor, a dev server) in its root pane.

## Install

Requires herdr ≥ 0.7.0 on Linux or macOS.

```bash
herdr plugin install mikedclarke/herdr-workspaces
```

Or from a checkout:

```bash
sh scripts/build.sh
herdr plugin link /path/to/herdr-workspaces
```

`scripts/build.sh` prefers a local Go toolchain (an exact build of the source you have) and falls back to `scripts/install.sh`, which downloads the matching prebuilt binary for your OS and architecture from the GitHub release and checks it against the release's `SHA256SUMS`. No Go toolchain is required.

Installing registers one plugin action, `Workspaces: Open`. herdr has no built-in menu for plugin actions, so bind it to a key in `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+p"
type = "plugin_action"
command = "mikedclarke.herdr-workspaces.workspaces"
description = "workspaces: open picker"
```

(Any key works; `prefix+p` is a natural choice if it's free.)

## Using the picker

Press your keybinding. Single keys act:

| key | action |
| --- | --- |
| `enter` (or click) | open the selected workspace |
| `→` (right arrow) | name this session, then open |
| `j` / `k` / arrows | move |
| `/` | fuzzy-filter names and descriptions |
| `a` | add a directory |
| `e` | edit the selected entry |
| `d` | delete the selected entry (asks to confirm) |
| `q` / `esc` | close |

While filtering, Enter opens the top match directly, and Esc leaves the filter applied so you can act on what you found (for example `/cli` Esc `e` to edit the first match).

### Naming a session

`enter` opens a workspace labeled with the entry's name. When you want a second session in the same directory (one is already running and you would rather not open another with the same label), press `→` on the entry instead: a one-line prompt opens, prefilled with the entry's name and the cursor at the end, so you can tack on a suffix like `-2` and press Enter. The label applies to this session only; the registered entry is untouched. Clearing the prompt and pressing Enter falls back to the entry's name, the same as `enter`.

The startup command is typed into the new workspace's root pane the way you would type it: the plugin waits for the shell prompt, types the command, waits for it to echo, and submits it with a real Enter, so it runs instead of sitting at the prompt.

## Registering workspaces

Each workspace is one TOML file in the plugin's config directory, `~/.config/herdr/plugins/config/mikedclarke.herdr-workspaces/workspaces/`:

```toml
name = "myrepo"                    # workspace label; defaults to the directory basename
dir = "~/code/myrepo"              # required; ~ and $VARS are expanded when opening
description = "The main product"   # optional; shown dimmed in the picker
group = "Work"                     # optional; clusters entries under a heading
command = "claude"                 # optional; runs in the root pane once the shell is up
```

You rarely write these by hand: press `a` or `e` in the picker, or use the CLI. A malformed file fails the picker loudly, naming the file, instead of silently dropping the entry.

## CLI

herdr builds the binary inside the plugin's own directory (`bin/herdr-workspaces`); symlink it onto your `$PATH` if you want it from any shell.

```
herdr-workspaces picker        # the picker (opens as a herdr pane when not on a TTY)
herdr-workspaces list          # registered workspaces
herdr-workspaces add <dir>     # register a directory (--name --description --group --command)
herdr-workspaces edit <name>   # change an entry; only the flags you pass change
herdr-workspaces remove <name> # delete an entry's config file
herdr-workspaces open <name>   # open one now, --label sets this session's name (needs herdr)
herdr-workspaces version
```

## Update

Updates ride herdr's plugin mechanics. Installed from GitHub, herdr pins the commit it installed; to move to the latest, reinstall:

```bash
herdr plugin uninstall mikedclarke.herdr-workspaces
herdr plugin install mikedclarke/herdr-workspaces
```

From a linked checkout, `git pull` and `sh scripts/build.sh`. Your workspace files live in herdr's per-plugin config directory and survive uninstall and reinstall.

## Development

```bash
sh scripts/build.sh    # build bin/herdr-workspaces
sh scripts/check.sh    # gofmt, go vet, go test -race; the pre-release gate
sh scripts/release.sh  # cross-compile dist/ tarballs + SHA256SUMS
```

This repository has no CI by design; `scripts/check.sh` is the contract. Run it before opening a pull request.

## License

MIT
