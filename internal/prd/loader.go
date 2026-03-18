package prd

// LoadPRD reads and parses a PRD markdown file from the given path.
func LoadPRD(path string) (*PRD, error) {
	return ParseMarkdownPRD(path)
}
