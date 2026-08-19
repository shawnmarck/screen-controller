# Screen layouts (Omarchy bar)

Bar widget for [screen-controller](https://github.com/shawnmarck/screen-controller).
It talks to the same `profiles.yaml` and `apply` path as the TUI.

- Left-click: pick a profile, save the current Hyprland layout, relabel, delete
- Right-click: open the existing TUI
- Super+Shift+F4 stays as a TUI bind; this widget does not replace it

Install: symlink this folder to `~/.config/omarchy/plugins/shawnmarck.screen-layouts`,
put the widget next to `omarchy.monitor` in `~/.config/omarchy/shell.json`, and
point `binary` at a built `screen-controller`.
