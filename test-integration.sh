#!/usr/bin/env bash
set -euo pipefail

# Integration test: exercises hook handler → session files → monitor output

BINARY="./ccmonitor"
TMPDIR_PREFIX="ccmonitor-integration"
PASS=0
FAIL=0

cleanup() {
    rm -rf "$SESSION_DIR"
}

assert_output_contains() {
    local label="$1"
    local pattern="$2"
    local output
    output=$("$BINARY" --once 2>&1) || true
    if echo "$output" | grep -qF "$pattern"; then
        echo "  PASS: $label"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $label — expected '$pattern' in output:"
        echo "$output"
        FAIL=$((FAIL + 1))
    fi
}

assert_output_not_contains() {
    local label="$1"
    local pattern="$2"
    local output
    output=$("$BINARY" --once 2>&1) || true
    if echo "$output" | grep -qF "$pattern"; then
        echo "  FAIL: $label — did NOT expect '$pattern' in output:"
        echo "$output"
        FAIL=$((FAIL + 1))
    else
        echo "  PASS: $label"
        PASS=$((PASS + 1))
    fi
}

pipe_hook() {
    echo "$1" | "$BINARY" hook
}

# --- Setup ---
SESSION_DIR=$(mktemp -d "/tmp/${TMPDIR_PREFIX}.XXXXXX")
export CCMONITOR_SESSIONS_DIR="$SESSION_DIR"
# Prevent hook from capturing real tmux pane info
unset TMUX_PANE 2>/dev/null || true
trap cleanup EXIT

echo "Integration tests (session dir: $SESSION_DIR)"
echo ""

SID1="aaaaaaaa-1111-2222-3333-444444444444"
SID2="bbbbbbbb-2222-3333-4444-555555555555"

# --- Scenario 1: PreToolUse creates a working session ---
echo "Scenario 1: PreToolUse creates working session"
pipe_hook "{
    \"session_id\": \"$SID1\",
    \"cwd\": \"/tmp/testproject\",
    \"hook_event_name\": \"PreToolUse\",
    \"tool_name\": \"Bash\",
    \"tool_input\": {\"command\": \"ls -la\"}
}"
assert_output_contains "status is Working" "Working"
assert_output_contains "detail shows Bash: ls -la" "Bash: ls -la"

# --- Scenario 2: UserPromptSubmit captures prompt ---
echo "Scenario 2: UserPromptSubmit captures prompt"
pipe_hook "{
    \"session_id\": \"$SID1\",
    \"cwd\": \"/tmp/testproject\",
    \"hook_event_name\": \"UserPromptSubmit\",
    \"prompt\": \"Fix the authentication bug\"
}"
assert_output_contains "prompt text appears" "Fix the authentication bug"

# --- Scenario 3: Stop sets idle ---
echo "Scenario 3: Stop sets idle"
pipe_hook "{
    \"session_id\": \"$SID1\",
    \"cwd\": \"/tmp/testproject\",
    \"hook_event_name\": \"Stop\"
}"
assert_output_contains "status is Idle" "Idle"

# --- Scenario 4: idle_prompt notification is ignored ---
echo "Scenario 4: idle_prompt notification is ignored (session stays idle)"
pipe_hook "{
    \"session_id\": \"$SID1\",
    \"cwd\": \"/tmp/testproject\",
    \"hook_event_name\": \"Notification\",
    \"notification_type\": \"idle_prompt\"
}"
assert_output_contains "still shows Idle" "Idle"
assert_output_not_contains "does not show Waiting" "Waiting"

# --- Scenario 5: permission_prompt with title sets waiting ---
echo "Scenario 5: permission_prompt notification sets waiting"
pipe_hook "{
    \"session_id\": \"$SID2\",
    \"cwd\": \"/tmp/testproject\",
    \"hook_event_name\": \"PreToolUse\",
    \"tool_name\": \"Bash\",
    \"tool_input\": {\"command\": \"rm -rf /tmp/stuff\"}
}"
pipe_hook "{
    \"session_id\": \"$SID2\",
    \"cwd\": \"/tmp/testproject\",
    \"hook_event_name\": \"Notification\",
    \"notification_type\": \"permission_prompt\",
    \"title\": \"Allow file deletion?\"
}"
assert_output_contains "status is Waiting" "Waiting"
assert_output_contains "title text appears" "Allow file deletion?"

# --- Scenario 6: SessionEnd removes session file ---
echo "Scenario 6: SessionEnd removes session"
pipe_hook "{
    \"session_id\": \"$SID2\",
    \"cwd\": \"/tmp/testproject\",
    \"hook_event_name\": \"SessionEnd\"
}"
assert_output_not_contains "session removed" "bbbbbbbb"

# --- Results ---
echo ""
TOTAL=$((PASS + FAIL))
echo "Results: $PASS/$TOTAL passed"
if [ "$FAIL" -gt 0 ]; then
    echo "FAILED"
    exit 1
fi
echo "ALL PASSED"
