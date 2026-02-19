package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Manager handles intent state persistence
type Manager struct {
	StateDir string
	ArmTTL   time.Duration
	ShapeTTL time.Duration
}

// State represents the current intent state
type State struct {
	CurrentWS   int
	ArmedWS     int
	ArmedTime   time.Time
	Shape       int
	ShapeTime   time.Time
	MaxWS       int
}

// NewManager creates a state manager
func NewManager(stateDir string, armTTL, shapeTTL time.Duration) *Manager {
	if stateDir == "" {
		cacheHome := os.Getenv("XDG_CACHE_HOME")
		if cacheHome == "" {
			cacheHome = filepath.Join(os.Getenv("HOME"), ".cache")
		}
		stateDir = filepath.Join(cacheHome, "intentile")
	}

	return &Manager{
		StateDir: stateDir,
		ArmTTL:   armTTL,
		ShapeTTL: shapeTTL,
	}
}

// Initialize ensures state directory exists
func (m *Manager) Initialize() error {
	return os.MkdirAll(m.StateDir, 0755)
}

// GetState reads current state from disk
func (m *Manager) GetState() (*State, error) {
	state := &State{}

	// Read current workspace
	ws, err := m.readInt("current_ws")
	if err != nil {
		ws = 1
	}
	state.CurrentWS = ws

	// Read armed workspace with timestamp
	armedWS, armedTime, err := m.readIntWithTime("arm_ws", "arm_ts")
	if err == nil && time.Since(armedTime) <= m.ArmTTL {
		state.ArmedWS = armedWS
		state.ArmedTime = armedTime
	}

	// Read shape with timestamp
	shape, shapeTime, err := m.readIntWithTime("shape", "shape_ts")
	if err == nil && time.Since(shapeTime) <= m.ShapeTTL {
		state.Shape = shape
		state.ShapeTime = shapeTime
	}

	return state, nil
}

// SetCurrentWS writes current workspace
func (m *Manager) SetCurrentWS(ws int) error {
	return m.writeInt("current_ws", ws)
}

// ArmWorkspace sets armed workspace with timestamp
func (m *Manager) ArmWorkspace(ws int) error {
	if err := m.writeInt("arm_ws", ws); err != nil {
		return err
	}
	return m.writeTime("arm_ts", time.Now())
}

// SetShape sets shape with timestamp
func (m *Manager) SetShape(shape int) error {
	if err := m.writeInt("shape", shape); err != nil {
		return err
	}
	return m.writeTime("shape_ts", time.Now())
}

// ClearArm clears armed state
func (m *Manager) ClearArm() error {
	_ = m.remove("arm_ws")
	_ = m.remove("arm_ts")
	return nil
}

// ClearShape clears shape state
func (m *Manager) ClearShape() error {
	_ = m.remove("shape")
	_ = m.remove("shape_ts")
	return nil
}

// ClearIntent clears all intent state
func (m *Manager) ClearIntent() error {
	_ = m.ClearArm()
	_ = m.ClearShape()
	return nil
}

// Helper methods for file I/O

func (m *Manager) readInt(name string) (int, error) {
	data, err := os.ReadFile(filepath.Join(m.StateDir, name))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (m *Manager) readIntWithTime(intName, timeName string) (int, time.Time, error) {
	val, err := m.readInt(intName)
	if err != nil {
		return 0, time.Time{}, err
	}

	ts, err := m.readTime(timeName)
	if err != nil {
		return 0, time.Time{}, err
	}

	return val, ts, nil
}

func (m *Manager) readTime(name string) (time.Time, error) {
	data, err := os.ReadFile(filepath.Join(m.StateDir, name))
	if err != nil {
		return time.Time{}, err
	}

	ms, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(0, ms*int64(time.Millisecond)), nil
}

func (m *Manager) writeInt(name string, val int) error {
	path := filepath.Join(m.StateDir, name)
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", val)), 0644)
}

func (m *Manager) writeTime(name string, t time.Time) error {
	path := filepath.Join(m.StateDir, name)
	ms := t.UnixNano() / int64(time.Millisecond)
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", ms)), 0644)
}

func (m *Manager) remove(name string) error {
	path := filepath.Join(m.StateDir, name)
	return os.Remove(path)
}
