# Changelog

## Unreleased

- `enter` on an entry whose workspace is already open switches to it instead
  of opening a second one with the same label (the lowest-numbered one when
  several share it). `→` still opens a new session, as does `open` from the
  CLI.

## 0.3.0

- Name a session on open: press `→` (right arrow) on a picker entry for a
  one-line prompt, prefilled with the entry's name, to label this session
  apart from one already running in the same directory. The label is for this
  open only; the registered entry is untouched, and a cleared prompt falls
  back to the name.
- `open --label <label>` does the same from the CLI.

## 0.2.0

- Navigation-first picker keys: `enter` open, `a` add, `e` edit, `d` delete
  (with confirm), `q` close, and `/` for the fuzzy filter. Esc leaves the
  filter applied so a filtered entry can be edited or deleted.
- Edit workspaces in place: the form (now including description) opens
  prefilled and saves back to the entry's file, moving it on rename.
- `edit` and `remove` CLI commands; `edit` changes only the flags you pass.
- The CLI finds the herdr-managed config directory from a plain shell.
- Prebuilt release binaries: `scripts/build.sh` falls back to
  `scripts/install.sh` (download + SHA256 verify) when Go is absent.

## 0.1.0

Initial release.

- Fuzzy workspace picker as a zoomed herdr pane, with group headings, mouse
  support, and an onboarding empty state.
- Workspaces registered as one TOML file per directory (name, description,
  group, dir, command).
- In-picker add form and `add` / `list` / `open` CLI commands.
- Optional startup command, pace-typed into the new workspace's root pane.
