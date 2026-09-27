package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/minicodemonkey/chief/internal/agent"
	"github.com/minicodemonkey/chief/internal/cmd"
	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/git"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/prd"
	"github.com/minicodemonkey/chief/internal/tui"
	"github.com/spf13/cobra"
)

var (
	// Version is set from main.go
	Version = "dev"

	// Persistent flags (global)
	flagAgent     string
	flagAgentPath string
	flagVerbose   bool

	// Local flags (root/TUI only)
	flagMaxIterations int
	flagNoRetry       bool
	flagMerge         bool
	flagForce         bool
)

var rootCmd = &cobra.Command{
	Use:   "chief [prd-name|path]",
	Short: "Chief - Autonomous PRD Agent",
	Long:  "Chief orchestrates AI agents to implement PRDs (Product Requirements Documents) autonomously.",
	RunE:  runTUI,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// Persistent flags (available to all subcommands)
	rootCmd.PersistentFlags().StringVar(&flagAgent, "agent", "", "Agent CLI to use: claude (default), codex, opencode, cursor, gemini, or copilot")
	rootCmd.PersistentFlags().StringVar(&flagAgentPath, "agent-path", "", "Custom path to agent CLI binary")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Show raw agent output in log")

	// Local flags (root/TUI only)
	rootCmd.Flags().IntVarP(&flagMaxIterations, "max-iterations", "n", 0, "Set maximum iterations (default: dynamic)")
	rootCmd.Flags().BoolVar(&flagNoRetry, "no-retry", false, "Disable auto-retry on agent crashes")
	rootCmd.Flags().BoolVar(&flagMerge, "merge", false, "Auto-merge progress on conversion conflicts")
	rootCmd.Flags().BoolVar(&flagForce, "force", false, "Auto-overwrite on conversion conflicts")

	// Wiggum easter egg
	rootCmd.AddCommand(&cobra.Command{
		Use:    "wiggum",
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			printWiggum()
		},
	})
}

// Execute is the main entry point called from main.go
func Execute(version string) {
	Version = version
	rootCmd.Version = version

	// Create the --version flag, then disable the -v shorthand to avoid
	// conflict with potential future --verbose shorthand
	rootCmd.InitDefaultVersionFlag()
	rootCmd.Flags().Lookup("version").Shorthand = ""

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// resolveProvider loads config and resolves the agent provider.
func resolveProvider() (loop.Provider, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to load .chief/config.yaml: %w", err)
	}
	provider, err := agent.Resolve(flagAgent, flagAgentPath, cfg)
	if err != nil {
		return nil, err
	}
	if err := agent.CheckInstalled(provider); err != nil {
		return nil, err
	}
	return provider, nil
}

// findAvailablePRD looks for any available PRD in .chief/prds/
func findAvailablePRD() string {
	prdsDir := ".chief/prds"
	entries, err := os.ReadDir(prdsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			prdPath := filepath.Join(prdsDir, entry.Name(), "prd.md")
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
			prdPath := filepath.Join(prdsDir, entry.Name(), "prd.md")
			if _, err := os.Stat(prdPath); err == nil {
				names = append(names, entry.Name())
			}
		}
	}
	return names
}

func runTUI(command *cobra.Command, args []string) error {
	// Validate --max-iterations (0 means dynamic/unset, negative is invalid)
	if flagMaxIterations < 0 {
		return fmt.Errorf("--max-iterations must be at least 1")
	}

	// Non-blocking version check on startup
	cmd.CheckVersionOnStartup(Version)

	provider, err := resolveProvider()
	if err != nil {
		return err
	}

	// Resolve PRD path from positional arg
	var prdPath string
	if len(args) > 0 {
		arg := args[0]
		if strings.HasSuffix(arg, ".md") || strings.HasSuffix(arg, ".json") || strings.HasSuffix(arg, "/") {
			prdPath = arg
		} else {
			prdPath = fmt.Sprintf(".chief/prds/%s/prd.md", arg)
		}
	}

	return runTUIWithOptions(prdPath, provider)
}

func runTUIWithOptions(prdPath string, provider loop.Provider) error {
	// If no PRD specified, try to find one
	if prdPath == "" {
		mainPath := ".chief/prds/main/prd.md"
		if _, err := os.Stat(mainPath); err == nil {
			prdPath = mainPath
		} else {
			prdPath = findAvailablePRD()
		}

		// If still no PRD found, run first-time setup
		if prdPath == "" {
			cwd, _ := os.Getwd()
			showGitignore := git.IsGitRepo(cwd) && !git.IsChiefIgnored(cwd)

			result, err := tui.RunFirstTimeSetup(cwd, showGitignore)
			if err != nil {
				return err
			}
			if result.Cancelled {
				return nil
			}

			cfg := config.Default()
			cfg.OnComplete.Push = result.PushOnComplete
			cfg.OnComplete.CreatePR = result.CreatePROnComplete
			if err := config.Save(cwd, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save config: %v\n", err)
			}

			newOpts := cmd.NewOptions{
				Name:     result.PRDName,
				Provider: provider,
			}
			if err := cmd.RunNew(newOpts); err != nil {
				return err
			}

			newPath := fmt.Sprintf(".chief/prds/%s/prd.md", result.PRDName)
			return runTUIWithOptions(newPath, provider)
		}
	}

	prdDir := filepath.Dir(prdPath)

	// Auto-migrate: if prd.json exists alongside prd.md, migrate status
	jsonPath := filepath.Join(prdDir, "prd.json")
	if _, err := os.Stat(jsonPath); err == nil {
		fmt.Println("Migrating status from prd.json to prd.md...")
		if err := prd.MigrateFromJSON(prdDir); err != nil {
			fmt.Printf("Warning: migration failed: %v\n", err)
		} else {
			fmt.Println("Migration complete (prd.json renamed to prd.json.bak).")
		}
	}

	app, err := tui.NewAppWithOptions(prdPath, flagMaxIterations, provider)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") {
			fmt.Printf("PRD not found: %s\n", prdPath)
			fmt.Println()
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
			os.Exit(1)
		}
		return err
	}

	if flagVerbose {
		app.SetVerbose(true)
	}
	if flagNoRetry {
		app.DisableRetry()
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	model, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	// Handle post-exit actions
	if finalApp, ok := model.(tui.App); ok {
		switch finalApp.PostExitAction {
		case tui.PostExitInit:
			newOpts := cmd.NewOptions{
				Name:     finalApp.PostExitPRD,
				Provider: provider,
			}
			if err := cmd.RunNew(newOpts); err != nil {
				return err
			}
			newPath := fmt.Sprintf(".chief/prds/%s/prd.md", finalApp.PostExitPRD)
			return runTUIWithOptions(newPath, provider)

		case tui.PostExitEdit:
			editOpts := cmd.EditOptions{
				Name:     finalApp.PostExitPRD,
				Provider: provider,
			}
			if err := cmd.RunEdit(editOpts); err != nil {
				return err
			}
			editPath := fmt.Sprintf(".chief/prds/%s/prd.md", finalApp.PostExitPRD)
			return runTUIWithOptions(editPath, provider)
		}
	}

	return nil
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
