package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "version":
		fmt.Printf("intentile %s\n", version)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "intentile: unknown command '%s'\n", cmd)
		printUsage()
		os.Exit(1)
	}
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
