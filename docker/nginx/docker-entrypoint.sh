#!/bin/sh
set -e

CONFIG_DIR="/tmp/dashboard-runtime"
mkdir -p "$CONFIG_DIR"

SLURM_SIF_PATH="${SLURM_SIF_PATH:-}"

if [ -n "$SLURM_SIF_PATH" ]; then
  CONFIGURED="true"
else
  CONFIGURED="false"
fi

cat > "$CONFIG_DIR/dashboard-config.js" <<EOF
window.SLURMTACK_CONFIG = {
  slurmSifPathConfigured: ${CONFIGURED},
  slurmSifPath: "${SLURM_SIF_PATH}"
};
EOF

exec nginx -g 'daemon off;'
