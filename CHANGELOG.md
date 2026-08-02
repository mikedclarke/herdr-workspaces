# Changelog

## 0.2.0

- Navigation-first picker keys: `enter` open, `a` add, `e` edit, `d` delete
  (with confirm), `q` close, and `/` for the fuzzy filter. Esc leaves the
  filter applied so a filtered entry can be edited or deleted.
- Edit workspaces in place: the form (now including description) opens
  prefilled and saves back to the entry's file, moving it on rename.
- `edit` and `remove` CLI commands; `edit` changes only the flags you pass.

## 0.1.0

Initial release.

- Fuzzy workspace picker as a zoomed herdr pane, with group headings, mouse
  support, and an onboarding empty state.
- Workspaces registered as one TOML file per directory (name, description,
  group, dir, command).
- In-picker add form (ctrl+a) and `add` / `list` / `open` CLI commands.
- Optional startup command, pace-typed into the new workspace's root pane.
