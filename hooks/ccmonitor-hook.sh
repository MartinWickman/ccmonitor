#!/bin/bash
#
# ccmonitor hook handler
# Called by Claude Code hooks on lifecycle events.
# Reads JSON from stdin, writes session status to a JSON file.
#

set -euo pipefail

SESSIONS_DIR="${CCMONITOR_SESSIONS_DIR:-$HOME/.ccmonitor/sessions}"
mkdir -p "$SESSIONS_DIR"

# Read hook input from stdin
INPUT=$(cat)

SESSION_ID=$(echo "$INPUT" | jq -r '.session_id')
CWD=$(echo "$INPUT" | jq -r '.cwd')
EVENT=$(echo "$INPUT" | jq -r '.hook_event_name')
TOOL=$(echo "$INPUT" | jq -r '.tool_name // empty')
TOOL_INPUT=$(echo "$INPUT" | jq -r '.tool_input // empty')
NOTIFICATION_TYPE=$(echo "$INPUT" | jq -r '.notification_type // empty')
PROMPT=$(echo "$INPUT" | jq -r '.prompt // empty')
TMUX_PANE_VAL="${TMUX_PANE:-}"
TMUX_TITLE=""
if [ -n "$TMUX_PANE_VAL" ]; then
    TMUX_TITLE=$(tmux display-message -p -t "$TMUX_PANE_VAL" '#{pane_title}' 2>/dev/null) || TMUX_TITLE=""
    # Strip the "✳ " prefix that Claude Code adds
    TMUX_TITLE="${TMUX_TITLE#✳ }"
fi
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

SESSION_FILE="$SESSIONS_DIR/$SESSION_ID.json"

# Remove session files with last_activity older than 1 hour
cleanup_stale() {
    local now
    now=$(date -u +%s)
    for f in "$SESSIONS_DIR"/*.json; do
        [ -f "$f" ] || continue
        local ts
        ts=$(jq -r '.last_activity // empty' "$f" 2>/dev/null) || continue
        [ -n "$ts" ] || continue
        local file_epoch
        file_epoch=$(date -u -d "$ts" +%s 2>/dev/null) || continue
        local age=$(( now - file_epoch ))
        if [ "$age" -gt 3600 ]; then
            rm -f "$f"
        fi
    done
}

# Build a short detail string from tool info
build_tool_detail() {
    local tool="$1"
    local input="$2"

    if [ -z "$tool" ]; then
        echo ""
        return
    fi

    case "$tool" in
        Bash)
            local cmd
            cmd=$(echo "$INPUT" | jq -r '.tool_input.command // empty' | head -c 80)
            echo "Bash: $cmd"
            ;;
        Edit|Write|Read)
            local file
            file=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
            echo "$tool ${file##*/}"
            ;;
        Glob)
            local pattern
            pattern=$(echo "$INPUT" | jq -r '.tool_input.pattern // empty')
            echo "Glob $pattern"
            ;;
        Grep)
            local pattern
            pattern=$(echo "$INPUT" | jq -r '.tool_input.pattern // empty')
            echo "Grep $pattern"
            ;;
        Task)
            local desc
            desc=$(echo "$INPUT" | jq -r '.tool_input.description // empty')
            echo "Task: $desc"
            ;;
        WebFetch|WebSearch)
            echo "$tool"
            ;;
        *)
            echo "$tool"
            ;;
    esac
}

# Map event to status and detail
case "$EVENT" in
    SessionStart)
        cleanup_stale
        STATUS="starting"
        DETAIL="Session started"
        ;;
    UserPromptSubmit)
        STATUS="working"
        DETAIL="Processing prompt..."
        ;;
    PreToolUse)
        STATUS="working"
        DETAIL=$(build_tool_detail "$TOOL" "$TOOL_INPUT")
        ;;
    PostToolUse)
        STATUS="working"
        DETAIL="Finished $TOOL, continuing..."
        ;;
    Notification)
        STATUS="waiting"
        case "$NOTIFICATION_TYPE" in
            idle_prompt)    DETAIL="Waiting for input" ;;
            permission_prompt) DETAIL="Awaiting response" ;;
            *)              DETAIL="$NOTIFICATION_TYPE" ;;
        esac
        ;;
    Stop)
        STATUS="idle"
        DETAIL="Finished responding"
        ;;
    SessionEnd)
        cleanup_stale
        rm -f "$SESSION_FILE"
        exit 0
        ;;
    *)
        exit 0
        ;;
esac

# Read existing last_prompt if we're not updating it
LAST_PROMPT=""
if [ "$EVENT" = "UserPromptSubmit" ]; then
    LAST_PROMPT="$PROMPT"
elif [ -f "$SESSION_FILE" ]; then
    LAST_PROMPT=$(jq -r '.last_prompt // empty' "$SESSION_FILE" 2>/dev/null) || LAST_PROMPT=""
fi

# Write session file
jq -n \
    --arg sid "$SESSION_ID" \
    --arg proj "$CWD" \
    --arg status "$STATUS" \
    --arg detail "$DETAIL" \
    --arg last_prompt "$LAST_PROMPT" \
    --arg notification_type "$NOTIFICATION_TYPE" \
    --arg ts "$TIMESTAMP" \
    --arg tmux_pane "$TMUX_PANE_VAL" \
    --arg summary "$TMUX_TITLE" \
    '{
        session_id: $sid,
        project: $proj,
        status: $status,
        detail: $detail,
        last_prompt: $last_prompt,
        notification_type: (if $notification_type == "" then null else $notification_type end),
        last_activity: $ts,
        tmux_pane: $tmux_pane,
        summary: $summary
    }' > "$SESSION_FILE"
