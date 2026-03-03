<p align="center">
  <img src="./assets/hero.png" alt="Melliza Hero" width="600" />
</p>

<h1 align="center">Melliza</h1>

<p align="center">
  <strong>Autonomous agent loop for the Gemini CLI</strong><br />
  Turn your PRDs into working code, one story at a time.
</p>

<p align="center">
  <a href="https://github.com/lvcoi/melliza/blob/main/LICENSE"><img src="https://img.shields.io/github/license/lvcoi/melliza?style=flat-square" alt="License" /></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat-square&logo=go" alt="Go Version" /></a>
  <a href="https://melliza.dev"><img src="https://img.shields.io/badge/docs-melliza.dev-blueviolet?style=flat-square" alt="Documentation" /></a>
</p>

---

Melliza is an autonomous agent loop that orchestrates the **Gemini CLI** to work through user stories in a **Product Requirements Document (PRD)**. 

Built on the "Ralph Wiggum loop" pattern, Melliza breaks down complex project requirements into manageable tasks, invokes Gemini to implement them one by one, and maintains persistent progress tracking.

<p align="center">
  <img src="./docs/public/images/tui-screenshot.png" alt="Melliza TUI" width="800" style="border-radius: 10px;" />
</p>

## ✨ Core Features

*   🤖 **Autonomous Loop**: Orchestrates Gemini CLI to work through user stories without manual intervention.
*   📄 **PRD-Driven Development**: Work directly from human-readable `prd.md` files.
*   📈 **Persistent Progress**: Progress is tracked in `prd.json` and `progress.md`, ensuring work can be resumed across sessions.
*   🖥️ **TUI Dashboard**: A real-time terminal user interface to monitor Gemini's progress, logs, and diffs.
*   🌿 **Smart Worktrees**: Automatically creates git branches or worktrees for each PRD to keep your main workspace clean.
*   ✅ **Auto-Commit & Test**: Gemini implements the story, runs your project's tests, and commits changes automatically.

## 🚀 Quick Start

### 1. Install Melliza

```bash
# Via Homebrew
brew install lvcoi/melliza/melliza

# Or via install script
curl -fsSL https://raw.githubusercontent.com/lvcoi/melliza/main/install.sh | bash
```

### 2. Prerequisites

Ensure you have the [Gemini CLI](https://github.com/google/gemini-cli) installed and your `GEMINI_API_KEY` configured.

### 3. Usage

```bash
# Create a new PRD (launches interactive session)
melliza new my-project

# Run the loop
melliza my-project
```

## ⚙️ How it Works

Melliza follows a simple, repeatable cycle:

1.  **Plan**: Identify the next incomplete story in `prd.json`.
2.  **Execute**: Invoke Gemini CLI with a specialized system prompt and the current story context.
3.  **Monitor**: Parse Gemini's `stream-json` output to update the TUI and progress files.
4.  **Finalize**: Once Gemini completes the story, Melliza moves to the next one.

## 📚 Documentation

Full documentation is available at [**melliza.dev**](https://melliza.dev).

*   [Installation Guide](https://melliza.dev/guide/installation)
*   [Quick Start](https://melliza.dev/guide/quick-start)
*   [How it Works](https://melliza.dev/concepts/how-it-works)
*   [PRD Format](https://melliza.dev/concepts/prd-format)

## 🛠️ Development

```bash
make build  # Build the binary
make test   # Run all tests
make run    # Launch the TUI (dev mode)
```

## 📄 License

This project is licensed under the [MIT License](./LICENSE).
