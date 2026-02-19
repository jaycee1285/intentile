# Intentile Taskboard

## V1 (Script MVP)

- [ ] Implement intent state in script: `arm`, `shape`, `slot`, `clear`
- [ ] Add placement command: `place <target> <spec>` (`2j`, `3k`, `4lr`, etc.)
- [ ] Add quarter regions + internal key mapping (`ul/ur/ll/lr`)
- [ ] Wire ergonomic binds in `rc.xml`: `A-S-a/s/d` + right-hand slot keys
- [ ] Add TTL + cancellation behavior for non-blocking armed state
- [ ] Add drift-safe commands: `sync`, `clear`, `reset`, plus sanity output
- [ ] Daily-drive test pass: 3 common flows (half split, thirds, next-workspace quarter)

## V2 (Systemization)

- [ ] Add occupancy model per workspace/shape (slot-aware, not just count-based)
- [ ] Implement overflow policy options (`next`, `prev`, `new-workspace`, `fail`)
- [ ] Add `reconcile` command to rebuild slot state from current visible windows
- [ ] Add optional two-stroke quarter input grammar (`ij/il/kj/kl` strict mode)
- [ ] Add lightweight status OSD/notify integration for armed/shape/slot state
- [ ] Define adapter boundary for Wayfire backend (same intent grammar, new executor)
- [ ] Write minimal integration tests for parser + state transitions
