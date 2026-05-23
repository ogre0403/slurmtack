#!/bin/sh

set -eu

WORKSPACE_FOLDER="${WORKSPACE_FOLDER:-${containerWorkspaceFolder:-}}"

if [ -z "$WORKSPACE_FOLDER" ]; then
    printf '%s\n' 'WORKSPACE_FOLDER is not set.' >&2
    exit 1
fi

mkdir -p /root/.claude_setting/claude
ln -sfn  /root/.claude_setting/claude       /root/.claude
ln -sf   /root/.claude_setting/.claude.json /root/.claude.json
ln -sfn  /root/.agents/skills               /root/.claude/skills


ln -sfn  "$WORKSPACE_FOLDER/.devcontainer/tmux.conf" /root/.tmux.conf
git config --global color.ui auto
git config --global core.pager 'less -FRX'
