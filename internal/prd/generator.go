package prd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

// Colors duplicated from tui/styles.go to avoid import cycle (tui → git → prd).
var (
	cPrimary = lipgloss.Color("#00D7FF")
	cSuccess = lipgloss.Color("#5AF78E")
	cMuted   = lipgloss.Color("#6C7086")
	cBorder  = lipgloss.Color("#45475A")
	cText    = lipgloss.Color("#CDD6F4")
)

// waitingJokes are shown on a rotating basis during long-running operations.
var waitingJokes = []string{
	"Why do programmers prefer dark mode? Because light attracts bugs.",
	"There are only 10 types of people: those who understand binary and those who don't.",
	"A SQL query walks into a bar, sees two tables and asks... 'Can I JOIN you?'",
	"!false — it's funny because it's true.",
	"A programmer's wife says: 'Go to the store and get a gallon of milk. If they have eggs, get a dozen.' He returns with 12 gallons of milk.",
	"Why do Java developers wear glasses? Because they can't C#.",
	"There's no place like 127.0.0.1.",
	"Algorithm: a word used by programmers when they don't want to explain what they did.",
	"It works on my machine. Ship it!",
	"99 little bugs in the code, 99 little bugs. Take one down, patch it around... 127 little bugs in the code.",
	"The best thing about a boolean is that even if you're wrong, you're only off by a bit.",
	"Debugging is like being the detective in a crime movie where you are also the murderer.",
	"How many programmers does it take to change a light bulb? None, that's a hardware problem.",
	"I asked the AI to write a PRD. It wrote a PRD about writing PRDs.",
	"You're absolutely right. That's a great point. I completely agree. — Claude, before doing what it was already going to do",
	"The AI said it was 95% confident. It was not.",
	"Prompt engineering: the art of saying 'no really, do what I said' in 47 different ways.",
	"The LLM hallucinated a library that doesn't exist. Honestly, the API looked pretty good though.",
	"AI will replace programmers any day now. — programmers, every year since 2022",
	"Homer Simpson: 'To start, press any key.' Where's the ANY key?!",
	"Homer Simpson: 'Kids, you tried your best and you failed miserably. The lesson is, never try.'",
	"The code works and nobody knows why. The code breaks and nobody knows why.",
	"Frink: 'You've got to listen to me! Elementary chaos theory tells us that all robots will eventually turn against their masters!'",
}

// ConvertOptions contains configuration for PRD conversion.
type ConvertOptions struct {
	PRDDir   string // Directory containing prd.md
	Merge    bool   // Auto-merge progress on conversion conflicts
	Force    bool   // Auto-overwrite on conversion conflicts
	// RunConversion runs the agent to convert prd.md to JSON. Required.
	RunConversion func(absPRDDir string) (string, error)
	// RunFixJSON runs the agent to fix invalid JSON. Required.
	RunFixJSON func(prompt string) (string, error)
}

// ProgressConflictChoice represents the user's choice when a progress conflict is detected.
type ProgressConflictChoice int

const (
	ChoiceMerge     ProgressConflictChoice = iota // Keep status for matching story IDs
	ChoiceOverwrite                               // Discard all progress
	ChoiceCancel                                  // Cancel conversion
)

// Convert converts prd.md to prd.json using the configured agent one-shot.
// This function is called:
// - After chief new (new PRD creation)
// - After chief edit (PRD modification)
// - Before chief run if prd.md is newer than prd.json
//
// Progress protection:
// - If prd.json has progress (passes: true or inProgress: true) and prd.md changed:
//   - opts.Merge: auto-merge, preserving status for matching story IDs
//   - opts.Force: auto-overwrite, discarding all progress
//   - Neither: prompt the user with Merge/Overwrite/Cancel options
func Convert(opts ConvertOptions) error {
	prdMdPath := filepath.Join(opts.PRDDir, "prd.md")
	prdJsonPath := filepath.Join(opts.PRDDir, "prd.json")

	// Check if prd.md exists
	if _, err := os.Stat(prdMdPath); os.IsNotExist(err) {
		return fmt.Errorf("prd.md not found in %s", opts.PRDDir)
	}

	// Resolve absolute path so the prompt can specify exact file locations
	absPRDDir, err := filepath.Abs(opts.PRDDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	if opts.RunConversion == nil || opts.RunFixJSON == nil {
		return fmt.Errorf("conversion requires RunConversion and RunFixJSON callbacks")
	}

	// Check for existing progress before conversion
	var existingPRD *PRD
	hasProgress := false
	if existing, err := LoadPRD(prdJsonPath); err == nil {
		existingPRD = existing
		hasProgress = HasProgress(existing)
	}

	// Run agent to convert prd.md → JSON string
	rawJSON, err := opts.RunConversion(absPRDDir)
	if err != nil {
		return err
	}

	// Clean up output (strip markdown fences if any)
	cleanedJSON := cleanJSONOutput(rawJSON)

	// Parse and validate
	newPRD, err := parseAndValidatePRD(cleanedJSON)
	if err != nil {
		// Retry once: ask agent to fix the invalid JSON
		fmt.Println("Conversion produced invalid JSON, retrying...")
		fmt.Printf("Raw output:\n---\n%s\n---\n", cleanedJSON)
		fixedJSON, retryErr := opts.RunFixJSON(fixPromptForRetry(cleanedJSON, err))
		if retryErr != nil {
			return fmt.Errorf("conversion retry failed: %w", retryErr)
		}

		cleanedJSON = cleanJSONOutput(fixedJSON)
		newPRD, err = parseAndValidatePRD(cleanedJSON)
		if err != nil {
			return fmt.Errorf("conversion produced invalid JSON after retry:\n---\n%s\n---\n%w", cleanedJSON, err)
		}
	}

	// Re-save through Go's JSON encoder to guarantee proper escaping and formatting
	normalizedContent, err := json.MarshalIndent(newPRD, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PRD: %w", err)
	}

	// Handle progress protection if existing prd.json has progress
	if hasProgress && existingPRD != nil {
		choice := ChoiceOverwrite // Default to overwrite if no progress

		if opts.Merge {
			choice = ChoiceMerge
		} else if opts.Force {
			choice = ChoiceOverwrite
		} else {
			// Prompt user for choice
			var promptErr error
			choice, promptErr = promptProgressConflict(existingPRD, newPRD)
			if promptErr != nil {
				return fmt.Errorf("failed to prompt for choice: %w", promptErr)
			}
		}

		switch choice {
		case ChoiceCancel:
			return fmt.Errorf("conversion cancelled by user")
		case ChoiceMerge:
			// Merge progress from existing PRD into new PRD
			MergeProgress(existingPRD, newPRD)
			// Re-marshal with merged progress
			mergedContent, err := json.MarshalIndent(newPRD, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal merged PRD: %w", err)
			}
			normalizedContent = mergedContent
		case ChoiceOverwrite:
			// Use the new PRD as-is (no progress)
		}
	}

	// Write the final normalized prd.json
	if err := os.WriteFile(prdJsonPath, append(normalizedContent, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write prd.json: %w", err)
	}

	fmt.Println(lipgloss.NewStyle().Foreground(cSuccess).Render("✓ PRD converted successfully"))
	return nil
}

// fixPromptForRetry builds the prompt for the agent to fix invalid JSON.
func fixPromptForRetry(badJSON string, validationErr error) string {
	return fmt.Sprintf(
		"The following JSON is invalid. The error is: %s\n\n"+
			"Fix the JSON (pay special attention to escaping double quotes inside string values with backslashes) "+
			"and return ONLY the corrected JSON — no markdown fences, no explanation.\n\n%s",
		validationErr.Error(), badJSON,
	)
}

// parseAndValidatePRD unmarshals a JSON string and validates it as a PRD.
func parseAndValidatePRD(jsonStr string) (*PRD, error) {
	var prd PRD
	if err := json.Unmarshal([]byte(jsonStr), &prd); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	if prd.Project == "" {
		return nil, fmt.Errorf("prd.json missing required 'project' field")
	}
	if len(prd.UserStories) == 0 {
		return nil, fmt.Errorf("prd.json has no user stories")
	}
	return &prd, nil
}

// loadAndValidateConvertedPRD loads prd.json from disk and validates it can be parsed as a PRD.
func loadAndValidateConvertedPRD(prdJsonPath string) (*PRD, error) {
	data, err := os.ReadFile(prdJsonPath)
	if err != nil {
		return nil, err
	}
	return parseAndValidatePRD(string(data))
}

// getTerminalWidth returns the current terminal width, defaulting to 80.
func getTerminalWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// wrapText wraps text to the given width at word boundaries.
func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := words[0]
	for _, w := range words[1:] {
		if len(line)+1+len(w) <= width {
			line += " " + w
		} else {
			lines = append(lines, line)
			line = w
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

// renderProgressBar renders a progress bar based on elapsed time vs estimated duration.
// Caps at 95% to avoid showing 100% prematurely.
func renderProgressBar(elapsed time.Duration, width int) string {
	const estimatedDuration = 90 * time.Second

	progress := elapsed.Seconds() / estimatedDuration.Seconds()
	if progress > 0.95 {
		progress = 0.95
	}
	if progress < 0 {
		progress = 0
	}

	pct := int(progress * 100)
	pctStr := fmt.Sprintf("%d%%", pct)

	barWidth := width - len(pctStr) - 2 // 2 for gap between bar and percentage
	if barWidth < 10 {
		barWidth = 10
	}

	fillWidth := int(float64(barWidth) * progress)
	emptyWidth := barWidth - fillWidth

	fill := lipgloss.NewStyle().Foreground(cSuccess).Render(strings.Repeat("█", fillWidth))
	empty := lipgloss.NewStyle().Foreground(cMuted).Render(strings.Repeat("░", emptyWidth))
	styledPct := lipgloss.NewStyle().Foreground(cMuted).Render(pctStr)

	return fill + empty + "  " + styledPct
}

// renderActivityLine renders a line with a cyan dot, activity text, and right-aligned elapsed time.
func renderActivityLine(activity string, elapsed time.Duration, contentWidth int) string {
	icon := lipgloss.NewStyle().Foreground(cPrimary).Render("●")
	elapsedFmt := formatElapsed(elapsed)
	elapsedStr := lipgloss.NewStyle().Foreground(cMuted).Render(elapsedFmt)

	// Truncate activity if it would overflow
	maxDescWidth := contentWidth - 2 - len(elapsedFmt) - 2 // icon+space, elapsed, gap
	if len(activity) > maxDescWidth && maxDescWidth > 3 {
		activity = activity[:maxDescWidth-1] + "…"
	}

	descStr := lipgloss.NewStyle().Foreground(cText).Render(activity)
	leftPart := icon + " " + descStr
	rightPart := elapsedStr
	gap := contentWidth - lipgloss.Width(leftPart) - lipgloss.Width(rightPart)
	if gap < 1 {
		gap = 1
	}
	return leftPart + strings.Repeat(" ", gap) + rightPart
}

// renderProgressBox builds the full lipgloss-styled progress panel with progress bar and joke.
func renderProgressBox(title, activity string, elapsed time.Duration, joke string, panelWidth int) string {
	contentWidth := panelWidth - 6 // 2 border + 4 padding (2 each side)
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Header: "chief  <title>"
	chiefStr := lipgloss.NewStyle().Bold(true).Foreground(cPrimary).Render("chief")
	titleStr := lipgloss.NewStyle().Foreground(cText).Render(title)
	header := chiefStr + "  " + titleStr

	// Divider
	divider := lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", contentWidth))

	// Activity + progress bar
	activityLine := renderActivityLine(activity, elapsed, contentWidth)
	progressLine := renderProgressBar(elapsed, contentWidth)

	// Joke (word-wrapped, muted)
	wrappedJoke := wrapText(joke, contentWidth)
	jokeStr := lipgloss.NewStyle().Foreground(cMuted).Render(wrappedJoke)

	content := strings.Join([]string{
		header,
		divider,
		"",
		activityLine,
		progressLine,
		"",
		divider,
		jokeStr,
	}, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cPrimary).
		Padding(1, 2).
		Width(panelWidth - 2)

	return style.Render(content)
}

// renderSpinnerBox builds a simpler bordered panel for non-streaming operations.
func renderSpinnerBox(title, activity string, elapsed time.Duration, panelWidth int) string {
	contentWidth := panelWidth - 6
	if contentWidth < 20 {
		contentWidth = 20
	}

	chiefStr := lipgloss.NewStyle().Bold(true).Foreground(cPrimary).Render("chief")
	titleStr := lipgloss.NewStyle().Foreground(cText).Render(title)
	header := chiefStr + "  " + titleStr

	divider := lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", contentWidth))
	activityLine := renderActivityLine(activity, elapsed, contentWidth)

	content := strings.Join([]string{
		header,
		divider,
		"",
		activityLine,
	}, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cPrimary).
		Padding(1, 2).
		Width(panelWidth - 2)

	return style.Render(content)
}

// clearPanelLines clears N lines of previous panel output by moving cursor up and erasing.
func clearPanelLines(n int) {
	if n <= 0 {
		return
	}
	// Move to first line
	if n > 1 {
		fmt.Printf("\033[%dA", n-1)
	}
	fmt.Print("\r")
	// Clear each line
	for i := 0; i < n; i++ {
		fmt.Print("\033[2K")
		if i < n-1 {
			fmt.Print("\n")
		}
	}
	// Return to first line
	if n > 1 {
		fmt.Printf("\033[%dA", n-1)
	}
	fmt.Print("\r")
}

// repaintBox repaints the panel box, handling cursor movement for the previous frame.
// Returns the new line count for the next frame.
func repaintBox(box string, prevLines int) int {
	newLines := strings.Count(box, "\n") + 1

	// Move cursor to start of previous panel
	if prevLines > 1 {
		fmt.Printf("\033[%dA", prevLines-1)
	}
	if prevLines > 0 {
		fmt.Print("\r")
	}

	// Print the new box
	fmt.Print(box)

	// Clear leftover lines if new box is shorter
	if newLines < prevLines {
		for i := 0; i < prevLines-newLines; i++ {
			fmt.Print("\n\033[2K")
		}
		fmt.Printf("\033[%dA", prevLines-newLines)
	}

	return newLines
}

// WaitWithSpinner runs a bordered panel while waiting for a command to finish.
// Exported for use by cmd when running agent conversion.
func WaitWithSpinner(cmd *exec.Cmd, title, message string, stderr *bytes.Buffer) error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	startTime := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	termWidth := getTerminalWidth()
	panelWidth := termWidth - 2
	if panelWidth > 62 {
		panelWidth = 62
	}

	prevLines := 0

	for {
		select {
		case err := <-done:
			clearPanelLines(prevLines)
			if err != nil {
				return fmt.Errorf("Claude failed: %s", stderr.String())
			}
			return nil
		case <-ticker.C:
			box := renderSpinnerBox(title, message, time.Since(startTime), panelWidth)
			prevLines = repaintBox(box, prevLines)
		}
	}
}

// WaitWithPanel runs a full progress panel (header, activity, progress bar, jokes)
// while waiting for a command to finish. Exported for use by cmd when running agent conversion.
func WaitWithPanel(cmd *exec.Cmd, title, activity string, stderr *bytes.Buffer) error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	startTime := time.Now()
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	// Pick a random starting joke and track rotation
	jokeIndex := rand.Intn(len(waitingJokes))
	currentJoke := waitingJokes[jokeIndex]
	lastJokeChange := time.Now()

	termWidth := getTerminalWidth()
	panelWidth := termWidth - 2
	if panelWidth > 62 {
		panelWidth = 62
	}

	prevLines := 0

	for {
		select {
		case err := <-done:
			clearPanelLines(prevLines)
			if err != nil {
				return fmt.Errorf("Claude failed: %s", stderr.String())
			}
			return nil
		case <-ticker.C:
			// Rotate joke every 30 seconds
			if time.Since(lastJokeChange) >= 30*time.Second {
				jokeIndex = (jokeIndex + 1 + rand.Intn(len(waitingJokes)-1)) % len(waitingJokes)
				currentJoke = waitingJokes[jokeIndex]
				lastJokeChange = time.Now()
			}

			box := renderProgressBox(title, activity, time.Since(startTime), currentJoke, panelWidth)
			prevLines = repaintBox(box, prevLines)
		}
	}
}

// formatElapsed formats a duration as a human-readable elapsed time string.
// Examples: "0s", "5s", "1m 12s", "2m 0s"
func formatElapsed(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}

// NeedsConversion checks if prd.md is newer than prd.json, indicating conversion is needed.
// Returns true if:
// - prd.md exists and prd.json does not exist
// - prd.md exists and is newer than prd.json
// Returns false if:
// - prd.md does not exist
// - prd.json is newer than or same age as prd.md
func NeedsConversion(prdDir string) (bool, error) {
	prdMdPath := filepath.Join(prdDir, "prd.md")
	prdJsonPath := filepath.Join(prdDir, "prd.json")

	// Check if prd.md exists
	mdInfo, err := os.Stat(prdMdPath)
	if os.IsNotExist(err) {
		// No prd.md, no conversion needed
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat prd.md: %w", err)
	}

	// Check if prd.json exists
	jsonInfo, err := os.Stat(prdJsonPath)
	if os.IsNotExist(err) {
		// prd.md exists but prd.json doesn't - needs conversion
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat prd.json: %w", err)
	}

	// Both exist - compare modification times
	return mdInfo.ModTime().After(jsonInfo.ModTime()), nil
}

// cleanJSONOutput removes markdown code blocks, conversational preamble, and trims
// whitespace from Claude's output to extract the JSON object.
func cleanJSONOutput(output string) string {
	output = strings.TrimSpace(output)

	// Remove markdown code blocks if present
	if strings.HasPrefix(output, "```json") {
		output = strings.TrimPrefix(output, "```json")
	} else if strings.HasPrefix(output, "```") {
		output = strings.TrimPrefix(output, "```")
	}

	if strings.HasSuffix(output, "```") {
		output = strings.TrimSuffix(output, "```")
	}

	output = strings.TrimSpace(output)

	// If output doesn't start with '{', Claude may have added preamble text.
	// Extract the JSON object by finding the first '{' and matching closing '}'.
	if len(output) > 0 && output[0] != '{' {
		start := strings.Index(output, "{")
		if start == -1 {
			return output // No JSON object found, return as-is for error handling
		}
		// Find the matching closing brace by counting brace depth
		depth := 0
		inString := false
		escaped := false
		end := -1
		for i := start; i < len(output); i++ {
			if escaped {
				escaped = false
				continue
			}
			ch := output[i]
			if ch == '\\' && inString {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		if end != -1 {
			output = output[start : end+1]
		} else {
			// No matching closing brace; take from first '{' to end
			output = output[start:]
		}
	}

	return strings.TrimSpace(output)
}

// validateJSON checks if the given string is valid JSON.
func validateJSON(content string) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// HasProgress checks if the PRD has any progress (passes: true or inProgress: true).
func HasProgress(prd *PRD) bool {
	if prd == nil {
		return false
	}
	for _, story := range prd.UserStories {
		if story.Passes || story.InProgress {
			return true
		}
	}
	return false
}

// MergeProgress merges progress from the old PRD into the new PRD.
// For stories with matching IDs, it preserves the Passes and InProgress status.
// New stories (in newPRD but not in oldPRD) are added without progress.
// Removed stories (in oldPRD but not in newPRD) are dropped.
func MergeProgress(oldPRD, newPRD *PRD) {
	if oldPRD == nil || newPRD == nil {
		return
	}

	// Create a map of old story statuses by ID
	oldStatus := make(map[string]struct {
		passes     bool
		inProgress bool
	})
	for _, story := range oldPRD.UserStories {
		oldStatus[story.ID] = struct {
			passes     bool
			inProgress bool
		}{
			passes:     story.Passes,
			inProgress: story.InProgress,
		}
	}

	// Apply old status to matching stories in new PRD
	for i := range newPRD.UserStories {
		if status, exists := oldStatus[newPRD.UserStories[i].ID]; exists {
			newPRD.UserStories[i].Passes = status.passes
			newPRD.UserStories[i].InProgress = status.inProgress
		}
	}
}

// promptProgressConflict prompts the user to choose how to handle a progress conflict.
func promptProgressConflict(oldPRD, newPRD *PRD) (ProgressConflictChoice, error) {
	// Count stories with progress
	progressCount := 0
	for _, story := range oldPRD.UserStories {
		if story.Passes || story.InProgress {
			progressCount++
		}
	}

	// Show warning
	fmt.Println()
	fmt.Printf("⚠️  Warning: prd.json has progress (%d stories with status)\n", progressCount)
	fmt.Println()
	fmt.Println("How would you like to proceed?")
	fmt.Println()
	fmt.Println("  [m] Merge  - Keep status for matching story IDs, add new stories, drop removed stories")
	fmt.Println("  [o] Overwrite - Discard all progress and use the new PRD")
	fmt.Println("  [c] Cancel - Cancel conversion and keep existing prd.json")
	fmt.Println()
	fmt.Print("Choice [m/o/c]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ChoiceCancel, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "m", "merge":
		return ChoiceMerge, nil
	case "o", "overwrite":
		return ChoiceOverwrite, nil
	case "c", "cancel", "":
		return ChoiceCancel, nil
	default:
		fmt.Printf("Invalid choice %q, cancelling conversion.\n", input)
		return ChoiceCancel, nil
	}
}
