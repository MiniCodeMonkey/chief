# Chief

<p align="center">
  <img src="assets/hero.png" alt="Chief" width="500">
</p>

Build big projects with AI coding agents. Chief breaks your work into tasks and runs your chosen agent in a loop until they're done.

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

Chief runs your chosen agent in a [Ralph Wiggum loop](https://ghuntley.com/ralph/): each iteration starts with a fresh context window, but progress is persisted between runs. This lets the agent work through large projects without hitting context limits.

## Supported Agents

Chief supports multiple coding agents:

- **Claude Code** (default) — Anthropic's CLI agent
- **Pi** — The Pi coding agent from badlogic

### Configuration

Set the agent in your project's `.chief/config.yaml`:

```yaml
agent: pi  # Use Pi instead of Claude
```

Or set via environment variable:

```bash
export CHIEF_AGENT=pi
```

## Requirements

- At least one supported agent installed and authenticated:
  - [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (for Claude agent)
  - [Pi CLI](https://github.com/badlogic/pi-mono) (for Pi agent)

## License

MIT

## Acknowledgments

- [snarktank/ralph](https://github.com/snarktank/ralph) — The original Ralph implementation that inspired this project
- [Geoffrey Huntley](https://ghuntley.com/ralph/) — For coining the "Ralph Wiggum loop" pattern
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Terminal styling
