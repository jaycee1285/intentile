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

## Current Direction

- Keep implementation script-first and deterministic.
- Avoid compositor patching for v1.
- Add adapter abstraction only after grammar is validated in daily use.
