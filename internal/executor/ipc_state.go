package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// IPCEvent is a parsed EVENT line from SartWC's subscribe-events stream.
type IPCEvent struct {
	Name   string
	Fields map[string]string
}

// IPCViewState is a single mapped view entry from list-views-json.
type IPCViewState struct {
	AppID         string `json:"app_id"`
	Title         string `json:"title"`
	Workspace     int    `json:"workspace"`
	WorkspaceName string `json:"workspace_name"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	W             int    `json:"w"`
	H             int    `json:"h"`
	Output        string `json:"output"`
	UsableX       int    `json:"usable_x"`
	UsableY       int    `json:"usable_y"`
	UsableW       int    `json:"usable_w"`
	UsableH       int    `json:"usable_h"`
	Maximized     bool   `json:"maximized"`
	Minimized     bool   `json:"minimized"`
	Fullscreen    bool   `json:"fullscreen"`
	Tiled         bool   `json:"tiled"`
	Focused       bool   `json:"focused"`
}

// IPCViewsState is the JSON response from SartWC list-views-json.
type IPCViewsState struct {
	CurrentWorkspace     int            `json:"current_workspace"`
	CurrentWorkspaceName string         `json:"current_workspace_name"`
	Views                []IPCViewState `json:"views"`
}

// QueryWorkspaceState returns the current workspace and configured workspace count.
func (e *LabWCExecutor) QueryWorkspaceState() (int, int, error) {
	path, err := ipcSocketPath()
	if err != nil {
		return 0, 0, err
	}

	conn, err := net.Dial("unix", path)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to connect to SartWC: %w", err)
	}
	defer conn.Close()

	if _, err := fmt.Fprint(conn, "list-workspaces\n"); err != nil {
		return 0, 0, fmt.Errorf("failed to send list-workspaces: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	decodePercent := false
	current := 0
	maxWS := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "END" {
			if current == 0 {
				return 0, 0, fmt.Errorf("list-workspaces missing current workspace")
			}
			return current, maxWS, nil
		}

		tokens := strings.Fields(line)
		if len(tokens) == 0 {
			continue
		}

		if tokens[0] == "workspace" {
			fields := parseKVTokens(tokens[1:], decodePercent)
			if idx, err := strconv.Atoi(fields["index"]); err == nil && idx > maxWS {
				maxWS = idx
			}
			continue
		}

		fields := parseKVTokens(tokens, decodePercent)
		if enc, ok := fields["encoding"]; ok && enc == "percent" {
			decodePercent = true
		}
		if cur, ok := fields["current"]; ok {
			if n, err := strconv.Atoi(cur); err == nil {
				current = n
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("failed to read list-workspaces response: %w", err)
	}
	return 0, 0, fmt.Errorf("list-workspaces connection closed before END")
}

// QueryViewsState returns SartWC's JSON view snapshot for reconciliation.
func (e *LabWCExecutor) QueryViewsState() (*IPCViewsState, error) {
	path, err := ipcSocketPath()
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SartWC: %w", err)
	}
	defer conn.Close()

	if _, err := fmt.Fprint(conn, "list-views-json\n"); err != nil {
		return nil, fmt.Errorf("failed to send list-views-json: %w", err)
	}

	var resp IPCViewsState
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode list-views-json response: %w", err)
	}

	return &resp, nil
}

// SubscribeEvents consumes SartWC's event stream until context cancellation or connection error.
func (e *LabWCExecutor) SubscribeEvents(ctx context.Context, fn func(IPCEvent)) error {
	path, err := ipcSocketPath()
	if err != nil {
		return err
	}

	conn, err := net.Dial("unix", path)
	if err != nil {
		return fmt.Errorf("failed to connect to SartWC: %w", err)
	}
	defer conn.Close()

	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()
	defer close(stop)

	if _, err := fmt.Fprint(conn, "subscribe-events\n"); err != nil {
		return fmt.Errorf("failed to subscribe to SartWC events: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read subscribe response: %w", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("connection closed before subscribe acknowledgement")
	}
	ack := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(ack, "OK") {
		return fmt.Errorf("subscribe-events failed: %s", ack)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "EVENT ") {
			continue
		}
		ev, ok := parseEventLine(line)
		if ok {
			fn(ev)
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("event stream read error: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("event stream closed")
}

func parseEventLine(line string) (IPCEvent, bool) {
	tokens := strings.Fields(line)
	if len(tokens) < 2 || tokens[0] != "EVENT" {
		return IPCEvent{}, false
	}
	return IPCEvent{
		Name:   tokens[1],
		Fields: parseKVTokens(tokens[2:], true),
	}, true
}

func parseKVTokens(tokens []string, decodePercent bool) map[string]string {
	fields := make(map[string]string, len(tokens))
	for _, tok := range tokens {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		if decodePercent {
			if dec, err := pctDecode(v); err == nil {
				v = dec
			}
		}
		fields[k] = v
	}
	return fields
}

func pctDecode(s string) (string, error) {
	if !strings.Contains(s, "%") {
		return s, nil
	}

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b.WriteByte(s[i])
			continue
		}
		if i+2 >= len(s) {
			return "", fmt.Errorf("truncated percent escape")
		}
		n, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
		if err != nil {
			return "", err
		}
		b.WriteByte(byte(n))
		i += 2
	}
	return b.String(), nil
}
