# Chief

<p align="center">
  <img src="assets/hero.png" alt="Chief" width="500">
</p>

Build big projects with Claude. Chief breaks your work into tasks and runs Claude Code in a loop until they're done.

**[Documentation](https://minicodemonkey.github.io/chief/)** · **[Quick Start](https://minicodemonkey.github.io/chief/guide/quick-start)**

![Chief TUI](https://minicodemonkey.github.io/chief/images/tui-screenshot.png)

## Install

```bash
brew install minicodemonkey/chief/chief
```

Or via install script:

```bash
curl -fsSL https://raw.githubusercontent.com/MiniCodeMonkey/chief/refs/heads/main/install.sh | sh
```

## Usage

```bash
# Create a new project
chief new

# Launch the TUI and press 's' to start
chief
```

Chief runs Claude in a [Ralph Wiggum loop](https://ghuntley.com/ralph/): each iteration starts with a fresh context window, but progress is persisted between runs. This lets Claude work through large projects without hitting context limits.

## How It Works

1. **Describe your project** as a series of tasks
2. **Chief runs Claude** in a loop, one task at a time
3. **One commit per task** — clean git history, easy to review

See the [documentation](https://minicodemonkey.github.io/chief/concepts/how-it-works) for details.

## Requirements

- **[Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)** or **[Codex CLI](https://developers.openai.com/codex/cli/reference)** installed and authenticated

Use Claude by default, or configure Codex in `.chief/config.yaml`:

```yaml
agent:
  provider: codex
  cliPath: /usr/local/bin/codex   # optional
```

Or run with `chief --agent codex` or set `CHIEF_AGENT=codex`.

## License

MIT

## Acknowledgments

- [snarktank/ralph](https://github.com/snarktank/ralph) — The original Ralph implementation that inspired this project
- [Geoffrey Huntley](https://ghuntley.com/ralph/) — For coining the "Ralph Wiggum loop" pattern
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Terminal styling
