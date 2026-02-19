package executor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// LabWCExecutor executes window operations via wtype + LabWC keybinds
type LabWCExecutor struct {
	debug bool
}

// NewLabWCExecutor creates a LabWC executor
func NewLabWCExecutor(debug bool) *LabWCExecutor {
	return &LabWCExecutor{debug: debug}
}

// SnapToSlot snaps the focused window to a slot
func (e *LabWCExecutor) SnapToSlot(shape, slot int) error {
	key := e.getSlotKey(shape, slot)
	if key == "" {
		return fmt.Errorf("unsupported shape:slot %d:%d", shape, slot)
	}

	if e.debug {
		fmt.Fprintf(os.Stderr, "[executor] snap to %s\n", key)
	}

	return e.sendKey(key)
}

// SendToWorkspace moves focused window to workspace
func (e *LabWCExecutor) SendToWorkspace(currentWS, targetWS, maxWS int) error {
	if currentWS == targetWS {
		return nil // Already on target workspace
	}

	if e.debug {
		fmt.Fprintf(os.Stderr, "[executor] send to workspace: current=%d target=%d max=%d\n", currentWS, targetWS, maxWS)
	}

	// Calculate shortest path (forward or backward with wrapping)
	forward := (targetWS - currentWS + maxWS) % maxWS
	backward := (currentWS - targetWS + maxWS) % maxWS

	var key string
	var steps int

	if forward <= backward {
		key = getEnv("INTENTILE_KEY_SEND_NEXT", "super+ctrl+shift+Right")
		steps = forward
	} else {
		key = getEnv("INTENTILE_KEY_SEND_PREV", "super+ctrl+shift+Left")
		steps = backward
	}

	// Send multiple keypresses to reach target workspace
	for i := 0; i < steps; i++ {
		if err := e.sendKey(key); err != nil {
			return fmt.Errorf("failed to send workspace key (step %d/%d): %w", i+1, steps, err)
		}
		// Small delay between keypresses to let compositor process
		// TODO: Make this configurable
		if steps > 1 && i < steps-1 {
			// time.Sleep(50 * time.Millisecond) // Commented out for now, may not be needed
		}
	}

	return nil
}

// SwitchToWorkspace switches the current view to a workspace
func (e *LabWCExecutor) SwitchToWorkspace(currentWS, targetWS, maxWS int) error {
	if currentWS == targetWS {
		return nil
	}

	if e.debug {
		fmt.Fprintf(os.Stderr, "[executor] switch to workspace: current=%d target=%d max=%d\n", currentWS, targetWS, maxWS)
	}

	// Calculate shortest path
	forward := (targetWS - currentWS + maxWS) % maxWS
	backward := (currentWS - targetWS + maxWS) % maxWS

	var key string
	var steps int

	if forward <= backward {
		key = getEnv("INTENTILE_KEY_WS_NEXT", "super+ctrl+Right")
		steps = forward
	} else {
		key = getEnv("INTENTILE_KEY_WS_PREV", "super+ctrl+Left")
		steps = backward
	}

	for i := 0; i < steps; i++ {
		if err := e.sendKey(key); err != nil {
			return fmt.Errorf("failed to switch workspace (step %d/%d): %w", i+1, steps, err)
		}
	}

	return nil
}

// getSlotKey returns the keybind for a shape:slot combination
func (e *LabWCExecutor) getSlotKey(shape, slot int) string {
	// Environment variable overrides (matching shell script pattern)
	// User can customize keybinds via env vars

	switch shape {
	case 2: // halves
		switch slot {
		case 1: // left
			return getEnv("INTENTILE_KEY_HALF_LEFT", "super+ctrl+h")
		case 2: // right
			return getEnv("INTENTILE_KEY_HALF_RIGHT", "super+ctrl+l")
		}
	case 3: // thirds
		switch slot {
		case 1: // left
			return getEnv("INTENTILE_KEY_THIRD_LEFT", "super+ctrl+j")
		case 2: // middle
			return getEnv("INTENTILE_KEY_THIRD_MID", "super+ctrl+k")
		case 3: // right
			return getEnv("INTENTILE_KEY_THIRD_RIGHT", "super+ctrl+semicolon")
		}
	case 4: // quarters
		switch slot {
		case 1: // UL
			return getEnv("INTENTILE_KEY_QUARTER_UL", "super+ctrl+u")
		case 2: // UR
			return getEnv("INTENTILE_KEY_QUARTER_UR", "super+ctrl+i")
		case 3: // LL
			return getEnv("INTENTILE_KEY_QUARTER_LL", "super+ctrl+o")
		case 4: // LR
			return getEnv("INTENTILE_KEY_QUARTER_LR", "super+ctrl+p")
		}
	}

	return ""
}

// sendKey sends a key chord via wtype
func (e *LabWCExecutor) sendKey(chord string) error {
	// Parse chord: "super+ctrl+h" -> separate key and modifiers
	parts := strings.Split(chord, "+")
	if len(parts) == 0 {
		return fmt.Errorf("invalid key chord: %s", chord)
	}

	key := parts[len(parts)-1]
	modifiers := parts[:len(parts)-1]

	// Build wtype arguments
	args := []string{}

	// Press modifiers
	for _, mod := range modifiers {
		args = append(args, "-M", mod)
	}

	// Press key
	args = append(args, "-k", key)

	// Release modifiers
	for _, mod := range modifiers {
		args = append(args, "-m", mod)
	}

	cmd := exec.Command("wtype", args...)
	if e.debug {
		fmt.Fprintf(os.Stderr, "[executor] wtype %v\n", args)
	}

	return cmd.Run()
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
