# Intentile Taskboard

## V1 (Script MVP) — Done

- [x] Implement intent state in script: `arm`, `shape`, `slot`, `clear`
- [x] Add quarter regions + internal key mapping (`ul/ur/ll/lr`)
- [x] Add TTL + cancellation behavior for non-blocking armed state
- [x] Socket-based daemon with auto-start
- [x] Occupancy tracking with shape-locking + overflow
- [x] SartWC IPC executor (replaced wtype)

## V2 (Mixed-Shape Placement + State)

### Collision detection

Side-tag collision model for mixed-shape workspaces. Each atomic slot maps to a screen side:

```
Slot  Shape       Position     Side
1     half        left         L
2     half        right        R
3     third       left         L
4     third       middle       C
5     third       right        R
7     UL quarter  upper-left   L
8     UR quarter  upper-right  R
9     LL quarter  lower-left   L
10    LR quarter  lower-right  R
```

(Slot 6 is reserved for future behavior; quarters use 7-10.)

**Collision rules:**
1. If workspace contains slot 4 (center third), only slots 3 and 5 are legal neighbors
2. Otherwise: same side-tag = collision → overflow to next workspace
3. No collision = place

This replaces V1's shape-locking with actual spatial awareness while staying a short conditional, no geometric rect math needed.

### Tasks

- [x] Implement side-tag collision model (replaces shape-locking in occupancy tracker)
- [ ] Mixed-shape layouts: allow e.g. `1+8` (left half + upper-right quarter) or `2+7` (right half + upper-left quarter) on same workspace
- [ ] Overflow policy options (`next`, `prev`, `new-workspace`, `fail`)
- [ ] `reconcile` command: rebuild slot state from `list-views` geometry
- [ ] Compositor-sourced state: query `list-views` for ground truth instead of maintaining parallel tracker. Infer slot from window geometry (~10px tolerance).
- [ ] Current-workspace tiling mode: `FN+Alt+A/S/D` → `intentile arm here 2/3/4`. Same slot keys, skip send, just snap.
- [ ] Two-stroke quarter input grammar (`ij/il/kj/kl` strict mode)
- [ ] Integration tests for collision logic + state transitions

## V3 (Polish)

- [ ] Lightweight status OSD/notify integration for armed/shape/slot state
- [ ] Define adapter boundary for Wayfire backend (same intent grammar, new executor)
