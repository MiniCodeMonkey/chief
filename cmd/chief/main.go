package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/minicodemonkey/chief/internal/cmd"
	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/git"
	"github.com/minicodemonkey/chief/internal/prd"
	"github.com/minicodemonkey/chief/internal/tui"
	"github.com/spf13/cobra"
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
}

func main() {
	rootCmd := buildRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	opts := &TUIOptions{}

	rootCmd := &cobra.Command{
		Use:   "chief [name|path/to/prd.json]",
		Short: "Chief - Autonomous PRD Agent",
		Long:  "Chief breaks down PRDs into user stories and uses Claude Code to implement them autonomously.",
		// Accept arbitrary args so positional PRD name/path works
		Args:    cobra.ArbitraryArgs,
		Version: Version,
		// Silence Cobra's default error/usage printing so we control output
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			// Non-blocking version check on startup for all interactive commands
			// Skip for update command itself and serve (which has its own check)
			name := c.Name()
			if name != "update" && name != "serve" && name != "version" {
				cmd.CheckVersionOnStartup(Version)
			}
		},
		RunE: func(c *cobra.Command, args []string) error {
			// Resolve positional argument as PRD name or path
			if len(args) > 0 {
				arg := args[0]
				if strings.HasSuffix(arg, ".json") || strings.HasSuffix(arg, "/") {
					opts.PRDPath = arg
				} else {
					opts.PRDPath = fmt.Sprintf(".chief/prds/%s/prd.json", arg)
				}
			}
			runTUIWithOptions(opts)
			return nil
		},
	}

	// Set custom version template to match previous output format
	rootCmd.SetVersionTemplate("chief version {{.Version}}\n")

	// Root flags (TUI mode)
	rootCmd.Flags().IntVarP(&opts.MaxIterations, "max-iterations", "n", 0, "Set maximum iterations (default: dynamic)")
	rootCmd.Flags().BoolVar(&opts.NoRetry, "no-retry", false, "Disable auto-retry on Claude crashes")
	rootCmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "Show raw Claude output in log")
	rootCmd.Flags().BoolVar(&opts.Merge, "merge", false, "Auto-merge progress on conversion conflicts")
	rootCmd.Flags().BoolVar(&opts.Force, "force", false, "Auto-overwrite on conversion conflicts")

	// Subcommands
	rootCmd.AddCommand(newNewCmd())
	rootCmd.AddCommand(newEditCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newLoginCmd())
	rootCmd.AddCommand(newLogoutCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newWiggumCmd())

	// Custom help for root command only (subcommands use default Cobra help)
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if c != rootCmd {
			defaultHelp(c, args)
			return
		}
		fmt.Print(`Chief - Autonomous PRD Agent

Usage:
  chief [options] [<name>|<path/to/prd.json>]
  chief <command> [arguments]

Commands:
  new [name] [context]       Create a new PRD interactively
  edit [name] [options]      Edit an existing PRD interactively
  status [name]              Show progress for a PRD (default: main)
  list                       List all PRDs with progress
  update                     Update Chief to the latest version
  login                      Authenticate with chiefloop.com
  logout                     Log out and deauthorize this device
  serve                      Start headless daemon for web app
  update                     Update Chief to the latest version

Options:
  --max-iterations N, -n N   Set maximum iterations (default: dynamic)
  --no-retry                 Disable auto-retry on Claude crashes
  --verbose                  Show raw Claude output in log
  --merge                    Auto-merge progress on conversion conflicts
  --force                    Auto-overwrite on conversion conflicts
  -h, --help                 Show this help message
  -v, --version              Show version number

Examples:
  chief                      Launch TUI with default PRD (.chief/prds/main/)
  chief auth                 Launch TUI with named PRD (.chief/prds/auth/)
  chief ./my-prd.json        Launch TUI with specific PRD file
  chief -n 20                Launch with 20 max iterations
  chief --max-iterations=5 auth
                             Launch auth PRD with 5 max iterations
  chief --verbose            Launch with raw Claude output visible
  chief new                  Create PRD in .chief/prds/main/
  chief new auth             Create PRD in .chief/prds/auth/
  chief new auth "JWT authentication for REST API"
                             Create PRD with context hint
  chief edit                 Edit PRD in .chief/prds/main/
  chief edit auth            Edit PRD in .chief/prds/auth/
  chief edit auth --merge    Edit and auto-merge progress
  chief status               Show progress for default PRD
  chief status auth          Show progress for auth PRD
  chief list                 List all PRDs with progress
  chief update               Update to the latest version
  chief --version            Show version number
`)
	})

	return rootCmd
}

func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new [name] [context...]",
		Short: "Create a new PRD interactively",
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			opts := cmd.NewOptions{}
			if len(args) > 0 {
				opts.Name = args[0]
			}
			if len(args) > 1 {
				opts.Context = strings.Join(args[1:], " ")
			}
			return cmd.RunNew(opts)
		},
	}
}

func newEditCmd() *cobra.Command {
	editOpts := &cmd.EditOptions{}

	editCmd := &cobra.Command{
		Use:   "edit [name]",
		Short: "Edit an existing PRD interactively",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) > 0 {
				editOpts.Name = args[0]
			}
			return cmd.RunEdit(*editOpts)
		},
	}

	editCmd.Flags().BoolVar(&editOpts.Merge, "merge", false, "Auto-merge progress on conversion conflicts")
	editCmd.Flags().BoolVar(&editOpts.Force, "force", false, "Auto-overwrite on conversion conflicts")

	return editCmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show progress for a PRD (default: main)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			opts := cmd.StatusOptions{}
			if len(args) > 0 {
				opts.Name = args[0]
			}
			return cmd.RunStatus(opts)
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all PRDs with progress",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			return cmd.RunList(cmd.ListOptions{})
		},
	}
}

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update Chief to the latest version",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			return cmd.RunUpdate(cmd.UpdateOptions{Version: Version})
		},
	}
}

func newLoginCmd() *cobra.Command {
	loginOpts := &cmd.LoginOptions{}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with chiefloop.com",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			return cmd.RunLogin(*loginOpts)
		},
	}

	loginCmd.Flags().StringVar(&loginOpts.DeviceName, "name", "", "Override device name (default: hostname)")
	loginCmd.Flags().StringVar(&loginOpts.SetupToken, "setup-token", "", "One-time setup token for automated auth")

	return loginCmd
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out and deauthorize this device",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			return cmd.RunLogout(cmd.LogoutOptions{})
		},
	}
}

func newServeCmd() *cobra.Command {
	serveOpts := &cmd.ServeOptions{}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start headless daemon for web app",
		Long:  "Starts a headless daemon that connects to chiefloop.com via WebSocket and accepts commands from the web app.",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			serveOpts.Version = Version
			return cmd.RunServe(*serveOpts)
		},
	}

	serveCmd.Flags().StringVar(&serveOpts.Workspace, "workspace", "", "Path to workspace directory (required)")
	serveCmd.Flags().StringVar(&serveOpts.DeviceName, "name", "", "Override device name for this session")
	serveCmd.Flags().StringVar(&serveOpts.LogFile, "log-file", "", "Path to log file (default: stdout)")

	return serveCmd
}

func newWiggumCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "wiggum",
		Short:  "Bake 'em away, toys!",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(c *cobra.Command, args []string) {
			printWiggum()
		},
	}
}

// findAvailablePRD looks for any available PRD in .chief/prds/
// Returns the path to the first PRD found, or empty string if none exist.
func findAvailablePRD() string {
	prdsDir := ".chief/prds"
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

// listAvailablePRDs returns all PRD names in .chief/prds/
func listAvailablePRDs() []string {
	prdsDir := ".chief/prds"
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

func runTUIWithOptions(opts *TUIOptions) {
	prdPath := opts.PRDPath

	// If no PRD specified, try to find one
	if prdPath == "" {
		// Try "main" first
		mainPath := ".chief/prds/main/prd.json"
		if _, err := os.Stat(mainPath); err == nil {
			prdPath = mainPath
		} else {
			// Look for any available PRD
			prdPath = findAvailablePRD()
		}

		// If still no PRD found, run first-time setup
		if prdPath == "" {
			cwd, _ := os.Getwd()
			showGitignore := git.IsGitRepo(cwd) && !git.IsChiefIgnored(cwd)

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
			opts.PRDPath = fmt.Sprintf(".chief/prds/%s/prd.json", result.PRDName)
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
					fmt.Printf("  chief %s\n", name)
				}
				fmt.Println()
			}
			fmt.Println("Or create a new one:")
			fmt.Println("  chief new               # Create default PRD")
			fmt.Println("  chief new <name>        # Create named PRD")
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
			opts.PRDPath = fmt.Sprintf(".chief/prds/%s/prd.json", finalApp.PostExitPRD)
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
			opts.PRDPath = fmt.Sprintf(".chief/prds/%s/prd.json", finalApp.PostExitPRD)
			runTUIWithOptions(opts)
		}
	}
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
                               - Chief Wiggum
`
	fmt.Print(art)
}
