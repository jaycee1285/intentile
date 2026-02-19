package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/jaycee1285/intentile/internal/occupancy"
	"github.com/jaycee1285/intentile/internal/state"
)

// Daemon manages intentile's long-running state and command processing
type Daemon struct {
	mu          sync.RWMutex
	state       *state.Manager
	occupancy   *occupancy.Tracker
	currentWS   int
	armedWS     int
	armedShape  int
	armedTime   time.Time
	maxWS       int
	debug       bool
}

// Config holds daemon configuration
type Config struct {
	StateDir  string
	MaxWS     int
	ArmTTL    time.Duration
	ShapeTTL  time.Duration
	Debug     bool
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

	d.notify("intentile daemon started")
	return nil
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
		targetWS = d.occupancy.FindAvailableWorkspace(targetWS, d.armedShape, d.maxWS)
		d.notify(fmt.Sprintf("Overflow: %s, moving to ws:%d", reason, targetWS))
	}

	// Place in occupancy tracker
	if err := d.occupancy.Place(targetWS, d.armedShape, slot); err != nil {
		return err
	}

	// TODO: Execute actual window movement via compositor backend

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
		return fmt.Errorf("invalid slot number: %d (expected 1-9)", slotNum)
	}

	// Arm next workspace with inferred shape
	if err := d.Arm("next", shape); err != nil {
		return err
	}

	// Place in the slot
	slotToken := d.slotNumberToToken(shape, slot)
	return d.Slot(slotToken)
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
	case num >= 6 && num <= 9:
		return 4, num - 5
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
