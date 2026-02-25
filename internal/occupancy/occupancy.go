package occupancy

import (
	"fmt"
	"sync"
)

// Tracker manages workspace occupancy.
type Tracker struct {
	mu         sync.RWMutex
	workspaces map[int]*Workspace // workspace index -> workspace state
}

// Workspace tracks occupancy for a single workspace.
type Workspace struct {
	Index         int          // Workspace index (1-based)
	Shape         int          // Debug-only: 0 empty, -1 mixed, 2/3/4 homogeneous
	OccupiedSlots map[int]bool // Atomic slots (1,2,3,4,5,7,8,9,10)
}

// NewTracker creates an occupancy tracker.
func NewTracker() *Tracker {
	return &Tracker{
		workspaces: make(map[int]*Workspace),
	}
}

// CanPlace checks if a slot can be placed in the workspace.
func (t *Tracker) CanPlace(wsIndex, shape, slot int) (bool, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.canPlaceLocked(wsIndex, shape, slot)
}

func (t *Tracker) canPlaceLocked(wsIndex, shape, slot int) (bool, string) {
	atomic, ok := atomicSlotID(shape, slot)
	if !ok {
		return false, fmt.Sprintf("unsupported placement shape=%d slot=%d", shape, slot)
	}

	ws, exists := t.workspaces[wsIndex]
	if !exists || len(ws.OccupiedSlots) == 0 {
		return true, ""
	}

	return canPlaceAtomic(wsIndex, ws, atomic)
}

func canPlaceAtomic(wsIndex int, ws *Workspace, atomic int) (bool, string) {
	if ws.OccupiedSlots[atomic] {
		return false, fmt.Sprintf("workspace %d slot %d already occupied", wsIndex, atomic)
	}

	// Special rule: center third (slot 4) only coexists with thirds 3/5.
	if ws.OccupiedSlots[4] && atomic != 3 && atomic != 5 {
		return false, fmt.Sprintf("workspace %d contains center third (slot 4); only slots 3 and 5 are legal neighbors", wsIndex)
	}
	if atomic == 4 {
		for existing := range ws.OccupiedSlots {
			if existing != 3 && existing != 5 {
				return false, fmt.Sprintf("slot 4 (center third) conflicts with existing slot %d on workspace %d", existing, wsIndex)
			}
		}
		return true, ""
	}

	inSide, ok := sideTag(atomic)
	if !ok {
		return false, fmt.Sprintf("no side-tag mapping for atomic slot %d", atomic)
	}

	for existing := range ws.OccupiedSlots {
		if existing == 4 {
			continue // handled above
		}
		side, ok := sideTag(existing)
		if !ok {
			continue
		}
		if side == inSide {
			return false, fmt.Sprintf("workspace %d side collision: slot %d (%s) conflicts with slot %d (%s)", wsIndex, atomic, inSide, existing, side)
		}
	}

	return true, ""
}

// Place marks a slot as occupied.
func (t *Tracker) Place(wsIndex, shape, slot int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	atomic, ok := atomicSlotID(shape, slot)
	if !ok {
		return fmt.Errorf("unsupported placement shape=%d slot=%d", shape, slot)
	}

	ws, exists := t.workspaces[wsIndex]
	if !exists {
		ws = &Workspace{
			Index:         wsIndex,
			Shape:         0,
			OccupiedSlots: make(map[int]bool),
		}
		t.workspaces[wsIndex] = ws
	}

	ws.OccupiedSlots[atomic] = true

	// Keep Shape as a debug hint only.
	if len(ws.OccupiedSlots) == 1 {
		ws.Shape = shape
	} else if ws.Shape != shape {
		ws.Shape = -1
	}

	return nil
}

// IsFull checks whether no supported atomic placement can be added.
func (t *Tracker) IsFull(wsIndex, shape int) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ws, exists := t.workspaces[wsIndex]
	if !exists || len(ws.OccupiedSlots) == 0 {
		return false
	}

	for _, atomic := range []int{1, 2, 3, 4, 5, 7, 8, 9, 10} {
		if can, _ := canPlaceAtomic(wsIndex, ws, atomic); can {
			return false
		}
	}
	return true
}

// FindAvailableWorkspace finds the next workspace that can accept the shape/slot.
func (t *Tracker) FindAvailableWorkspace(startWS, shape, slot, maxWS int) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for offset := 0; offset < maxWS; offset++ {
		wsIndex := ((startWS - 1 + offset) % maxWS) + 1
		if can, _ := t.canPlaceLocked(wsIndex, shape, slot); can {
			return wsIndex
		}
	}

	// Fallback: create new workspace (caller may clamp policy later)
	return maxWS + 1
}

// Clear removes a workspace from tracking.
func (t *Tracker) Clear(wsIndex int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.workspaces, wsIndex)
}

// GetState returns current workspace state (for debugging).
func (t *Tracker) GetState(wsIndex int) *Workspace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ws, exists := t.workspaces[wsIndex]
	if !exists {
		return &Workspace{Index: wsIndex, Shape: 0, OccupiedSlots: make(map[int]bool)}
	}

	copy := &Workspace{
		Index:         ws.Index,
		Shape:         ws.Shape,
		OccupiedSlots: make(map[int]bool),
	}
	for slot := range ws.OccupiedSlots {
		copy.OccupiedSlots[slot] = true
	}
	return copy
}

// GetAllWorkspaces returns a map of all workspaces (for status/debugging).
func (t *Tracker) GetAllWorkspaces() map[int]*Workspace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[int]*Workspace)
	for idx, ws := range t.workspaces {
		copy := &Workspace{
			Index:         ws.Index,
			Shape:         ws.Shape,
			OccupiedSlots: make(map[int]bool),
		}
		for slot := range ws.OccupiedSlots {
			copy.OccupiedSlots[slot] = true
		}
		result[idx] = copy
	}
	return result
}

func atomicSlotID(shape, slot int) (int, bool) {
	switch shape {
	case 2:
		if slot >= 1 && slot <= 2 {
			return slot, true
		}
	case 3:
		if slot >= 1 && slot <= 3 {
			return slot + 2, true // 3,4,5
		}
	case 4:
		if slot >= 1 && slot <= 4 {
			return slot + 6, true // 7,8,9,10 (6 reserved)
		}
	}
	return 0, false
}

func sideTag(atomic int) (string, bool) {
	switch atomic {
	case 1, 3, 7, 9:
		return "L", true
	case 2, 5, 8, 10:
		return "R", true
	case 4:
		return "C", true
	default:
		return "", false
	}
}
