# Keymap (Ergonomic Intent Grammar)

## Mental Model

**"Setting out a picnic blanket, then directing seating"**

1. **Left hand (FN + A/S/D)**: Choose destination workspace + layout size
2. **Right hand (J/K/L/I)**: Choose specific slot placement

The two-stroke pattern keeps your hands in home row position and avoids modifier pileups.

---

## Primary Bindings

### Left Hand: Arm Workspace + Shape

**Function + Letter** (sets intent for next window placement):

- `FN+A` → Arm next workspace, shape 2 (halves)
- `FN+S` → Arm next workspace, shape 3 (thirds)
- `FN+D` → Arm next workspace, shape 4 (quarters)

Triggers: `intentile arm next 2|3|4`

State persists until slot placement occurs.

---

### Right Hand: Slot Placement

**After arming with left hand**, press slot key to place focused window:

#### Shape 2 (Halves)
```
┌─────┬─────┐
│  j  │  l  │
│     │     │
└─────┴─────┘
```
- `j` → Left half
- `l` → Right half

#### Shape 3 (Thirds)
```
┌───┬───┬───┐
│ j │ k │ l │
│   │   │   │
└───┴───┴───┘
```
- `j` → Left third
- `k` → Middle third
- `l` → Right third

#### Shape 4 (Quarters) - Two-Stroke
```
┌─────┬─────┐
│ ij  │ il  │  ← i (top row)
├─────┼─────┤
│ kj  │ kl  │  ← k (bottom row)
└─────┴─────┘
  ↑     ↑
  j     l
 (left)(right)
```

**First key** (row): `i` (top) or `k` (bottom)
**Second key** (column): `j` (left) or `l` (right)

- `ij` → Upper-left (UL)
- `il` → Upper-right (UR)
- `kj` → Lower-left (LL)
- `kl` → Lower-right (LR)

**Visual mnemonic**: IJKL forms an arrow-key diamond on QWERTY:
```
    I (up/top)
J K L (left, down/bottom, right)
```

---

## Atomic Mode (Advanced)

Single-command placement without state:

**Number keys** map directly to slots:
```
Shape 2 (halves):    1 (left), 2 (right)
Shape 3 (thirds):    3 (left), 4 (mid), 5 (right)
Shape 4 (quarters):  6 (UL), 7 (UR), 8 (LL), 9 (LR)
```

**Example**:
- `intentile 5` → Arm next workspace, shape 3, place in right third (atomic)

Shape is inferred from the number (1-2 = shape 2, 3-5 = shape 3, 6-9 = shape 4).

---

## Why This Mapping

✅ **Ergonomic**: Hands stay near home row, no modifier pileups
✅ **Consistent**: J/K/L positions mirror their spatial meaning
✅ **Fast under fatigue**: Two clear strokes, no complex chords
✅ **Shape-aware**: Same keys (JKL) work across shapes, plus I/K for quarters
✅ **No tarantula hands**: Tested to avoid awkward finger gymnastics

---

## Compositor Integration (LabWC rc.xml example)

```xml
<!-- Left hand: Arm + Shape -->
<keybind key="Fn-a">
  <action name="Execute" command="intentile arm next 2" />
</keybind>
<keybind key="Fn-s">
  <action name="Execute" command="intentile arm next 3" />
</keybind>
<keybind key="Fn-d">
  <action name="Execute" command="intentile arm next 4" />
</keybind>

<!-- Right hand: Slot placement (shape 2) -->
<keybind key="j">
  <action name="Execute" command="intentile slot j" />
</keybind>
<keybind key="l">
  <action name="Execute" command="intentile slot l" />
</keybind>

<!-- Right hand: Slot placement (shape 3) -->
<keybind key="k">
  <action name="Execute" command="intentile slot k" />
</keybind>

<!-- Right hand: Slot placement (shape 4, two-stroke) -->
<keybind key="i">
  <action name="Execute" command="intentile slot i" />
</keybind>
<!-- Note: Shape 4 requires sequence detection in intentile daemon -->
```

---

## State Flow

1. User presses `FN+S` → `intentile arm next 3` (state: armed=next, shape=3)
2. User presses `k` → `intentile slot k` (moves focused window to next workspace, middle third)
3. State clears after placement

If user doesn't press slot key, state persists until next arm/clear command.

---

## Future: Mixed-Shape Layouts (V2)

Eventually, workspaces should support mixed shapes on the same canvas.

**Example**: Left half + two right quarters:
```
 _________
|   |   |
|   |---|
|___|___|
```

Achieved by firing: `intentile 1` → `intentile 7` → `intentile 9`

This treats the workspace as a 2x2 grid where slots can be subdivided. Slots are addressable by absolute position rather than shape-relative indexing.

**For v1**: Workspaces are shape-locked (first window placement determines workspace shape, subsequent windows must match that shape or overflow to next workspace).
