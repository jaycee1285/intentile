package occupancy

import (
	"fmt"
	"sync"
)

// Tracker manages workspace occupancy
type Tracker struct {
	mu         sync.RWMutex
	workspaces map[int]*Workspace // workspace index -> workspace state
}

// Workspace tracks occupancy for a single workspace
type Workspace struct {
	Index       int            // Workspace index (1-based)
	Shape       int            // Locked shape (2, 3, or 4), 0 if empty
	OccupiedSlots map[int]bool // Slot numbers that are filled
}

// NewTracker creates an occupancy tracker
func NewTracker() *Tracker {
	return &Tracker{
		workspaces: make(map[int]*Workspace),
	}
}

// CanPlace checks if a slot can be placed in the workspace
func (t *Tracker) CanPlace(wsIndex, shape, slot int) (bool, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ws, exists := t.workspaces[wsIndex]

	// Empty workspace - can always place
	if !exists || ws.Shape == 0 {
		return true, ""
	}

	// Shape mismatch - v1 doesn't support mixed shapes
	if ws.Shape != shape {
		return false, fmt.Sprintf("workspace %d locked to shape %d, cannot place shape %d", wsIndex, ws.Shape, shape)
	}

	// Check if slot is already occupied
	if ws.OccupiedSlots[slot] {
		return false, fmt.Sprintf("workspace %d slot %d already occupied", wsIndex, slot)
	}

	return true, ""
}

// Place marks a slot as occupied
func (t *Tracker) Place(wsIndex, shape, slot int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ws, exists := t.workspaces[wsIndex]
	if !exists {
		ws = &Workspace{
			Index:         wsIndex,
			Shape:         shape,
			OccupiedSlots: make(map[int]bool),
		}
		t.workspaces[wsIndex] = ws
	}

	// Lock workspace to shape on first placement
	if ws.Shape == 0 {
		ws.Shape = shape
	}

	ws.OccupiedSlots[slot] = true
	return nil
}

// IsFull checks if workspace is full for the given shape
func (t *Tracker) IsFull(wsIndex, shape int) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ws, exists := t.workspaces[wsIndex]
	if !exists {
		return false
	}

	// Shape mismatch means it's "full" (can't add different shape in v1)
	if ws.Shape != 0 && ws.Shape != shape {
		return true
	}

	// Count max slots for shape
	maxSlots := getMaxSlots(shape)
	return len(ws.OccupiedSlots) >= maxSlots
}

// FindAvailableWorkspace finds the next workspace that can accept the shape
func (t *Tracker) FindAvailableWorkspace(startWS, shape, maxWS int) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for offset := 0; offset < maxWS; offset++ {
		wsIndex := ((startWS - 1 + offset) % maxWS) + 1

		ws, exists := t.workspaces[wsIndex]

		// Empty workspace
		if !exists || ws.Shape == 0 {
			return wsIndex
		}

		// Same shape and not full
		if ws.Shape == shape && len(ws.OccupiedSlots) < getMaxSlots(shape) {
			return wsIndex
		}
	}

	// Fallback: create new workspace
	return maxWS + 1
}

// Clear removes a workspace from tracking
func (t *Tracker) Clear(wsIndex int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.workspaces, wsIndex)
}

// GetState returns current workspace state (for debugging)
func (t *Tracker) GetState(wsIndex int) *Workspace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ws, exists := t.workspaces[wsIndex]
	if !exists {
		return &Workspace{Index: wsIndex, Shape: 0, OccupiedSlots: make(map[int]bool)}
	}

	// Return a copy to avoid race conditions
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

// GetAllWorkspaces returns a map of all workspaces (for status/debugging)
func (t *Tracker) GetAllWorkspaces() map[int]*Workspace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Return a deep copy to avoid race conditions
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

func getMaxSlots(shape int) int {
	switch shape {
	case 2:
		return 2
	case 3:
		return 3
	case 4:
		return 4
	default:
		return 0
	}
}
