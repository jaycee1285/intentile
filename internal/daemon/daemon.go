package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/jaycee1285/intentile/internal/executor"
	"github.com/jaycee1285/intentile/internal/occupancy"
	"github.com/jaycee1285/intentile/internal/state"
)

// Daemon manages intentile's long-running state and command processing
type Daemon struct {
	mu         sync.RWMutex
	state      *state.Manager
	occupancy  *occupancy.Tracker
	executor   *executor.LabWCExecutor
	currentWS  int
	armedWS    int
	armedShape int
	armedTime  time.Time
	maxWS      int
	debug      bool
}

// Config holds daemon configuration
type Config struct {
	StateDir string
	MaxWS    int
	ArmTTL   time.Duration
	ShapeTTL time.Duration
	Debug    bool
}

// NewDaemon creates a daemon instance
func NewDaemon(cfg Config) *Daemon {
	if cfg.MaxWS == 0 {
		cfg.MaxWS = 12 // Default from shell script
	}
	if cfg.ArmTTL == 0 {
		cfg.ArmTTL = 900 * time.Millisecond
	}
	if cfg.ShapeTTL == 0 {
		cfg.ShapeTTL = 1400 * time.Millisecond
	}

	return &Daemon{
		state:     state.NewManager(cfg.StateDir, cfg.ArmTTL, cfg.ShapeTTL),
		occupancy: occupancy.NewTracker(),
		executor:  executor.NewLabWCExecutor(cfg.Debug),
		currentWS: 1,
		maxWS:     cfg.MaxWS,
		debug:     cfg.Debug,
	}
}

// Start initializes the daemon
func (d *Daemon) Start(ctx context.Context) error {
	if err := d.state.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}
	if err := d.syncCompositorState(); err != nil {
		d.notify(fmt.Sprintf("compositor sync failed: %v", err))
	}
	if err := d.Reconcile(); err != nil {
		d.notify(fmt.Sprintf("occupancy reconcile failed: %v", err))
	}

	go d.watchCompositorEvents(ctx)

	d.notify("intentile daemon started")
	return nil
}

func (d *Daemon) syncCompositorState() error {
	current, maxWS, err := d.executor.QueryWorkspaceState()
	if err != nil {
		return err
	}

	d.mu.Lock()
	if current > 0 {
		d.currentWS = current
	}
	if maxWS > 0 {
		d.maxWS = maxWS
	}
	d.mu.Unlock()

	if current > 0 {
		_ = d.state.SetCurrentWS(current)
	}
	return nil
}

func (d *Daemon) watchCompositorEvents(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		err := d.executor.SubscribeEvents(ctx, d.handleCompositorEvent)
		if ctx.Err() != nil {
			return
		}
		d.notify(fmt.Sprintf("event stream disconnected: %v", err))

		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (d *Daemon) handleCompositorEvent(ev executor.IPCEvent) {
	switch ev.Name {
	case "workspace-list-changed":
		if err := d.syncCompositorState(); err != nil {
			d.notify(fmt.Sprintf("workspace state sync failed: %v", err))
			return
		}
		if err := d.Reconcile(); err != nil {
			d.notify(fmt.Sprintf("occupancy reconcile failed: %v", err))
		}
	case "workspace-changed", "focus-changed":
		cur, err := strconv.Atoi(ev.Fields["current"])
		if err != nil || cur < 1 {
			return
		}
		d.setCurrentWorkspace(cur)
	case "view-mapped", "view-unmapped":
		if err := d.Reconcile(); err != nil {
			d.notify(fmt.Sprintf("occupancy reconcile failed: %v", err))
		}
	}
}

// Arm sets the armed workspace and shape
func (d *Daemon) Arm(target string, shape int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	ws, err := d.resolveTarget(target)
	if err != nil {
		return err
	}

	d.armedWS = ws
	d.armedShape = shape
	d.armedTime = time.Now()

	d.notify(fmt.Sprintf("ARM ws:%d shape:%d", ws, shape))
	return nil
}

// Slot places focused window in the armed slot
func (d *Daemon) Slot(slotToken string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if armed state is valid
	if d.armedShape == 0 {
		return fmt.Errorf("no active shape (run 'arm' first)")
	}

	// TODO: Check if armed state has expired
	if time.Since(d.armedTime) > 1400*time.Millisecond {
		d.armedShape = 0
		return fmt.Errorf("armed state expired")
	}

	// Normalize slot token to slot number
	slot, err := d.normalizeSlot(d.armedShape, slotToken)
	if err != nil {
		return err
	}

	// Determine target workspace
	targetWS := d.armedWS
	if targetWS == 0 {
		targetWS = d.currentWS
	}

	// Check if workspace can accept this placement
	canPlace, reason := d.occupancy.CanPlace(targetWS, d.armedShape, slot)
	if !canPlace {
		// Find next available workspace
		targetWS = d.occupancy.FindAvailableWorkspace(targetWS, d.armedShape, slot, d.maxWS)
		d.notify(fmt.Sprintf("Overflow: %s, moving to ws:%d", reason, targetWS))
	}
	if targetWS > d.maxWS {
		if err := d.ensureWorkspaceExistsLocked(targetWS); err != nil {
			d.notify(fmt.Sprintf("PLACE ERROR (workspace-create): %v", err))
			return fmt.Errorf("failed to create workspace %d: %w", targetWS, err)
		}
	}

	// Send window to target workspace if different from current
	if targetWS != d.currentWS {
		if err := d.executor.SendToWorkspace(d.currentWS, targetWS, d.maxWS); err != nil {
			d.notify(fmt.Sprintf("PLACE ERROR (workspace): %v", err))
			return fmt.Errorf("failed to send to workspace: %w", err)
		}
	}

	// Snap window into its slot on the target workspace
	if err := d.executor.SnapToSlot(d.armedShape, slot); err != nil {
		d.notify(fmt.Sprintf("PLACE ERROR (snap): %v", err))
		return fmt.Errorf("failed to snap window: %w", err)
	}

	// Place in occupancy tracker
	if err := d.occupancy.Place(targetWS, d.armedShape, slot); err != nil {
		return err
	}

	d.notify(fmt.Sprintf("PLACE ws:%d shape:%d slot:%d", targetWS, d.armedShape, slot))

	// Clear armed state
	d.armedWS = 0
	d.armedShape = 0

	return nil
}

// PlaceAtomic performs atomic placement (number key mode)
func (d *Daemon) PlaceAtomic(slotNum int) error {
	// Infer shape from slot number
	shape, slot := d.inferShapeAndSlot(slotNum)
	if shape == 0 {
		return fmt.Errorf("invalid slot number: %d (expected 1-5 or 7-10; 6 is reserved)", slotNum)
	}

	// Arm next workspace with inferred shape
	if err := d.Arm("next", shape); err != nil {
		return err
	}

	// Place in the slot
	slotToken := d.slotNumberToToken(shape, slot)
	return d.Slot(slotToken)
}

// WorkspaceAdd creates a workspace via compositor IPC and resyncs local state.
func (d *Daemon) WorkspaceAdd(name string) error {
	if err := d.executor.WorkspaceAdd(name); err != nil {
		return err
	}
	if err := d.syncCompositorState(); err != nil {
		return err
	}
	return d.Reconcile()
}

// WorkspaceRemoveLast removes the last (highest-index) workspace.
func (d *Daemon) WorkspaceRemoveLast() error {
	d.mu.Lock()
	index := d.maxWS
	d.mu.Unlock()
	if index < 2 {
		return fmt.Errorf("cannot remove last workspace")
	}
	return d.WorkspaceRemove(index)
}

// WorkspaceRemove removes a workspace by 1-based index and resyncs local state.
func (d *Daemon) WorkspaceRemove(index int) error {
	if err := d.executor.WorkspaceRemove(index); err != nil {
		return err
	}
	if err := d.syncCompositorState(); err != nil {
		return err
	}
	return d.Reconcile()
}

// WorkspaceRename renames a workspace and resyncs local state.
func (d *Daemon) WorkspaceRename(index int, name string) error {
	if err := d.executor.WorkspaceRename(index, name); err != nil {
		return err
	}
	if err := d.syncCompositorState(); err != nil {
		return err
	}
	return d.Reconcile()
}

// Reconcile rebuilds occupancy from the compositor's current mapped/tiled views.
func (d *Daemon) Reconcile() error {
	snapshot, err := d.executor.QueryViewsState()
	if err != nil {
		return err
	}

	rebuilt := occupancy.NewTracker()
	placed := 0

	for _, view := range snapshot.Views {
		if view.Workspace < 1 {
			continue
		}
		if view.Minimized || view.Fullscreen || view.Maximized || !view.Tiled {
			continue
		}

		shape, slot, ok := inferPlacementFromView(view)
		if !ok {
			continue
		}

		if err := rebuilt.Place(view.Workspace, shape, slot); err != nil {
			if d.debug {
				fmt.Fprintf(os.Stderr, "[intentile] reconcile skip ws:%d shape:%d slot:%d: %v\n", view.Workspace, shape, slot, err)
			}
			continue
		}
		placed++
	}

	d.mu.Lock()
	d.occupancy = rebuilt
	if snapshot.CurrentWorkspace > 0 {
		d.currentWS = snapshot.CurrentWorkspace
	}
	d.mu.Unlock()

	if snapshot.CurrentWorkspace > 0 {
		_ = d.state.SetCurrentWS(snapshot.CurrentWorkspace)
	}
	if d.debug {
		fmt.Fprintf(os.Stderr, "[intentile] reconcile: placed=%d mapped=%d\n", placed, len(snapshot.Views))
	}
	return nil
}

// Clear clears armed state
func (d *Daemon) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.armedWS = 0
	d.armedShape = 0
	d.notify("CLEAR")
	return nil
}

// Helper methods

func (d *Daemon) resolveTarget(target string) (int, error) {
	switch target {
	case "current", "here":
		return d.currentWS, nil
	case "next", "right":
		return (d.currentWS % d.maxWS) + 1, nil
	case "prev", "left":
		prev := d.currentWS - 1
		if prev < 1 {
			prev = d.maxWS
		}
		return prev, nil
	default:
		return 0, fmt.Errorf("invalid target: %s", target)
	}
}

func (d *Daemon) normalizeSlot(shape int, token string) (int, error) {
	// Map slot tokens to slot numbers
	// For simplicity, using 1-based indexing matching the atomic mode
	switch shape {
	case 2: // halves: j=1, l=2
		switch token {
		case "j", "left":
			return 1, nil
		case "l", "right":
			return 2, nil
		}
	case 3: // thirds: j=1, k=2, l=3
		switch token {
		case "j", "left":
			return 1, nil
		case "k", "mid", "middle":
			return 2, nil
		case "l", "right":
			return 3, nil
		}
	case 4: // quarters: ij=1, il=2, kj=3, kl=4
		switch token {
		case "ij", "ul":
			return 1, nil
		case "il", "ur":
			return 2, nil
		case "kj", "ll":
			return 3, nil
		case "kl", "lr":
			return 4, nil
		}
	}
	return 0, fmt.Errorf("invalid slot token '%s' for shape %d", token, shape)
}

func (d *Daemon) inferShapeAndSlot(num int) (shape int, slot int) {
	switch {
	case num >= 1 && num <= 2:
		return 2, num
	case num >= 3 && num <= 5:
		return 3, num - 2
	case num >= 7 && num <= 10:
		return 4, num - 6
	default:
		return 0, 0
	}
}

func (d *Daemon) slotNumberToToken(shape, slot int) string {
	switch shape {
	case 2:
		if slot == 1 {
			return "j"
		}
		return "l"
	case 3:
		tokens := []string{"j", "k", "l"}
		if slot >= 1 && slot <= 3 {
			return tokens[slot-1]
		}
	case 4:
		tokens := []string{"ij", "il", "kj", "kl"}
		if slot >= 1 && slot <= 4 {
			return tokens[slot-1]
		}
	}
	return ""
}

func (d *Daemon) ensureWorkspaceExistsLocked(targetWS int) error {
	for d.maxWS < targetWS {
		if err := d.executor.WorkspaceAdd(""); err != nil {
			return err
		}
		d.maxWS++
	}
	return nil
}

func inferPlacementFromView(view executor.IPCViewState) (shape int, slot int, ok bool) {
	if view.UsableW <= 0 || view.UsableH <= 0 || view.W <= 0 || view.H <= 0 {
		return 0, 0, false
	}

	cx := (float64(view.X-view.UsableX) + float64(view.W)/2.0) / float64(view.UsableW)
	cy := (float64(view.Y-view.UsableY) + float64(view.H)/2.0) / float64(view.UsableH)
	rw := float64(view.W) / float64(view.UsableW)
	rh := float64(view.H) / float64(view.UsableH)

	if cx < -0.1 || cx > 1.1 || cy < -0.1 || cy > 1.1 {
		return 0, 0, false
	}

	// Quarters: about half width and half height, classified by center quadrant.
	if rw >= 0.35 && rw <= 0.65 && rh >= 0.25 && rh <= 0.65 {
		left := cx < 0.5
		top := cy < 0.5
		switch {
		case top && left:
			return 4, 1, true
		case top && !left:
			return 4, 2, true
		case !top && left:
			return 4, 3, true
		default:
			return 4, 4, true
		}
	}

	// Halves and thirds are full-height layouts.
	if rh < 0.60 || rh > 1.10 {
		return 0, 0, false
	}

	if rw >= 0.40 && rw <= 0.65 {
		if cx < 0.5 {
			return 2, 1, true
		}
		return 2, 2, true
	}

	if rw >= 0.20 && rw <= 0.45 {
		switch {
		case cx < 0.34:
			return 3, 1, true
		case cx > 0.66:
			return 3, 3, true
		default:
			return 3, 2, true
		}
	}

	return 0, 0, false
}

func (d *Daemon) setCurrentWorkspace(ws int) {
	d.mu.Lock()
	changed := d.currentWS != ws
	d.currentWS = ws
	d.mu.Unlock()

	_ = d.state.SetCurrentWS(ws)

	if changed && d.debug {
		fmt.Fprintf(os.Stderr, "[intentile] current workspace -> %d\n", ws)
	}
}

func (d *Daemon) notify(msg string) {
	if !d.debug {
		return
	}

	// Use notify-send if available
	cmd := exec.Command("notify-send", "-a", "intentile", msg)
	_ = cmd.Run()

	// Also log to stderr
	fmt.Fprintf(os.Stderr, "[intentile] %s\n", msg)
}
