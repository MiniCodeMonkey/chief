---
description: Troubleshoot common Chief issues including Claude not found, permission errors, audio problems, and loop failures.
---

# Common Issues

Solutions to frequently encountered problems.

## Claude Not Found

**Symptom:** Error message about Claude Code CLI not being installed.

```
Error: Claude Code CLI not found. Please install it first.
```

**Cause:** Claude Code isn't installed or isn't in your PATH.

**Solution:**

Install Claude Code following the [official instructions](https://docs.anthropic.com/en/docs/claude-code/getting-started), then verify:

```bash
claude --version
```

## Permission Denied

**Symptom:** Claude keeps asking for permission, disrupting autonomous flow.

**Cause:** Claude Code requires explicit permission for file writes and command execution.

**Solution:**

Chief automatically runs Claude with permission prompts disabled for autonomous operation. If you're still seeing permission issues, ensure you're running Chief (not Claude directly) and that your Claude Code installation is up to date.

## No Sound on Completion

**Symptom:** Chief completes but no sound plays.

**Cause:** Audio system configuration or muted output.

**Solution:**

1. Check system volume isn't muted
2. Verify audio device is selected correctly
3. Run with `--no-sound` if audio isn't needed:

```bash
chief --no-sound
```

## PRD Not Updating

**Symptom:** Stories stay incomplete even though Claude seems to finish.

**Cause:** Claude didn't output the completion signal, or file watching failed.

**Solution:**

1. Check `claude.log` for errors:
   ```bash
   tail -100 .chief/prds/your-prd/claude.log
   ```

2. Manually mark story complete if appropriate:
   ```json
   {
     "id": "US-001",
     "passes": true,
     "inProgress": false
   }
   ```

3. Restart Chief to pick up where it left off

## Loop Not Progressing

**Symptom:** Chief runs but doesn't make progress on stories.

**Cause:** Various—Claude may be stuck, context too large, or PRD unclear.

**Solution:**

1. Check `claude.log` for what Claude is doing:
   ```bash
   tail -f .chief/prds/your-prd/claude.log
   ```

2. Simplify the current story's acceptance criteria

3. Add context to `prd.md` about the codebase

4. Try restarting Chief:
   ```bash
   # Press 'x' to stop (or Ctrl+C to quit)
   chief  # Launch TUI
   # Press 's' to start the loop
   ```

## Max Iterations Reached

**Symptom:** Chief stops with "max iterations reached" message.

**Cause:** Claude hasn't completed after the iteration limit (default 10).

**Solution:**

1. Increase the limit:
   ```bash
   chief --max-iterations 200
   ```

2. Or investigate why it's taking so many iterations:
   - Story too complex? Split it
   - Stuck in a loop? Check `claude.log`
   - Unclear acceptance criteria? Clarify them

## "No PRD Found"

**Symptom:** Error about no PRD being found.

**Cause:** Missing `.chief/prds/` directory or invalid PRD structure.

**Solution:**

1. Create a PRD:
   ```bash
   chief new
   ```

2. Or specify the PRD explicitly:
   ```bash
   chief my-feature
   ```

3. Verify structure:
   ```
   .chief/
   └── prds/
       └── my-feature/
           ├── prd.md
           └── prd.json
   ```

## Invalid JSON

**Symptom:** Error parsing `prd.json`.

**Cause:** Syntax error in the JSON file.

**Solution:**

1. Validate your JSON:
   ```bash
   cat .chief/prds/your-prd/prd.json | jq .
   ```

2. Common issues:
   - Trailing commas (not allowed in JSON)
   - Missing quotes around keys
   - Unescaped characters in strings

## Still Stuck?

If none of these solutions help:

1. Check the [FAQ](/troubleshooting/faq)
2. Search [GitHub Issues](https://github.com/minicodemonkey/chief/issues)
3. Open a new issue with:
   - Chief version (`chief --version`)
   - Your `prd.json` (sanitized)
   - Relevant `claude.log` excerpts
