---
description: Troubleshoot common Melliza issues including Gemini not found, permission errors, worktree problems, and loop failures.
---

# Common Issues

Solutions to frequently encountered problems.

## Gemini Not Found

**Symptom:** Error message about Gemini CLI not being installed.

```
Error: Gemini CLI not found. Please install it first.
```

**Cause:** Gemini CLI isn't installed or isn't in your PATH.

**Solution:**

Install Gemini CLI following the [official instructions](https://docs.anthropic.com/en/docs/gemini-code/getting-started), then verify:

```bash
gemini --version
```

## Permission Denied

**Symptom:** Gemini keeps asking for permission, disrupting autonomous flow.

**Cause:** Gemini CLI requires explicit permission for file writes and command execution.

**Solution:**

Melliza automatically runs Gemini with permission prompts disabled for autonomous operation. If you're still seeing permission issues, ensure you're running Melliza (not Gemini directly) and that your Gemini CLI installation is up to date.

## PRD Not Updating

**Symptom:** Stories stay incomplete even though Gemini seems to finish.

**Cause:** Gemini didn't output the completion signal, or file watching failed.

**Solution:**

1. Check `gemini.log` for errors:
   ```bash
   tail -100 .melliza/prds/your-prd/gemini.log
   ```

2. Manually mark story complete if appropriate:
   ```json
   {
     "id": "US-001",
     "passes": true,
     "inProgress": false
   }
   ```

3. Restart Melliza to pick up where it left off

## Loop Not Progressing

**Symptom:** Melliza runs but doesn't make progress on stories.

**Cause:** Various—Gemini may be stuck, context too large, or PRD unclear.

**Solution:**

1. Check `gemini.log` for what Gemini is doing:
   ```bash
   tail -f .melliza/prds/your-prd/gemini.log
   ```

2. Simplify the current story's acceptance criteria

3. Add context to `prd.md` about the codebase

4. Try restarting Melliza:
   ```bash
   # Press 'x' to stop (or Ctrl+C to quit)
   melliza  # Launch TUI
   # Press 's' to start the loop
   ```

## Max Iterations Reached

**Symptom:** Melliza stops with "max iterations reached" message.

**Cause:** Gemini hasn't completed after the iteration limit.

**Solution:**

1. Increase the limit:
   ```bash
   melliza --max-iterations 200
   ```

2. Or investigate why it's taking so many iterations:
   - Story too complex? Split it
   - Stuck in a loop? Check `gemini.log`
   - Unclear acceptance criteria? Clarify them

## "No PRD Found"

**Symptom:** Error about no PRD being found.

**Cause:** Missing `.melliza/prds/` directory or invalid PRD structure.

**Solution:**

1. Create a PRD:
   ```bash
   melliza new
   ```

2. Or specify the PRD explicitly:
   ```bash
   melliza my-feature
   ```

3. Verify structure:
   ```
   .melliza/
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
   cat .melliza/prds/your-prd/prd.json | jq .
   ```

2. Common issues:
   - Trailing commas (not allowed in JSON)
   - Missing quotes around keys
   - Unescaped characters in strings

## Worktree Setup Failures

**Symptom:** Worktree creation fails when starting a PRD.

**Cause:** The branch already exists, the worktree path is in use, or git state is corrupted.

**Solution:**

1. Melliza automatically handles common cases (reuses valid worktrees, cleans stale ones). If it still fails:

2. Manually clean up:
   ```bash
   # Remove the worktree
   git worktree remove .melliza/worktrees/<prd-name> --force

   # Delete the branch if needed
   git branch -D melliza/<prd-name>

   # Prune git's worktree tracking
   git worktree prune
   ```

3. Restart Melliza and try again

## PR Creation Failures

**Symptom:** Auto-PR creation fails after a PRD completes.

**Cause:** `gh` CLI not installed, not authenticated, or network issues.

**Solution:**

1. Verify `gh` is installed and authenticated:
   ```bash
   gh auth status
   ```

2. If not installed, get it from [cli.github.com](https://cli.github.com/)

3. If not authenticated:
   ```bash
   gh auth login
   ```

4. You can also create the PR manually:
   ```bash
   git push -u origin melliza/<prd-name>
   gh pr create --title "feat: <prd-name>" --body "..."
   ```

5. Auto-PR can be disabled in Settings (`,`) — push-only mode still works

## Orphaned Worktrees

**Symptom:** The picker shows entries marked `[orphaned]` or `[orphaned worktree]`.

**Cause:** A previous Melliza session crashed or was terminated without cleaning up its worktree.

**Solution:**

1. Orphaned worktrees are harmless but take disk space
2. Select the orphaned entry in the picker and press `c` to clean it up
3. Choose "Remove worktree + delete branch" or "Remove worktree only" as appropriate

Melliza automatically prunes git's internal worktree tracking on startup, but does not auto-delete worktree directories to avoid data loss.

## Merge Conflicts

**Symptom:** Merging a completed branch fails with conflict list.

**Cause:** The PRD's branch has changes that conflict with the target branch.

**Solution:**

1. Melliza shows the list of conflicting files in the merge result dialog
2. Resolve conflicts manually in a terminal:
   ```bash
   cd /path/to/project
   git merge melliza/<prd-name>
   # Resolve conflicts in the listed files
   git add .
   git commit
   ```
3. Or push the branch and resolve via a pull request on GitHub

## Still Stuck?

If none of these solutions help:

1. Check the [FAQ](/troubleshooting/faq)
2. Search [GitHub Issues](https://github.com/lvcoi/melliza/issues)
3. Open a new issue with:
   - Melliza version (`melliza --version`)
   - Your `prd.json` (sanitized)
   - Relevant `gemini.log` excerpts
