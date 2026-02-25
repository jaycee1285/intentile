# intentile Smoke Test - 2025-02-19

## What Changed

Executor backend replaced: wtype keybind simulation → direct SartWC IPC.
No more wtype dependency, rc.xml keybinds for snapping, or env var key overrides.
Commands go straight to the compositor over a Unix socket.

---

## Prerequisites

### 1. SartWC running with IPC enabled

```bash
# Verify socket exists
ls $SARTWC_IPC_SOCKET 2>/dev/null || ls $XDG_RUNTIME_DIR/sartwc-$WAYLAND_DISPLAY.sock

# Quick connection test
echo "ping" | socat - UNIX:$SARTWC_IPC_SOCKET
# Expected: OK
```

### 2. Build intentile

```bash
nix develop --command go build -o intentile .
```

---

## Test 1: Daemon Lifecycle

```bash
# Start with debug output
INTENTILE_DEBUG=1 ./intentile daemon &

# Verify running
./intentile status
# Expected: current_ws:1, armed_ws:0, occupancy:(empty)

# Stop daemon
./intentile stop
# Expected: "daemon stopped"

# Verify stopped
./intentile status
# Expected: "daemon not running"
```

**Pass/Fail:** ___
**Notes:**

---

## Test 2: IPC Connection

```bash
# Start daemon
INTENTILE_DEBUG=1 ./intentile daemon &

# Open a test window (foot, kitty, etc.)

# Try atomic placement — this exercises the full IPC path
./intentile 1
```

**Expected:**
- Debug output shows `[executor] ipc: MoveTo x=0 y=0` and `[executor] ipc: ResizeTo width=50% height=100%`
- No errors about socket connection

**If it fails:**
- `failed to connect to SartWC` → socket path wrong, check env vars
- `ERROR unknown action` → compositor doesn't support the action name
- `connection closed` → compositor dropped the connection mid-sequence

**Pass/Fail:** ___
**Notes:**

---

## Test 3: Atomic Mode — The Setup Flow

This is the core use case: stage a workspace from your current one.

**Setup:**
- Be on workspace 1 with some windows open
- Have at least 3 empty workspaces available

**Scenario: Set up a study session on ws2**

```bash
# Launch an editor, throw it to ws2 as right half
foot &  # or whatever
./intentile 2
# Expected: window sent to ws2, snapped to right half (x=50%, w=50%)

# Launch a reader, throw it to ws2 as left half
foot &
./intentile 1
# Expected: window sent to ws2, snapped to left half (x=0, w=50%)

# Check the layout
./intentile status
# Expected: ws2: shape=2 slots=[1,2]
```

Now switch to ws2 manually — both windows should be tiled side by side.

**Pass/Fail:** ___
**Notes:**

---

## Test 4: Two-Stroke Mode (Arm + Slot)

```bash
# Restart fresh
./intentile stop
INTENTILE_DEBUG=1 ./intentile daemon &

# Open a window
foot &

# Arm next workspace with thirds
./intentile arm next 3

# Check armed state
./intentile status
# Expected: armed_ws:2, armed_shape:3

# Place in middle slot
./intentile slot k
```

**Expected:**
- Window sent to ws2, snapped to middle third (x=33%, w=34%)
- Armed state clears after placement
- User stays on ws1

**Verify:**

```bash
./intentile status
# Expected: armed_ws:0, armed_shape:0, ws2: shape=3 slots=[2]
```

**Pass/Fail:** ___
**Notes:**

---

## Test 5: Occupancy & Overflow

```bash
# Fresh daemon, on ws1

# Fill ws2 with halves
foot &
./intentile 1  # ws2, left half

foot &
./intentile 2  # ws2, right half

# ws2 is now full — next placement should overflow
foot &
./intentile 1  # should go to ws3, left half
```

**Verify:**

```bash
./intentile status
# Expected:
#   ws2: shape=2 slots=[1,2]
#   ws3: shape=2 slots=[1]
```

**Pass/Fail:** ___
**Notes:**

---

## Test 6: Shape Locking

```bash
# Fresh daemon

# Lock ws2 to thirds
foot &
./intentile 3  # ws2, left third

# Try halves on same workspace — should overflow
foot &
./intentile 1  # should skip ws2, go to ws3

./intentile status
# Expected: ws2: shape=3, ws3: shape=2
```

**Pass/Fail:** ___
**Notes:**

---

## Test 7: Quarters (Two-Stroke Slots)

```bash
./intentile arm next 4

./intentile slot ij   # upper-left
# Expected: x=0, y=0, w=50%, h=50%

foot &
./intentile arm next 4
./intentile slot kl   # lower-right
# Expected: x=50%, y=50%, w=50%, h=50%
```

**Pass/Fail:** ___
**Notes:**

---

## Test 8: Geometry Spot-Check

Use `list-views` to verify actual window geometry matches expectations.

```bash
echo "list-views" | socat - UNIX:$SARTWC_IPC_SOCKET
```

Compare reported `x`, `y`, `w`, `h` against what the slot should produce.
For a 1920x1080 output:

| Slot | Expected x | Expected w | Expected h |
|------|-----------|-----------|-----------|
| Half left | 0 | ~960 | ~1080 |
| Half right | ~960 | ~960 | ~1080 |
| Third left | 0 | ~634 | ~1080 |
| Third mid | ~634 | ~652 | ~1080 |
| Third right | ~1286 | ~634 | ~1080 |
| Quarter UL | 0 | ~960 | ~540 |
| Quarter LR | ~960 | ~960 | ~540 |

(Values approximate — gaps/decorations will shift things by a few px.)

**Pass/Fail:** ___
**Notes:**

---

## Common Issues

**`failed to connect to SartWC`:**
- Is SartWC running (not plain labwc)?
- Check `echo $SARTWC_IPC_SOCKET` — should be a path
- Check `ls $XDG_RUNTIME_DIR/sartwc-*.sock`

**Window doesn't move to other workspace:**
- Does `echo "SendToDesktop to=2 follow=no" | socat - UNIX:$SARTWC_IPC_SOCKET` work manually?
- Check debug output for ERROR responses

**Window moves but wrong geometry:**
- Does `echo "MoveTo x=0 y=0" | socat - UNIX:$SARTWC_IPC_SOCKET` work?
- Do percentage values work? Try `echo "ResizeTo width=50% height=100%" | socat - UNIX:$SARTWC_IPC_SOCKET`
- If percentages fail for MoveTo, we need to switch to pixel values using output dimensions

**Snap happens but on wrong window:**
- The send-then-snap order depends on the compositor still targeting the sent window for subsequent IPC commands on the same connection. If a different window gets snapped, this is a sequencing issue to investigate.

**Daemon won't start:**
- Check socket: `ls $XDG_RUNTIME_DIR/intentile.sock`
- Remove stale socket: `rm $XDG_RUNTIME_DIR/intentile.sock`
- Check PID file: `cat $XDG_RUNTIME_DIR/intentile.pid`

---

## Success Criteria

- [ ] Daemon starts and stops cleanly
- [ ] IPC connection to SartWC works
- [ ] Atomic mode (1-5, 7-10; 6 reserved) places windows on correct workspace
- [ ] Geometry matches expected slot positions
- [ ] Two-stroke mode (arm + slot) works
- [ ] Occupancy tracking prevents double-stacking
- [ ] Shape mismatch triggers overflow
- [ ] Status command shows accurate state
- [ ] Send-then-snap order works (correct window gets snapped after send)

---

## Next Steps After Smoke Test

If tests pass:
- Daily-drive: real workflows for 1-2 days
- Edge cases: multiple monitors, workspace wrap-around
- Keybind ergonomics: validate FN+A/S/D + J/K/L/I feel right
- Performance: memory usage over time

If tests fail:
- Document failures in the Notes fields above
- Test IPC commands manually with socat to isolate compositor vs intentile issues
- Check debug output (`INTENTILE_DEBUG=1`) for the exact command sequence

---

Last updated: 2025-02-19
