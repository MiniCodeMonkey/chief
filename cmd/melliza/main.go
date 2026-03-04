package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lvcoi/melliza/internal/cmd"
	"github.com/lvcoi/melliza/internal/config"
	"github.com/lvcoi/melliza/internal/git"
	"github.com/lvcoi/melliza/internal/prd"
	"github.com/lvcoi/melliza/internal/tui"
)

// Version is set at build time via ldflags
var Version = "dev"

// TUIOptions holds the parsed command-line options for the TUI
type TUIOptions struct {
	PRDPath       string
	MaxIterations int
	Verbose       bool
	Merge         bool
	Force         bool
	NoRetry       bool
	StartWithInit bool   // Start in PRD creation chat mode
	InitName      string // PRD name for init mode
	InitContext   string // Optional context for init mode
}

func main() {
	// Handle subcommands first
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "new":
			runNew()
			return
		case "edit":
			runEdit()
			return
		case "status":
			runStatus()
			return
		case "list":
			runList()
			return
		case "help":
			printHelp()
			return
		case "--help", "-h":
			printHelp()
			return
		case "--version", "-v":
			fmt.Printf("melliza version %s\n", Version)
			return
		case "update":
			runUpdate()
			return
		case "wiggum":
			printWiggum()
			return
		}
	}

	// Non-blocking version check on startup (for interactive TUI sessions)
	cmd.CheckVersionOnStartup(Version)

	// Parse flags for TUI mode
	opts := parseTUIFlags()

	// Handle special flags that were parsed
	if opts == nil {
		// Already handled (--help or --version)
		return
	}

	// Run the TUI
	runTUIWithOptions(opts)
}

// findAvailablePRD looks for any available PRD in .melliza/prds/
// Returns the path to the first PRD found, or empty string if none exist.
func findAvailablePRD() string {
	prdsDir := ".melliza/prds"
	entries, err := os.ReadDir(prdsDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			prdPath := filepath.Join(prdsDir, entry.Name(), "prd.json")
			if _, err := os.Stat(prdPath); err == nil {
				return prdPath
			}
		}
	}
	return ""
}

// listAvailablePRDs returns all PRD names in .melliza/prds/
func listAvailablePRDs() []string {
	prdsDir := ".melliza/prds"
	entries, err := os.ReadDir(prdsDir)
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			prdPath := filepath.Join(prdsDir, entry.Name(), "prd.json")
			if _, err := os.Stat(prdPath); err == nil {
				names = append(names, entry.Name())
			}
		}
	}
	return names
}

// parseTUIFlags parses command-line flags for TUI mode
func parseTUIFlags() *TUIOptions {
	opts := &TUIOptions{
		PRDPath:       "", // Will be resolved later
		MaxIterations: 0,  // 0 signals dynamic calculation (remaining stories + 5)
		Verbose:       false,
		Merge:         false,
		Force:         false,
		NoRetry:       false,
	}

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]

		switch {
		case arg == "--help" || arg == "-h":
			printHelp()
			return nil
		case arg == "--version" || arg == "-v":
			fmt.Printf("melliza version %s\n", Version)
			return nil
		case arg == "--verbose":
			opts.Verbose = true
		case arg == "--merge":
			opts.Merge = true
		case arg == "--force":
			opts.Force = true
		case arg == "--no-retry":
			opts.NoRetry = true
		case arg == "--max-iterations" || arg == "-n":
			// Next argument should be the number
			if i+1 < len(os.Args) {
				i++
				n, err := strconv.Atoi(os.Args[i])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid value for %s: %s\n", arg, os.Args[i])
					os.Exit(1)
				}
				if n < 1 {
					fmt.Fprintf(os.Stderr, "Error: --max-iterations must be at least 1\n")
					os.Exit(1)
				}
				opts.MaxIterations = n
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s requires a value\n", arg)
				os.Exit(1)
			}
		case strings.HasPrefix(arg, "--max-iterations="):
			val := strings.TrimPrefix(arg, "--max-iterations=")
			n, err := strconv.Atoi(val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid value for --max-iterations: %s\n", val)
				os.Exit(1)
			}
			if n < 1 {
				fmt.Fprintf(os.Stderr, "Error: --max-iterations must be at least 1\n")
				os.Exit(1)
			}
			opts.MaxIterations = n
		case strings.HasPrefix(arg, "-n="):
			val := strings.TrimPrefix(arg, "-n=")
			n, err := strconv.Atoi(val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid value for -n: %s\n", val)
				os.Exit(1)
			}
			if n < 1 {
				fmt.Fprintf(os.Stderr, "Error: -n must be at least 1\n")
				os.Exit(1)
			}
			opts.MaxIterations = n
		case strings.HasPrefix(arg, "-"):
			// Unknown flag
			fmt.Fprintf(os.Stderr, "Error: unknown flag: %s\n", arg)
			fmt.Fprintf(os.Stderr, "Run 'melliza --help' for usage.\n")
			os.Exit(1)
		default:
			// Positional argument: PRD name or path
			if strings.HasSuffix(arg, ".json") || strings.HasSuffix(arg, "/") {
				opts.PRDPath = arg
			} else {
				// Treat as PRD name
				opts.PRDPath = fmt.Sprintf(".melliza/prds/%s/prd.json", arg)
			}
		}
	}

	return opts
}

func runNew() {
	opts := &TUIOptions{
		StartWithInit: true,
		InitName:      "main",
		Verbose:       false, // Default to non-verbose
	}

	// Parse arguments: melliza new [name] [context...]
	if len(os.Args) > 2 {
		opts.InitName = os.Args[2]
	}
	if len(os.Args) > 3 {
		opts.InitContext = strings.Join(os.Args[3:], " ")
	}

	runTUIWithOptions(opts)
}

func runEdit() {
	opts := cmd.EditOptions{}

	// Parse arguments: melliza edit [name] [--merge] [--force]
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

func runStatus() {
	opts := cmd.StatusOptions{}

	// Parse arguments: melliza status [name]
	if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") {
		opts.Name = os.Args[2]
	}

	if err := cmd.RunStatus(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runUpdate() {
	if err := cmd.RunUpdate(cmd.UpdateOptions{
		Version: Version,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runList() {
	opts := cmd.ListOptions{}

	if err := cmd.RunList(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runTUIWithOptions(opts *TUIOptions) {
	prdPath := opts.PRDPath

	// If starting with init, we don't need a PRD path initially
	if opts.StartWithInit {
		cwd, _ := os.Getwd()
		// If no PRD found, but we want init, we can't create an App easily with missing PRD
		// Wait, NewAppWithOptions expects a valid PRD path.
		
		// If we're starting fresh, create a temporary/default prd.json if needed
		// or just use a dummy path and have the TUI handle it.
		
		// Let's create the directory for the new PRD and an empty prd.json first
		prdDir := filepath.Join(cwd, ".melliza", "prds", opts.InitName)
		_ = os.MkdirAll(prdDir, 0755)
		prdPath = filepath.Join(prdDir, "prd.json")
		if _, err := os.Stat(prdPath); os.IsNotExist(err) {
			emptyPRD := `{"project": "` + opts.InitName + `", "userStories": []}`
			_ = os.WriteFile(prdPath, []byte(emptyPRD), 0644)
		}
	}

	// If no PRD specified, try to find one
	if prdPath == "" {
		// Try "main" first
		mainPath := ".melliza/prds/main/prd.json"
		if _, err := os.Stat(mainPath); err == nil {
			prdPath = mainPath
		} else {
			// Look for any available PRD
			prdPath = findAvailablePRD()
		}

		// If still no PRD found, run first-time setup
		if prdPath == "" {
			cwd, _ := os.Getwd()
			showGitignore := git.IsGitRepo(cwd) && !git.IsMellizaIgnored(cwd)

			// Run the first-time setup TUI
			result, err := tui.RunFirstTimeSetup(cwd, showGitignore)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if result.Cancelled {
				return
			}

			// Save config from setup
			cfg := config.Default()
			cfg.OnComplete.Push = result.PushOnComplete
			cfg.OnComplete.CreatePR = result.CreatePROnComplete
			if err := config.Save(cwd, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save config: %v\n", err)
			}

			// Create the PRD
			newOpts := cmd.NewOptions{
				Name: result.PRDName,
			}
			if err := cmd.RunNew(newOpts); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Restart TUI with the new PRD
			opts.PRDPath = fmt.Sprintf(".melliza/prds/%s/prd.json", result.PRDName)
			runTUIWithOptions(opts)
			return
		}
	}

	prdDir := filepath.Dir(prdPath)

	// Check if prd.md is newer than prd.json and run conversion if needed
	needsConvert, err := prd.NeedsConversion(prdDir)
	if err != nil {
		fmt.Printf("Warning: failed to check conversion status: %v\n", err)
	} else if needsConvert {
		fmt.Println("prd.md is newer than prd.json, running conversion...")
		convertOpts := prd.ConvertOptions{
			PRDDir: prdDir,
			Merge:  opts.Merge,
			Force:  opts.Force,
		}
		if err := prd.Convert(convertOpts); err != nil {
			fmt.Printf("Error converting PRD: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Conversion complete.")
	}

	app, err := tui.NewAppWithOptions(prdPath, opts.MaxIterations)
	if err != nil {
		// Check if this is a missing PRD file error
		if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") {
			fmt.Printf("PRD not found: %s\n", prdPath)
			fmt.Println()
			// Show available PRDs if any exist
			available := listAvailablePRDs()
			if len(available) > 0 {
				fmt.Println("Available PRDs:")
				for _, name := range available {
					fmt.Printf("  melliza %s\n", name)
				}
				fmt.Println()
			}
			fmt.Println("Or create a new one:")
			fmt.Println("  melliza new               # Create default PRD")
			fmt.Println("  melliza new <name>        # Create named PRD")
		} else {
			fmt.Printf("Error: %v\n", err)
		}
		os.Exit(1)
	}

	// Set verbose mode if requested
	if opts.Verbose {
		app.SetVerbose(true)
	}

	// Disable retry if requested
	if opts.NoRetry {
		app.DisableRetry()
	}

	// Trigger init flow if requested
	if opts.StartWithInit {
		app.StartWithInit(opts.InitName, opts.InitContext)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	model, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Check for post-exit actions
	if finalApp, ok := model.(tui.App); ok {
		switch finalApp.PostExitAction {
		case tui.PostExitInit:
			// Run new command then restart TUI
			newOpts := cmd.NewOptions{
				Name: finalApp.PostExitPRD,
			}
			if err := cmd.RunNew(newOpts); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			// Restart TUI with the new PRD
			opts.PRDPath = fmt.Sprintf(".melliza/prds/%s/prd.json", finalApp.PostExitPRD)
			runTUIWithOptions(opts)

		case tui.PostExitEdit:
			// Run edit command then restart TUI
			editOpts := cmd.EditOptions{
				Name:  finalApp.PostExitPRD,
				Merge: opts.Merge,
				Force: opts.Force,
			}
			if err := cmd.RunEdit(editOpts); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			// Restart TUI with the edited PRD
			opts.PRDPath = fmt.Sprintf(".melliza/prds/%s/prd.json", finalApp.PostExitPRD)
			runTUIWithOptions(opts)
		}
	}
}

func printHelp() {
	fmt.Println(`Melliza - Autonomous PRD Agent

Usage:
  melliza [options] [<name>|<path/to/prd.json>]
  melliza <command> [arguments]

Commands:
  new [name] [context]      Create a new PRD interactively
  edit [name] [options]     Edit an existing PRD interactively
  status [name]             Show progress for a PRD (default: main)
  list                      List all PRDs with progress
  update                    Update Melliza to the latest version
  help                      Show this help message

Global Options:
  --max-iterations N, -n N  Set maximum iterations (default: dynamic)
  --no-retry                Disable auto-retry on Gemini crashes
  --verbose                 Show raw Gemini output in log
  --merge                   Auto-merge progress on conversion conflicts
  --force                   Auto-overwrite on conversion conflicts
  --help, -h                Show this help message
  --version, -v             Show version number

Edit Options:
  --merge                   Auto-merge progress on conversion conflicts
  --force                   Auto-overwrite on conversion conflicts

Positional Arguments:
  <name>                    PRD name (loads .melliza/prds/<name>/prd.json)
  <path/to/prd.json>        Direct path to a prd.json file

Examples:
  melliza                     Launch TUI with default PRD (.melliza/prds/main/)
  melliza auth                Launch TUI with named PRD (.melliza/prds/auth/)
  melliza ./my-prd.json       Launch TUI with specific PRD file
  melliza -n 20               Launch with 20 max iterations
  melliza --max-iterations=5 auth
                            Launch auth PRD with 5 max iterations
  melliza --verbose           Launch with raw Gemini output visible
  melliza new                 Create PRD in .melliza/prds/main/
  melliza new auth            Create PRD in .melliza/prds/auth/
  melliza new auth "JWT authentication for REST API"
                            Create PRD with context hint
  melliza edit                Edit PRD in .melliza/prds/main/
  melliza edit auth           Edit PRD in .melliza/prds/auth/
  melliza edit auth --merge   Edit and auto-merge progress
  melliza status              Show progress for default PRD
  melliza status auth         Show progress for auth PRD
  melliza list                List all PRDs with progress
  melliza --version           Show version number`)
}

func printWiggum() {
	// ANSI color codes
	blue := "\033[34m"
	yellow := "\033[33m"
	reset := "\033[0m"

	art := blue + `
                                                                 -=
                                      +%#-   :=#%#**%-
                                     ##+**************#%*-::::=*-
                                   :##***********************+***#
                                 :@#********%#%#******************#*
                                 :##*****%+-:::-%%%%%##************#:
                                   :#%###%%-:::+#*******##%%%*******#%*:
                                      -+%**#%%@@%%%%%%%%%#****#%##*##%%=
                                      -@@%%%%%%%%%%%%%%@*#%%#*##:::
                                    +%%%%%%%%%%%%%%@#+--=#--=#@+:
                                   -@@@@@%@@@@#%#=-=**--+*-----=#:
` + yellow + `                                       :*     *-   - :#-:*=-----=#:
                                       %::%@- *:  *@# +::=*--#=:-%:
                                       #- =+**##-    =*:::#*#-++:*:
                                        #+:-::+--%***-::::::::-*##
                                      :+#:+=:-==-*:::::::::::::::-%
                                     *=::::::::::::::-=*##*:::::::-+
                                     *-::::::::-=+**+-+%%%%+:::::--+
                                      :*%##**==++%%%######%:::::--%-
                                        :-=#--%####%%%%@@+:::::--%=
` + blue + `                     -#%%%%#-` + yellow + `          *:::+%%##%%#%%*:::::::-*#%-
                   :##++++=+++%:` + yellow + `        :@%*:::::::::::::::-=##*%%*%=
                  :%++++@%#+=++#` + yellow + `         %%%=--:::::---=+%%****%##@%#%%*:
                -%=-:-%%%*=+++##` + yellow + `      :*@%***@%%%###*********%%#%********%-
               *#+==**%++++++#*-` + yellow + `   :*%@*+*%*%%%%@*********%%**##****%=--#%*#
             *%#%-:+*++++*%#=#-` + yellow + `  :%#%#*+***#@%%%@%#%%%@%#*****%****%::::::##%-
            :*::::*-%@%@#=*%-` + yellow + `  :%*#%+*******%%%@#*************%****%-::::::**%=
             +==%*+-----+%` + yellow + `    %#*%#********#@%%@********%*%***#%**+*%-:::::*#*%:
              *=::----##**%:` + yellow + `+%#*@**********@%%%%*+***%-::::::#*%#****%#:::-%***%-
               #-:+@#***+*@%` + yellow + `**#%**********%%%#%%*****%::::::-#**%***************%
               =%*****+%%+**` + yellow + `@#%***********@%#%%#******%:::::%****@*********+****##
` + blue + `                %*#%@#*+++**#%` + yellow + `************%%%%%#********###*******@**************%:
                =#**++***+**@` + yellow + `************%%%%#%%*******************%*************##
                 %*++******@#` + yellow + `************@%%#%%@*******************#@*************@:
                  #***+***%#*` + yellow + `************@%%%%%@#*******************#%*************+
                   +#***##%**` + yellow + `************@%%%%%%%********************%************%
                     :######**` + yellow + `*+**********%%%%%%%%*********************%************%
                       :+%@#**` + yellow + `*******+*****#%@@%#******+***************#@*****+*****%:
` + blue + `                         @*********************************************##*+**+*****#+
                        =%%%%%@@@%%#**************************##%%@@@%%%@**********##
                        =%%#%%%%%%%%%%%%%----====%%%%%%%%%%%%%%%%#%%#%%%%%******#%#*%
                        :@@%%#%%%%%%%%%%#::::::::*%%%%%%%%%%%%%%%%%%#%%%@@#%%%##***#%
                          %*##%%@@@@%%%%%::::::::#%%%%%%%@@@@@@%%####****##****#%#==#
                          :%*********************************************#%#*+=-----*-
                           :%************************************+********@:::::----=+
                             ##**********+******************+************##::-::=--#-%
                              =%******************+*+*********************%:=-*:++:#-%
                               *#*****************************************@*#:*:*=:*+=
                                %*********#%#**************************+*%   -#+%**=:
                                **************#%%%%###*******************#
                                =#***************%      #****************#
                                :@***+**********##      *****************#
                                 %**************#=      =#+******+*******#
                                 =#*************%:      :@***************#
                                 :#****+********#        #***************#
                                 :#**************        =#**************#
                                 :%************%-        :%*************##
                                  #***********##          %*************%=
                                -%@@@%######%@@+          =%#***#*#%@@%#@:
                              :%%%%%%%%%%%%%%%%#         +@%%%%%%%%%%%%%%*
                             +@%%%%%%%%%%%%%%%%+       :%%%%%%%%%%%%%%##@+
                             #%%%%%%%%%%%@%@%@*       :@%%%%%%%%%%%%@%%@*
` + reset + `
                         "Bake 'em away, toys!"
                               - Melliza Wiggum
`
	fmt.Print(art)
}
