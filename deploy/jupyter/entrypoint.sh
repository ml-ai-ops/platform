#!/bin/sh
set -e

# Seed the quickstart once; everything under /workspace persists in the
# jupyter-data volume, so user edits are never overwritten.
if [ ! -f /workspace/quickstart.ipynb ]; then
  cp /opt/seed/quickstart.ipynb /workspace/quickstart.ipynb
fi

exec jupyter lab \
  --ip=0.0.0.0 \
  --port=8888 \
  --no-browser \
  --ServerApp.root_dir=/workspace \
  --IdentityProvider.token="${JUPYTER_TOKEN:-mlaiops-local}"
