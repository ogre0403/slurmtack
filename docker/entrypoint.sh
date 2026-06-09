#!/bin/sh
set -e

TARGET_UID="${PUID:-1000}"
TARGET_GID="${PGID:-1000}"

CURRENT_UID=$(id -u slurmtack)
CURRENT_GID=$(id -g slurmtack)

if [ "$TARGET_GID" != "$CURRENT_GID" ]; then
    groupmod -g "$TARGET_GID" slurmtack
fi

if [ "$TARGET_UID" != "$CURRENT_UID" ]; then
    usermod -u "$TARGET_UID" slurmtack
fi

# chown -R slurmtack:slurmtack /data /home/slurmtack
chown -R slurmtack:slurmtack /home/slurmtack

# Fix ownership of the database file so both Docker and Podman rootless can
# write it.  Deliberately avoids recursive chown on /data to preserve host
# ownership of config files (.env, nginx/, etc.) that share the same mount.
# NOTE: When use Podman, uid in host will be different with container.
DB_FILE="${DB_PATH:-/data/slurmtack.db}"
DB_DIR=$(dirname "$DB_FILE")
chown slurmtack:slurmtack "$DB_DIR"
for f in "$DB_FILE" "${DB_FILE}-wal" "${DB_FILE}-shm" "${DB_FILE}-journal"; do
    [ -e "$f" ] && chown slurmtack:slurmtack "$f"
done

# If SSH_PRIVATE_KEY_PATH is set and the file exists, copy it into the
# slurmtack home dir (while still root) so that it is readable regardless of
# how the container runtime (Docker vs Podman rootless) maps the mount owner.
HOME_KEY=/home/slurmtack/.ssh/id_rsa
if [ -n "${SSH_PRIVATE_KEY_PATH}" ] && [ -f "${SSH_PRIVATE_KEY_PATH}" ]; then
    cp "${SSH_PRIVATE_KEY_PATH}" "$HOME_KEY"
    chown slurmtack:slurmtack "$HOME_KEY"
    chmod 600 "$HOME_KEY"
    export SSH_PRIVATE_KEY_PATH="$HOME_KEY"
fi

exec su-exec slurmtack slurmtack "$@"
