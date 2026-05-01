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

## Usage

### TUI

```bash
./screen-controller
./screen-controller -config /path/to/profiles.yaml
```

Keys: **j/k** or arrows, **Enter** apply, **r** reload YAML, **q** / **Esc** quit.

### CLI

```bash
./screen-controller list
./screen-controller apply dual_sdr
./screen-controller -config ./profiles.yaml apply single_left_sdr
```

## `profiles.yaml`

- **`primary_monitor`**: output name used for documentation (migration uses `safe_workspace`).
- **`safe_workspace`**: workspace id (integer) windows are moved to before an output is disabled — use one pinned to your primary monitor (e.g. `1` on DP-1).
- **`profile_order`**: list of profile keys for TUI ordering.
- **`profiles.<id>.label`**: shown in the TUI.
- **`profiles.<id>.monitors`**: Hyprland `monitor=` lines (`Output,mode,...` or `Output,disable`).

Example:

```yaml
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
- **`apply`** and the **TUI** call **`hyprctl version`** (or use `HYPRLAND_INSTANCE_SIGNATURE`) so you get a clear error if you are not in a Hyprland session.
- If **`hyprctl --batch`** fails, the tool retries **`hyprctl keyword monitor …`** per line. If every line works, the layout still applies (batch quirks, quoting, or Hyprland version edge cases).
- Window moves that fail are collected; you see up to five failures (then a count). Migration does not stop mid-queue.
- The TUI window is skipped during migration (`org.omarchy.screen-controller` / `screen-layout-tui`).
- gocui tries **truecolor** output first, then **normal** and **256-color** modes if the terminal rejects the first.
- Unknown CLI words (anything other than `list` / `apply`) print usage and exit `2` instead of opening the TUI by mistake.

## License

Personal project; use however you like.
