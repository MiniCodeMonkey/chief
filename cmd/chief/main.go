package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/minicodemonkey/chief/internal/cmd"
	"github.com/minicodemonkey/chief/internal/prd"
	"github.com/minicodemonkey/chief/internal/tui"
)

func main() {
	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			runInit()
			return
		case "edit":
			runEdit()
			return
		case "help", "--help", "-h":
			printHelp()
			return
		}
	}

	// Default: run the TUI
	runTUI()
}

func runInit() {
	opts := cmd.InitOptions{}

	// Parse arguments: chief init [name] [context...]
	if len(os.Args) > 2 {
		opts.Name = os.Args[2]
	}
	if len(os.Args) > 3 {
		opts.Context = strings.Join(os.Args[3:], " ")
	}

	if err := cmd.RunInit(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runEdit() {
	opts := cmd.EditOptions{}

	// Parse arguments: chief edit [name] [--merge] [--force]
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--merge":
			opts.Merge = true
		case "--force":
			opts.Force = true
		default:
			// If not a flag, treat as PRD name (first non-flag arg)
			if opts.Name == "" && !strings.HasPrefix(arg, "-") {
				opts.Name = arg
			}
		}
	}

	if err := cmd.RunEdit(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI() {
	// For now, use a default PRD path (will be configurable via CLI flags in US-022)
	prdPath := ".chief/prds/main/prd.json"

	// Check for command-line argument for PRD path
	if len(os.Args) > 1 {
		arg := os.Args[1]
		// If it looks like a path or name, use it
		if strings.HasSuffix(arg, ".json") || strings.HasSuffix(arg, "/") {
			prdPath = arg
		} else if !strings.HasPrefix(arg, "-") {
			// Treat as PRD name
			prdPath = fmt.Sprintf(".chief/prds/%s/prd.json", arg)
		}
	}

	// Check if prd.md is newer than prd.json and run conversion if needed
	prdDir := filepath.Dir(prdPath)
	needsConvert, err := prd.NeedsConversion(prdDir)
	if err != nil {
		fmt.Printf("Warning: failed to check conversion status: %v\n", err)
	} else if needsConvert {
		fmt.Println("prd.md is newer than prd.json, running conversion...")
		if err := prd.Convert(prd.ConvertOptions{PRDDir: prdDir}); err != nil {
			fmt.Printf("Error converting PRD: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Conversion complete.")
	}

	app, err := tui.NewApp(prdPath)
	if err != nil {
		fmt.Printf("Error loading PRD: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`Chief - Autonomous PRD Agent

Usage:
  chief                     Launch TUI with default PRD (.chief/prds/main/)
  chief <name>              Launch TUI with named PRD (.chief/prds/<name>/)
  chief <path/to/prd.json>  Launch TUI with specific PRD file

Commands:
  init [name] [context]     Create a new PRD interactively
  edit [name] [options]     Edit an existing PRD interactively
  help                      Show this help message

Edit Options:
  --merge                   Auto-merge progress on conversion conflicts
  --force                   Auto-overwrite on conversion conflicts

Examples:
  chief init                Create PRD in .chief/prds/main/
  chief init auth           Create PRD in .chief/prds/auth/
  chief init auth "JWT authentication for REST API"
                            Create PRD with context hint
  chief edit                Edit PRD in .chief/prds/main/
  chief edit auth           Edit PRD in .chief/prds/auth/
  chief edit auth --merge   Edit and auto-merge progress`)
}
