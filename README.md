# intentile

Intent-first autotiling for stacking compositors.

Core idea:
- left hand sets transport + layout intent (workspace direction + 2/3/4)
- right hand sets slot placement
- overflow policy handles "no room" without manual drag choreography

Primary target:
- LabWC (script-first MVP)

Secondary target:
- Wayfire adapter once intent grammar is stable

## Repo Layout

- `scripts/labwc-niri`: working baseline script (copied from local config)
- `TASKBOARD.md`: v1/v2 execution plan
- `docs/KEYMAP.md`: ergonomic key grammar and rc.xml mapping
- `docs/borrowed/`: source snapshots from repos used for design
- `docs/digtwin/`: copied process docs for agent collaboration discipline

## Installation

### Build from source

```bash
nix develop
go build -o intentile
```

### Install wtype (required runtime dependency)

```bash
# NixOS
nix-env -iA nixpkgs.wtype

# Or add to system packages
```

### Configure LabWC

Add the keybinds from `docs/labwc-rc.xml.example` to `~/.config/labwc/rc.xml`

## Usage

### Start daemon

```bash
intentile daemon &
# Or let it auto-start on first command
```

### Two-stroke mode (arm + slot)

```bash
# Arm next workspace with 3-column layout
intentile arm next 3

# Place focused window in middle slot
intentile slot k
```

### Atomic mode (single command)

```bash
intentile 1    # Next workspace, left half
intentile 5    # Next workspace, shape 3, right third
intentile 9    # Next workspace, bottom-right quarter
```

### Status and control

```bash
intentile status   # Show daemon state
intentile stop     # Stop daemon
```

## Environment Variables

Customize keybinds by setting environment variables before starting daemon:

```bash
export INTENTILE_KEY_HALF_LEFT="super+alt+h"
export INTENTILE_KEY_THIRD_MID="super+alt+k"
# ... etc (see internal/executor/labwc.go for full list)
```

## Current Status

- ✅ Socket-based daemon with occupancy tracking
- ✅ wtype executor backend for LabWC
- ✅ Workspace switching (relative movement)
- ✅ Shape-locked workspaces (v1)
- 🚧 Testing needed in real LabWC environment
