...
- **Multi-Agent Workflows**: Support for coordinated, simultaneous execution of code generation, review, and refactoring within a globally aware project state.
- **Interactive Control**: Full authority to stop, pause, or resume any AI process, ensuring the developer remains the "Melliza" of the operation.
- **Extensible & Portable**: A modular architecture that supports custom skills and tools, making it portable across different development environments.
- **Multi-Project Management**: Seamless switching and orchestration across multiple active PRDs and projects.

---

## The .melliza Directory
Melliza stores all of its state in a single `.melliza/` directory at the root of your project. This is a deliberate design choice — there are no global config files, no hidden state in your home directory, no external databases. Everything Melliza needs lives right alongside your code.

### Directory Structure
A typical `.melliza/` directory looks like this:
...
If you want collaborators to see progress and continue where you left off, commit everything except the log files. This shares:
- `prd.md`: Your requirements, the source of truth for what to build
- `prd.json`: Story state and progress, so collaborators see what's done
- `progress.md`: Implementation history and learnings, valuable project context
The `gemini.log` files are large, regenerated each run, and only useful for debugging.
