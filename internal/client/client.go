package client

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Client communicates with the intentile daemon via Unix socket
type Client struct {
	socketPath string
	pidPath    string
}

// NewClient creates a client instance
func NewClient() *Client {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), fmt.Sprintf("intentile-%d", os.Getuid()))
	}

	return &Client{
		socketPath: filepath.Join(runtimeDir, "intentile.sock"),
		pidPath:    filepath.Join(runtimeDir, "intentile.pid"),
	}
}

// IsRunning checks if daemon is running
func (c *Client) IsRunning() bool {
	_, err := os.Stat(c.socketPath)
	return err == nil
}

// StartDaemon starts the daemon process
func (c *Client) StartDaemon() error {
	if c.IsRunning() {
		return nil // Already running
	}

	// Get current executable path
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start daemon in background
	cmd := exec.Command(exe, "daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Detach from parent
	_ = cmd.Process.Release()

	// Wait for socket to appear (max 2 seconds)
	for i := 0; i < 20; i++ {
		if c.IsRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon failed to start (socket not found)")
}

// SendCommand sends a command to the daemon and returns the response
func (c *Client) SendCommand(cmdLine string) (string, error) {
	// Auto-start daemon if not running
	if !c.IsRunning() {
		if err := c.StartDaemon(); err != nil {
			return "", err
		}
	}

	// Connect to socket
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return "", fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer conn.Close()

	// Send command
	if _, err := conn.Write([]byte(cmdLine + "\n")); err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	// Read response
	scanner := bufio.NewScanner(conn)
	var response strings.Builder
	for scanner.Scan() {
		response.WriteString(scanner.Text())
		response.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return strings.TrimSpace(response.String()), nil
}

// Arm sends arm command
func (c *Client) Arm(target string, shape int) error {
	resp, err := c.SendCommand(fmt.Sprintf("ARM %s %d", target, shape))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "OK") {
		return fmt.Errorf("%s", resp)
	}
	return nil
}

// Slot sends slot command
func (c *Client) Slot(token string) error {
	resp, err := c.SendCommand(fmt.Sprintf("SLOT %s", token))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "OK") {
		return fmt.Errorf("%s", resp)
	}
	return nil
}

// PlaceAtomic sends atomic placement command
func (c *Client) PlaceAtomic(num int) error {
	resp, err := c.SendCommand(fmt.Sprintf("PLACE_ATOMIC %d", num))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "OK") {
		return fmt.Errorf("%s", resp)
	}
	return nil
}

// Clear sends clear command
func (c *Client) Clear() error {
	resp, err := c.SendCommand("CLEAR")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "OK") {
		return fmt.Errorf("%s", resp)
	}
	return nil
}

// Status gets daemon status
func (c *Client) Status() (string, error) {
	if !c.IsRunning() {
		return "daemon not running", nil
	}
	return c.SendCommand("STATUS")
}

// Stop stops the daemon
func (c *Client) Stop() error {
	if !c.IsRunning() {
		return fmt.Errorf("daemon not running")
	}

	resp, err := c.SendCommand("STOP")
	if err != nil {
		return err
	}

	// Wait for daemon to exit
	for i := 0; i < 30; i++ {
		if !c.IsRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill if still running
	pidData, err := os.ReadFile(c.pidPath)
	if err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(pidData))); err == nil {
			proc, err := os.FindProcess(pid)
			if err == nil {
				_ = proc.Kill()
			}
		}
	}

	return fmt.Errorf("daemon did not stop cleanly: %s", resp)
}
