package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/jaycee1285/intentile/internal/daemon"
)

const version = "0.1.0"

var daemonInstance *daemon.Daemon

func main() {
	// Initialize daemon (in-memory for now, later can be socket-based)
	daemonInstance = daemon.NewDaemon(daemon.Config{
		Debug: os.Getenv("INTENTILE_DEBUG") == "1",
	})

	ctx := context.Background()
	if err := daemonInstance.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "intentile: failed to start: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	var err error
	switch cmd {
	case "version":
		fmt.Printf("intentile %s\n", version)
	case "help":
		printUsage()
	case "arm":
		err = handleArm()
	case "slot":
		err = handleSlot()
	case "place":
		err = handlePlace()
	case "clear":
		err = daemonInstance.Clear()
	default:
		// Try parsing as atomic slot number (1-9)
		if num, parseErr := strconv.Atoi(cmd); parseErr == nil && num >= 1 && num <= 9 {
			err = daemonInstance.PlaceAtomic(num)
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

func handleArm() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: intentile arm <target> <shape>")
	}
	target := os.Args[2]
	shape, err := strconv.Atoi(os.Args[3])
	if err != nil || (shape != 2 && shape != 3 && shape != 4) {
		return fmt.Errorf("invalid shape '%s' (expected 2, 3, or 4)", os.Args[3])
	}
	return daemonInstance.Arm(target, shape)
}

func handleSlot() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: intentile slot <token>")
	}
	token := os.Args[2]
	return daemonInstance.Slot(token)
}

func handlePlace() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: intentile place <target> <spec>")
	}
	// TODO: implement place command (parse spec like "2l", "3k", "4ij")
	return fmt.Errorf("place command not yet implemented")
}

func printUsage() {
	usage := `intentile - Intent-first autotiling for stacking compositors

Usage:
  intentile arm <next|prev|here|ws-index>
  intentile shape <2|3|4>
  intentile slot <token>
  intentile place <target> <spec>
  intentile clear
  intentile reset
  intentile version
  intentile help

Intent Grammar:
  - Left hand: choose destination + layout (arm + shape)
  - Right hand: choose slot placement

Examples:
  intentile arm next          # Arm next workspace
  intentile shape 3           # Set 3-column layout
  intentile slot k            # Place in middle slot
  intentile place next 2l     # Place in next workspace, left half
`
	fmt.Print(usage)
}
