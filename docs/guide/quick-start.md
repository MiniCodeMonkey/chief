# Quick Start

Get Chief running in under 5 minutes.

## Prerequisites

Before you begin, make sure you have:

- **Claude Code CLI** installed and authenticated. [Install Claude Code →](https://github.com/anthropics/claude-code)
- A project you want to work on (or create a new one)

::: tip Verify Claude Code is working
Run `claude --version` in your terminal to confirm Claude Code is installed.
:::

## Step 1: Install Chief

Choose your preferred installation method:

::: code-group

```bash [Homebrew (Recommended)]
brew install minicodemonkey/chief/chief
```

```bash [Install Script]
curl -fsSL https://raw.githubusercontent.com/minicodemonkey/chief/main/install.sh | bash
```

```bash [From Source]
git clone https://github.com/minicodemonkey/chief.git
cd chief
go build -o chief ./cmd/chief
mv chief /usr/local/bin/
```

:::

Verify the installation:

```bash
chief --version
```

## Step 2: Create Your First PRD

Navigate to your project directory and initialize Chief:

```bash
cd your-project
chief init
```

This creates a `.chief/` directory with a sample PRD to get you started. The PRD includes:

- `prd.md` - Human-readable project requirements
- `prd.json` - Machine-readable user stories for Chief to execute

::: tip Customize your PRD
Open `.chief/prds/default/prd.md` in your editor to write your own user stories. Chief will parse it into `prd.json` automatically.
:::

## Step 3: Run the Loop

Start the autonomous loop:

```bash
chief
```

That's it! Chief takes over from here.

## Step 4: Watch It Work

Chief launches a beautiful TUI (Terminal User Interface) that shows:

- **Current Story** - Which user story is being implemented
- **Live Output** - Real-time streaming from Claude
- **Progress** - How many stories are complete vs remaining

### Keyboard Controls

| Key | Action |
|-----|--------|
| `Tab` | Switch between output and log views |
| `↑/↓` | Scroll through output |
| `q` | Quit Chief |

::: info Hands-off operation
Chief runs autonomously. You can watch the progress or walk away - it will complete your PRD and play a sound when done.
:::

## What's Next?

Now that you've run your first Chief loop, explore these resources:

- [Installation Guide](/guide/installation) - Detailed installation options for all platforms
- [How Chief Works](/concepts/how-it-works) - Understand the autonomous loop
- [The Ralph Loop](/concepts/ralph-loop) - Deep dive into the execution model
- [PRD Format](/concepts/prd-format) - Write effective PRDs
- [CLI Reference](/reference/cli) - All available commands and options
