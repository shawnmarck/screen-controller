# screen-controller

Switch **Hyprland monitor layouts** from a small terminal UI (gocui) or from the CLI. When you disable an output, it **moves windows off that monitor** first (by workspace) so fewer clients are lost on hot-unplug.

Designed for **Omarchy** (theme colors from `~/.config/omarchy/current/theme/kitty.conf`) and **Hyprland 0.54+** window rules.

## Requirements

- **Hyprland** session (`hyprctl` on `PATH`)
- **Go** 1.22+ (build only)
- Optional: **Omarchy** `omarchy-launch-tui` for the Super+Shift+F4 launcher (uses your default `xdg-terminal-exec` terminal)

## Build

```bash
cd ~/projects/screen-controller
go build -buildvcs=false -o screen-controller .
```

## Config path (`-config`)

If you omit **`-config`**, the default is the first path that exists:

1. **`$XDG_CONFIG_HOME/screen-controller/profiles.yaml`** (often `~/.config/screen-controller/profiles.yaml`)
2. **`~/projects/screen-controller/profiles.yaml`** (legacy layout from development)

If neither exists yet, the default is **`$XDG_CONFIG_HOME/screen-controller/profiles.yaml`** so new installs have a stable location; otherwise **`./profiles.yaml`** if neither home nor XDG config dir could be resolved.

## Usage

### TUI

```bash
./screen-controller
./screen-controller -config /path/to/profiles.yaml
```

Keys: **j/k** or arrows, **Home/End**, **1–9** jump, **Enter** apply, **r** reload YAML, **?** help, **q** / **Esc** quit. **●** marks the profile whose enabled outputs match Hyprland; a right pane (wide terminals) shows `monitor=` lines for the selection.

### CLI

```bash
./screen-controller list
./screen-controller apply dual_sdr
./screen-controller describe              # Hyprland vs profiles (matched id)
./screen-controller describe dual_sdr    # plus that profile’s monitor lines
./screen-controller -config ./profiles.yaml apply single_left_sdr
```

## `profiles.yaml`

- **`primary_monitor`**: Hyprland output name (first field of a `monitor=` line). **Every profile must include at least one line whose output name matches** `primary_monitor` (active or `disable`), so typos are caught at load time.
- **`safe_workspace`**: workspace id (integer) windows are moved to before an output is disabled — use one pinned to your primary monitor (e.g. `1` on DP-1). **`0` is allowed**; omitting the key is an error.
- **`profile_order`**: list of profile keys for TUI ordering.
- **`profiles.<id>.label`**: shown in the TUI.
- **`profiles.<id>.monitors`**: Hyprland `monitor=` lines (`Output,mode,...` or `Output,disable`).

Example:

```yaml
primary_monitor: DP-1
safe_workspace: 1
profiles:
  dual_sdr:
    label: Dual SDR
    monitors:
      - DP-1,3840x2160@144,0x0,1.5,bitdepth,10,cm,auto
      - HDMI-A-1,3840x2160@144,2560x0,1.5,bitdepth,10,cm,auto
```

Adjust names and modes to match `hyprctl monitors` / your `monitors.conf`.

## Hyprland integration (example)

**Bind** (Omarchy-style launcher):

```ini
bindd = SUPER SHIFT, F4, Screen layouts, exec, omarchy-launch-tui /ABS/PATH/TO/screen-controller
```

**Window rules** for app-id `org.omarchy.screen-controller`:

```ini
windowrule = float on, match:class ^org\.omarchy\.screen-controller$
windowrule = center on, match:class ^org\.omarchy\.screen-controller$
windowrule = size 920 540, match:class ^org\.omarchy\.screen-controller$
```

Reload: `hyprctl reload`.

## Reliability notes

- Runs **`hyprctl`** only; does not reload your full Hyprland config.
- **`apply`** and the **TUI** always run **`hyprctl version`** (after checking `hyprctl` is on `PATH`) so a stale `HYPRLAND_INSTANCE_SIGNATURE` alone does not skip the check.
- If **`hyprctl --batch`** fails, the tool retries **`hyprctl keyword monitor …`** per line. If every line works, the layout still applies (batch quirks, quoting, or Hyprland version edge cases).
- Window moves that fail are collected; you see up to five failures (then a count). Migration does not stop mid-queue.
- The TUI window is skipped during migration (`org.omarchy.screen-controller` / `screen-layout-tui`).
- **Connected outputs not named** in the active profile’s `monitors` list get a **stderr warning** on apply: their windows are not migrated by this tool (only outputs being removed relative to the profile’s active set are migrated).
- gocui tries **truecolor** output first, then **normal** and **256-color** modes if the terminal rejects the first.
- Unknown CLI words (anything other than `list` / `apply` / `describe`) print usage and exit `2` instead of opening the TUI by mistake.

## License

Personal project; use however you like.
