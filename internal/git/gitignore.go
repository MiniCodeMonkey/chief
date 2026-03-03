package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsMellizaIgnored checks if .melliza is gitignored either locally or globally.
// Returns true if .melliza is already ignored, false otherwise.
func IsMellizaIgnored(dir string) bool {
	// Use git check-ignore which respects both local and global gitignore
	cmd := exec.Command("git", "check-ignore", "-q", ".melliza")
	cmd.Dir = dir
	err := cmd.Run()
	// Exit code 0 means it IS ignored, exit code 1 means it's NOT ignored
	return err == nil
}

// AddMellizaToGitignore adds .melliza to the local .gitignore file.
// Creates the file if it doesn't exist.
func AddMellizaToGitignore(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	// Check if .gitignore exists and if .melliza is already in it
	if _, err := os.Stat(gitignorePath); err == nil {
		// File exists, check if .melliza is already there
		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			return fmt.Errorf("failed to read .gitignore: %w", err)
		}

		// Check each line for .melliza entry
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == ".melliza" || trimmed == ".melliza/" {
				// Already present
				return nil
			}
		}

		// Append to existing file
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open .gitignore: %w", err)
		}
		defer f.Close()

		// Add newline before if file doesn't end with one
		if len(content) > 0 && content[len(content)-1] != '\n' {
			if _, err := f.WriteString("\n"); err != nil {
				return fmt.Errorf("failed to write to .gitignore: %w", err)
			}
		}

		if _, err := f.WriteString(".melliza/\n"); err != nil {
			return fmt.Errorf("failed to write to .gitignore: %w", err)
		}
	} else if os.IsNotExist(err) {
		// Create new .gitignore file
		if err := os.WriteFile(gitignorePath, []byte(".melliza/\n"), 0644); err != nil {
			return fmt.Errorf("failed to create .gitignore: %w", err)
		}
	} else {
		return fmt.Errorf("failed to check .gitignore: %w", err)
	}

	return nil
}

// PromptAddMellizaToGitignore asks the user if they want to add .melliza to .gitignore.
// Returns true if the user wants to add it, false otherwise.
func PromptAddMellizaToGitignore() bool {
	fmt.Println("Would you like to add .melliza to .gitignore?")
	fmt.Println("This keeps your PRD plans local and out of version control.")
	fmt.Println("(Not required, but recommended if you prefer local-only plans)")
	fmt.Print("\nAdd .melliza to .gitignore? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
