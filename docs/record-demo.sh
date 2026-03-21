#!/usr/bin/env bash
# Record an asciinema demo of the Chief TUI.
# Uses tmux for PTY + high-frequency capture-pane for smooth animations.
#
# Shows all 7 stories completing one by one, with time accelerating from 0 → 1h32m.
#
# Prerequisites: tmux, python3
# Usage: ./docs/record-demo.sh
# Output: docs/public/demo.cast

set -euo pipefail

unset TMUX
TMUX_BIN=/opt/homebrew/bin/tmux
DEMO_DIR="$HOME/projects/chief-demo"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUTPUT="$SCRIPT_DIR/public/demo.cast"
SESSION="chief-demo"
COLS=120
ROWS=36
FPS=15
INTERVAL=$(python3 -c "print(1/$FPS)")

FRAMES_DIR=$(mktemp -d)

# Reset ALL stories to incomplete
python3 -c "
import json
prd_file = '$DEMO_DIR/.chief/prds/weather-cli/prd.json'
with open(prd_file) as f:
    prd = json.load(f)
for story in prd['userStories']:
    story['passes'] = False
    story.pop('inProgress', None)
with open(prd_file, 'w') as f:
    json.dump(prd, f, indent=2)
    f.write('\n')
"

# Ensure demo repo is on a feature branch
(cd "$DEMO_DIR" && git checkout -B feat/weather-cli 2>/dev/null)

# Clean up any previous session
$TMUX_BIN kill-session -t "$SESSION" 2>/dev/null || true
$TMUX_BIN new-session -d -s "$SESSION" -x "$COLS" -y "$ROWS"

# Start chief with enough iterations for all 7 stories
$TMUX_BIN send-keys -t "$SESSION" \
  "cd $DEMO_DIR && chief weather-cli --agent-path $DEMO_DIR/fake-claude.sh -n 10" Enter

# Wait for the TUI to fully render
sleep 4

# Start background frame capture at ${FPS}fps
touch "$FRAMES_DIR/.capturing"
export TMUX_BIN SESSION FRAMES_DIR INTERVAL
(
  set +eu
  frame=0
  while [ -f "$FRAMES_DIR/.capturing" ]; do
    $TMUX_BIN capture-pane -t "$SESSION" -p -e > "$FRAMES_DIR/$(printf '%06d' $frame)" 2>/dev/null || true
    frame=$((frame + 1))
    sleep "$INTERVAL"
  done
) &
CAPTURE_PID=$!

# === Choreography ===

# Show the ready state briefly
sleep 3

# Start the loop — all 7 stories complete (~7s each = ~49s)
$TMUX_BIN send-keys -t "$SESSION" s

# Wait for all stories to complete + confetti
sleep 55

# Brief linger
sleep 2

# === End choreography ===

# Stop capture
rm -f "$FRAMES_DIR/.capturing"
wait $CAPTURE_PID 2>/dev/null || true

# Quit chief
$TMUX_BIN send-keys -t "$SESSION" q
sleep 0.5
$TMUX_BIN kill-session -t "$SESSION" 2>/dev/null || true

# Reset demo repo
(cd "$DEMO_DIR" && git checkout main 2>/dev/null && git branch -D feat/weather-cli 2>/dev/null) || true

# Convert frames to asciicast v2
export FRAMES_DIR OUTPUT COLS ROWS FPS
python3 << 'PYEOF'
import json, os, glob, re

frames_dir = os.environ["FRAMES_DIR"]
output = os.environ["OUTPUT"]
cols = int(os.environ["COLS"])
rows = int(os.environ["ROWS"])
fps = int(os.environ["FPS"])
interval = 1.0 / fps

frame_files = sorted(glob.glob(os.path.join(frames_dir, "[0-9]*")))
if not frame_files:
    print("ERROR: No frames captured!")
    exit(1)

header = {
    "version": 2,
    "width": cols,
    "height": rows,
    "env": {"SHELL": "/bin/zsh", "TERM": "xterm-256color"},
    "title": "Chief - Autonomous PRD Agent"
}

# --- Time-lapse: map real recording time (0 → ~40s) to display time (0 → 1h32m07s) ---
TARGET_END_SECONDS = 1 * 3600 + 32 * 60 + 7  # 1h32m07s

def format_time(total_secs):
    h = total_secs // 3600
    m = (total_secs % 3600) // 60
    s = total_secs % 60
    if h > 0:
        return f"{h}h{m:02d}m{s:02d}s"
    elif m > 0:
        return f"{m}m{s:02d}s"
    else:
        return f"{s}s"

def patch_header(raw, elapsed_secs, total_duration):
    """Replace time display with accelerated clock, preserve line width."""
    if total_duration <= 0:
        return raw

    # Map elapsed real time to display time (linear interpolation)
    progress = min(elapsed_secs / total_duration, 1.0)
    display_secs = int(progress * TARGET_END_SECONDS)
    t = format_time(display_secs)

    # Match: spaces + ANSI + "Iteration: X/Y" + ANSI mid + "Time: VALUE"
    pattern = r'( +)(\x1b\[[0-9;]*m)Iteration: (\d+/\d+)(\x1b\[39m  \x1b\[38;2;108;112;134m)Time: [0-9hms]+'

    def replacer(m):
        leading_spaces = m.group(1)
        ansi_before = m.group(2)
        orig_iter = m.group(3)
        ansi_mid = m.group(4)

        new_stats_visible = f'Iteration: {orig_iter}  Time: {t}'
        orig_visible = len(m.group(0)) - len(ansi_before) - len(ansi_mid)
        new_visible = len(leading_spaces) + len(new_stats_visible)

        diff = new_visible - orig_visible
        if diff > 0 and diff < len(leading_spaces):
            adjusted_spaces = leading_spaces[:-diff]
        elif diff < 0:
            adjusted_spaces = leading_spaces + ' ' * (-diff)
        else:
            adjusted_spaces = leading_spaces

        return f'{adjusted_spaces}{ansi_before}Iteration: {orig_iter}{ansi_mid}Time: {t}'

    raw = re.sub(pattern, replacer, raw)
    return raw

# --- Patch completion screen timing ---
STORY_TIMES = {
    "Geocoding and Location Lookup":  " 8m12s",
    "Current Conditions Display":     "14m03s",
    "7-Day Forecast Table":           "11m47s",
    "Hourly Sparkline Chart":         "12m31s",
    "Unit Preferences and Config":    " 9m55s",
    "Weather Alerts and Warnings":    "16m22s",
    "Shell Completions and Manpage":  "19m17s",
}

ANSI = r'(?:\x1b\[[0-9;]*m)*'  # match zero or more ANSI escape sequences

def patch_completion_screen(raw):
    """Patch the PRD Complete dialog timing to show realistic durations."""
    # Replace total time: "Completed in XXs" → "Completed in 1h32m"
    raw = re.sub(r'Completed in \d+s', 'Completed in 1h32m', raw)

    # Replace per-story times in ANSI-colored text
    # Pattern: story_name (ANSI+dots) (ANSI) TIME (ANSI) (spaces+bars)
    for story_name, fake_time in STORY_TIMES.items():
        # The time is wrapped in ANSI: \x1b[...]m5s\x1b[39m
        # Match the ANSI-wrapped time after the dots
        pattern = (re.escape(story_name) +
                   r'(' + ANSI + r'\s*' + ANSI + r'\.+' + ANSI + r'\s*)' +
                   ANSI + r'(\d+s)' + ANSI)

        def make_replacer(ft):
            def replacer(m):
                full = m.group(0)
                old_time = m.group(2)  # e.g. "5s"
                # Replace old time with fake time, same ANSI wrapping
                return full.replace(old_time, ft)
            return replacer

        raw = re.sub(pattern, make_replacer(fake_time), raw)

    return raw

# First pass: collect all frames and find total duration
raw_frames = []
started = False
for i, frame_path in enumerate(frame_files):
    with open(frame_path, "r") as f:
        raw = f.read()
    if not started:
        clean = re.sub(r'\x1b\[[0-9;]*m', '', raw)
        if ("Ready" in clean or "Running" in clean) and len(raw) > 1000:
            started = True
        else:
            continue
    raw_frames.append(raw)

total_real_duration = len(raw_frames) * interval

# Second pass: build events with patched time
events = []
prev_content = None
timestamp = 0.0

for raw in raw_frames:
    raw = patch_header(raw, timestamp, total_real_duration)
    raw = patch_completion_screen(raw)

    if raw == prev_content:
        timestamp += interval
        continue
    prev_content = raw

    lines = raw.split("\n")
    while len(lines) < rows:
        lines.append("")
    lines = lines[:rows]

    output_str = "\x1b[H"
    for i, line in enumerate(lines):
        padded = line + "\x1b[K"
        if i < rows - 1:
            output_str += padded + "\r\n"
        else:
            output_str += padded

    events.append([round(timestamp, 4), "o", output_str])
    timestamp += interval

# Trim: cut right before the completion screen (keep it as a surprise for users)
# Keep 2 seconds of the 100% state, then end
complete_start = None
for i, event in enumerate(events):
    clean = re.sub(r'\x1b\[[0-9;]*m', '', event[2])
    if 'PRD Complete' in clean:
        complete_start = event[0]
        break

if complete_start is not None:
    events = [e for e in events if e[0] < complete_start]
    print(f"Trimmed to {len(events)} frames (cut before completion screen at {complete_start:.1f}s)")

with open(output, "w") as f:
    f.write(json.dumps(header) + "\n")
    for event in events:
        f.write(json.dumps(event) + "\n")

duration = events[-1][0] if events else 0
print(f"Generated {len(events)} unique frames over {duration:.1f}s ({len(raw_frames)} total captured at {fps}fps)")
PYEOF

# Clean up frames
rm -rf "$FRAMES_DIR"

echo ""
echo "Recording saved to: $OUTPUT"
echo "Preview with: asciinema play '$OUTPUT'"
