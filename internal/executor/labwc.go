package executor

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

// LabWCExecutor executes window operations via SartWC IPC socket
type LabWCExecutor struct {
	debug bool
}

// NewLabWCExecutor creates a LabWC executor
func NewLabWCExecutor(debug bool) *LabWCExecutor {
	return &LabWCExecutor{debug: debug}
}

type slotGeometry struct {
	x, y string
	w, h string
}

var slotGeometries = map[int]map[int]slotGeometry{
	2: { // halves
		1: {"0", "0", "50%", "100%"},
		2: {"50%", "0", "50%", "100%"},
	},
	3: { // thirds
		1: {"0", "0", "33%", "100%"},
		2: {"33%", "0", "34%", "100%"},
		3: {"67%", "0", "33%", "100%"},
	},
	4: { // quarters
		1: {"0", "0", "50%", "50%"},
		2: {"50%", "0", "50%", "50%"},
		3: {"0", "50%", "50%", "50%"},
		4: {"50%", "50%", "50%", "50%"},
	},
}

// SnapToSlot snaps the focused window to a slot via IPC
func (e *LabWCExecutor) SnapToSlot(shape, slot int) error {
	slots, ok := slotGeometries[shape]
	if !ok {
		return fmt.Errorf("unsupported shape: %d", shape)
	}
	geo, ok := slots[slot]
	if !ok {
		return fmt.Errorf("unsupported slot %d for shape %d", slot, shape)
	}

	return e.sendIPC(
		fmt.Sprintf("MoveTo x=%s y=%s", geo.x, geo.y),
		fmt.Sprintf("ResizeTo width=%s height=%s", geo.w, geo.h),
	)
}

// SendToWorkspace moves focused window to a workspace without following
func (e *LabWCExecutor) SendToWorkspace(currentWS, targetWS, maxWS int) error {
	if currentWS == targetWS {
		return nil
	}
	return e.sendIPC(fmt.Sprintf("SendToDesktop to=%d follow=no", targetWS))
}

// SwitchToWorkspace switches the active workspace
func (e *LabWCExecutor) SwitchToWorkspace(currentWS, targetWS, maxWS int) error {
	if currentWS == targetWS {
		return nil
	}
	return e.sendIPC(fmt.Sprintf("GoToDesktop to=%d", targetWS))
}

// WorkspaceAdd creates a workspace at runtime (name optional).
func (e *LabWCExecutor) WorkspaceAdd(name string) error {
	if strings.TrimSpace(name) == "" {
		return e.sendIPC("workspace-add")
	}
	return e.sendIPC(fmt.Sprintf("workspace-add name=%s", pctEncode(name)))
}

// WorkspaceRemove removes a workspace by 1-based index.
func (e *LabWCExecutor) WorkspaceRemove(index int) error {
	if index < 1 {
		return fmt.Errorf("invalid workspace index: %d", index)
	}
	return e.sendIPC(fmt.Sprintf("workspace-remove index=%d", index))
}

// WorkspaceRename renames a workspace by 1-based index.
func (e *LabWCExecutor) WorkspaceRename(index int, name string) error {
	if index < 1 {
		return fmt.Errorf("invalid workspace index: %d", index)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}
	return e.sendIPC(fmt.Sprintf("workspace-rename index=%d name=%s", index, pctEncode(name)))
}

// sendIPC sends commands to SartWC and validates responses
func (e *LabWCExecutor) sendIPC(commands ...string) error {
	path, err := ipcSocketPath()
	if err != nil {
		return err
	}

	conn, err := net.Dial("unix", path)
	if err != nil {
		return fmt.Errorf("failed to connect to SartWC (is the compositor running?): %w", err)
	}
	defer conn.Close()

	for _, cmd := range commands {
		if e.debug {
			fmt.Fprintf(os.Stderr, "[executor] ipc: %s\n", cmd)
		}
		if _, err := fmt.Fprintf(conn, "%s\n", cmd); err != nil {
			return fmt.Errorf("failed to send '%s': %w", cmd, err)
		}
	}

	scanner := bufio.NewScanner(conn)
	for _, cmd := range commands {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read error after '%s': %w", cmd, err)
			}
			return fmt.Errorf("connection closed after '%s'", cmd)
		}
		resp := scanner.Text()
		if strings.HasPrefix(resp, "ERROR") {
			return fmt.Errorf("'%s': %s", cmd, resp)
		}
	}

	return nil
}

func pctEncode(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func ipcSocketPath() (string, error) {
	if path := os.Getenv("SARTWC_IPC_SOCKET"); path != "" {
		return path, nil
	}

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	display := os.Getenv("WAYLAND_DISPLAY")
	if runtimeDir == "" || display == "" {
		return "", fmt.Errorf("SARTWC_IPC_SOCKET not set and cannot construct path (need XDG_RUNTIME_DIR + WAYLAND_DISPLAY)")
	}

	return fmt.Sprintf("%s/sartwc-%s.sock", runtimeDir, display), nil
}
