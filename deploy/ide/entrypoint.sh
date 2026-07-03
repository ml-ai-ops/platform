#!/bin/sh
set -eu

mkdir -p /home/coder/.codex /home/coder/.claude /workspace
chown -R coder:coder /home/coder/.codex /home/coder/.claude /workspace

exec runuser -u coder -- code-server \
  --bind-addr 0.0.0.0:8080 \
  --auth password \
  /workspace
