# intentile Smoke Test - 2025-02-19

## V1 MVP Complete (7 commits)

Full workflow implemented:
- Socket-based daemon (5.6 MB idle)
- wtype executor backend for LabWC
- Workspace switching with shortest-path calculation
- Occupancy tracking with shape-locking
- Auto-start on first command

---

## Prerequisites

### 1. Install wtype

```bash
nix-env -iA nixpkgs.wtype
# Verify: which wtype
```

### 2. Configure LabWC rc.xml

Add keybinds from `docs/labwc-rc.xml.example` to `~/.config/labwc/rc.xml`

Key sections needed:
- Shape 2: `W-C-h`, `W-C-l` (halves)
- Shape 3: `W-C-j`, `W-C-k`, `W-C-semicolon` (thirds)
- Shape 4: `W-C-u`, `W-C-i`, `W-C-o`, `W-C-p` (quarters)
- Workspace: `W-C-Right`, `W-C-Left`, `W-C-S-Right`, `W-C-S-Left`

### 3. Reconfigure LabWC

```bash
labwc -r
```

---

## Test 1: Daemon Lifecycle

```bash
# Start with debug output
INTENTILE_DEBUG=1 intentile daemon &

# Verify running
intentile status
# Expected: current_ws:1, armed_ws:0, occupancy:(empty)

# Stop daemon
intentile stop
# Expected: "daemon stopped"

# Verify stopped
intentile status
# Expected: "daemon not running"
```

---

## Test 2: Atomic Mode (Single Command)

**Setup:**
- Open a test window (e.g., `kitty` or `firefox`)
- Note current workspace

**Test:**

```bash
# Auto-starts daemon if not running
intentile 1
```

**Expected:**
- Window moves to next workspace
- Window snaps to left half (50% width, full height)
- User stays on original workspace

**Verify:**

```bash
intentile status
# Expected: occupancy shows ws2:shape=2 slots=[1]
```

**Additional atomic tests:**

```bash
# Open new window, test shape 3
intentile 5
# Expected: ws2, shape 3, right third

# Check status
intentile status
# Expected: ERROR - ws2 locked to shape 2, cannot place shape 3
# Should overflow to ws3

# Open new window, test shape 4
intentile 9
# Expected: ws3 (or next available), bottom-right quarter
```

---

## Test 3: Two-Stroke Mode (Arm + Slot)

**Setup:**
- Restart daemon: `intentile stop && INTENTILE_DEBUG=1 intentile daemon &`
- Open test window

**Test:**

```bash
# Arm next workspace with 3-column layout
intentile arm next 3

# Check armed state
intentile status
# Expected: armed_ws:2, armed_shape:3

# Place in middle slot
intentile slot k
```

**Expected:**
- Window moves to workspace 2
- Window snaps to middle third (33% left, 34% width)
- User stays on current workspace
- Armed state clears after placement

**Verify:**

```bash
intentile status
# Expected: armed_ws:0, armed_shape:0, occupancy shows ws2:shape=3 slots=[2]
```

---

## Test 4: Occupancy Tracking & Overflow

**Setup:**
- Fresh daemon
- Workspace 1 active

**Scenario:**

```bash
# Place 2 windows in shape 2 (halves)
# Window 1
intentile 1
# Expected: ws2, left half, occupancy=[1]

# Window 2
intentile 2
# Expected: ws2, right half, occupancy=[1,2]

# Window 3 (workspace should be full)
intentile 1
# Expected: ws3, left half (overflowed to next workspace)
```

**Verify:**

```bash
intentile status
# Expected:
# ws2: shape=2 slots=[1,2]
# ws3: shape=2 slots=[1]
```

---

## Test 5: Shape Locking (v1 behavior)

**Setup:**
- Fresh daemon

**Test:**

```bash
# Lock ws2 to shape 3
intentile 3  # Left third on ws2

# Try to place shape 2 on same workspace
intentile 1  # Should fail or overflow

# Check status
intentile status
# Expected: ws2 locked to shape 3, new placement on ws3
```

---

## Test 6: Slot Token Variations

**Two-stroke with different tokens:**

```bash
# Halves (shape 2)
intentile arm next 2
intentile slot j      # Left
intentile slot l      # Right

# Thirds (shape 3)
intentile arm next 3
intentile slot j      # Left
intentile slot k      # Middle
intentile slot l      # Right

# Quarters (shape 4) - two-stroke input
intentile arm next 4
intentile slot ij     # Upper-left
intentile slot il     # Upper-right
intentile slot kj     # Lower-left
intentile slot kl     # Lower-right
```

---

## Test 7: Environment Variable Overrides

**Setup:**

```bash
# Stop daemon
intentile stop

# Set custom keybind
export INTENTILE_KEY_HALF_LEFT="super+alt+h"

# Start daemon
INTENTILE_DEBUG=1 intentile daemon &

# Test
intentile 1
# Should send super+alt+h instead of super+ctrl+h
```

---

## Common Issues

**Window doesn't move:**
- Check wtype is installed: `which wtype`
- Verify rc.xml keybinds are configured
- Check LabWC was reconfigured: `labwc -r`
- Look for errors in debug output

**Wrong workspace:**
- Check current_ws tracking in status
- Verify SendToDesktop keybinds in rc.xml

**Wrong geometry:**
- Check MoveTo/ResizeTo actions in rc.xml
- Verify percentage values (50%, 33%, etc.)

**Daemon won't start:**
- Check socket: `ls $XDG_RUNTIME_DIR/intentile.sock`
- Remove stale socket: `rm $XDG_RUNTIME_DIR/intentile.sock`
- Check PID file: `cat $XDG_RUNTIME_DIR/intentile.pid`

---

## Debug Commands

```bash
# Watch daemon output
tail -f /tmp/intentile-debug.log  # If redirected

# Check wtype manually
wtype -M super -M ctrl -k h -m ctrl -m super
# Should snap focused window to left half

# Check LabWC action manually
# (No direct way to trigger actions, must use keybinds)
```

---

## Success Criteria

- [ ] Daemon starts and stops cleanly
- [ ] Atomic mode (1-9) places windows correctly
- [ ] Two-stroke mode (arm + slot) works
- [ ] Workspace switching moves windows to correct workspace
- [ ] Occupancy tracking prevents overlaps
- [ ] Shape mismatch triggers overflow to next workspace
- [ ] Status command shows accurate state
- [ ] Memory usage stays under 10 MB

---

## Next Steps After Smoke Test

If tests pass:
- Daily-drive test: Real workflows for 1-2 days
- Performance check: Memory usage over time
- Edge cases: Multiple monitors, workspace wrap-around
- Keybind ergonomics: Validate FN+A/S/D + J/K/L/I feel right

If tests fail:
- Document failures in this file
- Check debug output for error messages
- Verify wtype/LabWC integration
- Test keybinds manually to isolate issue

---

Last updated: 2025-02-19
