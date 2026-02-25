package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jaycee1285/intentile/internal/client"
	"github.com/jaycee1285/intentile/internal/daemon"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	// Daemon mode - run as server
	if cmd == "daemon" {
		runDaemon()
		return
	}

	// Client mode - send commands to daemon
	c := client.NewClient()

	var err error
	switch cmd {
	case "version":
		fmt.Printf("intentile %s\n", version)
	case "help":
		printUsage()
	case "arm":
		err = handleArm(c)
	case "slot":
		err = handleSlot(c)
	case "workspace", "ws":
		err = handleWorkspace(c)
	case "place":
		err = handlePlace(c)
	case "clear":
		err = c.Clear()
	case "reconcile":
		err = c.Reconcile()
	case "status":
		status, statusErr := c.Status()
		if statusErr != nil {
			err = statusErr
		} else {
			fmt.Println(status)
		}
	case "stop":
		err = c.Stop()
		if err == nil {
			fmt.Println("daemon stopped")
		}
	default:
		// Try parsing as atomic slot number (1-10, skipping 6)
		if num, parseErr := strconv.Atoi(cmd); parseErr == nil && num >= 1 && num <= 10 && num != 6 {
			err = c.PlaceAtomic(num)
		} else {
			fmt.Fprintf(os.Stderr, "intentile: unknown command '%s'\n", cmd)
			printUsage()
			os.Exit(1)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "intentile: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon() {
	d := daemon.NewDaemon(daemon.Config{
		Debug: os.Getenv("INTENTILE_DEBUG") == "1",
	})

	server, err := daemon.NewServer(d)
	if err != nil {
		fmt.Fprintf(os.Stderr, "intentile: failed to create server: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "intentile: daemon error: %v\n", err)
		os.Exit(1)
	}
}

func handleArm(c *client.Client) error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: intentile arm <target> <shape>")
	}
	target := os.Args[2]
	shape, err := strconv.Atoi(os.Args[3])
	if err != nil || (shape != 2 && shape != 3 && shape != 4) {
		return fmt.Errorf("invalid shape '%s' (expected 2, 3, or 4)", os.Args[3])
	}
	return c.Arm(target, shape)
}

func handleSlot(c *client.Client) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: intentile slot <token>")
	}
	token := os.Args[2]
	return c.Slot(token)
}

func handlePlace(c *client.Client) error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: intentile place <target> <spec>")
	}
	// TODO: implement place command (parse spec like "2l", "3k", "4ij")
	return fmt.Errorf("place command not yet implemented")
}

func handleWorkspace(c *client.Client) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: intentile workspace <add|remove|rename> ...")
	}

	switch os.Args[2] {
	case "add":
		name := ""
		if len(os.Args) > 3 {
			name = strings.Join(os.Args[3:], " ")
		}
		return c.WorkspaceAdd(name)
	case "remove", "rm", "del":
		if len(os.Args) < 4 {
			return fmt.Errorf("usage: intentile workspace remove <index>")
		}
		index, err := strconv.Atoi(os.Args[3])
		if err != nil {
			return fmt.Errorf("invalid workspace index '%s'", os.Args[3])
		}
		return c.WorkspaceRemove(index)
	case "rename":
		if len(os.Args) < 5 {
			return fmt.Errorf("usage: intentile workspace rename <index> <name>")
		}
		index, err := strconv.Atoi(os.Args[3])
		if err != nil {
			return fmt.Errorf("invalid workspace index '%s'", os.Args[3])
		}
		name := strings.Join(os.Args[4:], " ")
		return c.WorkspaceRename(index, name)
	default:
		return fmt.Errorf("unknown workspace subcommand '%s'", os.Args[2])
	}
}

func printUsage() {
	usage := `intentile - Intent-first autotiling for stacking compositors

Usage:
  intentile daemon                  Start daemon server
  intentile arm <next|prev|here> <2|3|4>
  intentile slot <token>            Place in slot (j/k/l/ij/il/kj/kl)
  intentile <1-5|7-10>                   Atomic placement (auto-starts daemon)
  intentile workspace add [name]    Add workspace (live, SartWC IPC)
  intentile workspace remove <idx>   Remove workspace by index (live)
  intentile workspace rename <idx> <name>
  intentile reconcile               Rebuild occupancy from compositor state
  intentile clear                   Clear armed state
  intentile status                  Show daemon status
  intentile stop                    Stop daemon
  intentile version
  intentile help

Intent Grammar:
  - Left hand (FN+A/S/D): arm workspace + shape
  - Right hand (J/K/L/I): slot placement

Examples:
  intentile arm next 3      # Arm next workspace, shape 3
  intentile slot k          # Place in middle slot (shape 3)
  intentile 5               # Atomic: next ws, shape 3, right third
  intentile workspace add code
  intentile workspace rename 4 web docs
  intentile workspace remove 7
  intentile reconcile       # Rebuild occupancy tracker from live windows
  intentile status          # Show current state

Commands auto-start daemon if not running.
Set INTENTILE_DEBUG=1 for notify-send debugging.
`
	fmt.Print(usage)
}
