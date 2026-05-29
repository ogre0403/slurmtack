#!/bin/sh

set -eu

SESSION_NAME="ai-workbench"
WORKSPACE_FOLDER="${WORKSPACE_FOLDER:-${containerWorkspaceFolder:-}}"

if [ -z "$WORKSPACE_FOLDER" ]; then
    printf '%s\n' 'WORKSPACE_FOLDER is not set.' >&2
    exit 1
fi

TMUX_CONF="$WORKSPACE_FOLDER/.devcontainer/tmux.conf"

run_agent() {
    window_name="$1"
    command_name="$2"

    if command -v "$command_name" >/dev/null 2>&1; then
        tmux -f "$TMUX_CONF" new-window -t "$SESSION_NAME": -n "$window_name" \
            "/bin/zsh -lc 'if $command_name; then exit_code=0; else exit_code=\$?; fi; if [ \$exit_code -eq 0 ]; then printf \"\\n%s exited. Returning to shell.\\n\" \"$command_name\"; else printf \"\\n%s exited with status %s. Returning to shell.\\n\" \"$command_name\" \"\$exit_code\"; fi; exec /bin/zsh -l'"
    else
        tmux -f "$TMUX_CONF" new-window -t "$SESSION_NAME": -n "$window_name" \
            "/bin/zsh -lc 'printf \"%s not found in PATH.\\n\" \"$command_name\"; exec /bin/zsh -l'"
    fi
}

if tmux -f "$TMUX_CONF" has-session -t "$SESSION_NAME" 2>/dev/null; then
    exec tmux -f "$TMUX_CONF" attach-session -t "$SESSION_NAME"
fi

tmux -f "$TMUX_CONF" new-session -d -s "$SESSION_NAME" -n work "exec /bin/zsh -l"
run_agent claude claude
run_agent opencode opencode
run_agent codex codex
tmux -f "$TMUX_CONF" new-window -t "$SESSION_NAME": -n ops "exec /bin/zsh -l"
tmux -f "$TMUX_CONF" select-window -t "$SESSION_NAME":1

exec tmux -f "$TMUX_CONF" attach-session -t "$SESSION_NAME"
