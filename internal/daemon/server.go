package daemon

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Server manages the socket-based daemon server
type Server struct {
	daemon     *Daemon
	socketPath string
	pidPath    string
	listener   net.Listener
	mu         sync.Mutex
	shutdown   chan struct{}
}

// NewServer creates a daemon server
func NewServer(daemon *Daemon) (*Server, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), fmt.Sprintf("intentile-%d", os.Getuid()))
		if err := os.MkdirAll(runtimeDir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create runtime dir: %w", err)
		}
	}

	return &Server{
		daemon:     daemon,
		socketPath: filepath.Join(runtimeDir, "intentile.sock"),
		pidPath:    filepath.Join(runtimeDir, "intentile.pid"),
		shutdown:   make(chan struct{}),
	}, nil
}

// Start starts the socket server
func (s *Server) Start(ctx context.Context) error {
	// Remove stale socket if exists
	_ = os.Remove(s.socketPath)

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	s.listener = listener

	// Write PID file
	if err := os.WriteFile(s.pidPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Start daemon
	if err := s.daemon.Start(ctx); err != nil {
		_ = listener.Close()
		_ = os.Remove(s.pidPath)
		return err
	}

	s.daemon.notify("daemon server started")

	// Accept connections
	go s.acceptLoop()

	// Wait for shutdown signal
	<-s.shutdown

	// Clean shutdown
	_ = listener.Close()
	_ = os.Remove(s.socketPath)
	_ = os.Remove(s.pidPath)

	return nil
}

// Stop signals the server to shutdown
func (s *Server) Stop() {
	close(s.shutdown)
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Listener closed, exit loop
			return
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	line := scanner.Text()
	response := s.handleCommand(line)

	// Send response
	_, _ = conn.Write([]byte(response + "\n"))
}

func (s *Server) handleCommand(line string) string {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "ERROR: empty command"
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "ARM":
		if len(args) < 2 {
			return "ERROR: usage: ARM <target> <shape>"
		}
		target := args[0]
		shape, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Sprintf("ERROR: invalid shape: %v", err)
		}
		if err := s.daemon.Arm(target, shape); err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return "OK"

	case "SLOT":
		if len(args) < 1 {
			return "ERROR: usage: SLOT <token>"
		}
		token := args[0]
		if err := s.daemon.Slot(token); err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return "OK"

	case "PLACE_ATOMIC":
		if len(args) < 1 {
			return "ERROR: usage: PLACE_ATOMIC <number>"
		}
		num, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Sprintf("ERROR: invalid number: %v", err)
		}
		if err := s.daemon.PlaceAtomic(num); err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return "OK"

	case "CLEAR":
		if err := s.daemon.Clear(); err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return "OK"

	case "STATUS":
		return s.getStatus()

	case "STOP":
		// Signal shutdown after responding
		go func() {
			s.Stop()
		}()
		return "OK: stopping daemon"

	default:
		return fmt.Sprintf("ERROR: unknown command: %s", cmd)
	}
}

func (s *Server) getStatus() string {
	s.daemon.mu.RLock()
	defer s.daemon.mu.RUnlock()

	var lines []string
	lines = append(lines, fmt.Sprintf("current_ws: %d", s.daemon.currentWS))
	lines = append(lines, fmt.Sprintf("armed_ws: %d", s.daemon.armedWS))
	lines = append(lines, fmt.Sprintf("armed_shape: %d", s.daemon.armedShape))
	lines = append(lines, fmt.Sprintf("max_ws: %d", s.daemon.maxWS))

	// Get occupancy state
	workspaces := s.daemon.occupancy.GetAllWorkspaces()

	if len(workspaces) == 0 {
		lines = append(lines, "occupancy: (empty)")
	} else {
		lines = append(lines, "occupancy:")
		for wsIdx, ws := range workspaces {
			slots := []string{}
			for slot := range ws.OccupiedSlots {
				slots = append(slots, strconv.Itoa(slot))
			}
			lines = append(lines, fmt.Sprintf("  ws%d: shape=%d slots=[%s]", wsIdx, ws.Shape, strings.Join(slots, ",")))
		}
	}

	return strings.Join(lines, "\n")
}

// SocketPath returns the socket path
func (s *Server) SocketPath() string {
	return s.socketPath
}

// PIDPath returns the PID file path
func (s *Server) PIDPath() string {
	return s.pidPath
}
