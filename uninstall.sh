#!/bin/sh
set -eu

INSTALL_PATH="/usr/local/bin/claudelock"
CONFIG_PATH="${HOME}/.config/claudelock.yaml"
DRY_RUN=0

usage() {
  printf 'usage: uninstall.sh [--dry-run]\n' >&2
  exit 1
}

log() {
  printf '%s\n' "$1"
}

path_exists() {
  [ -e "$1" ] || [ -L "$1" ]
}

remove_file() {
  label="$1"
  path="$2"

  if ! path_exists "$path"; then
    log "${label}: already absent"
    return 0
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    log "${label}: would remove ${path}"
    return 0
  fi

  rm -f -- "$path"
  log "${label}: removed ${path}"
}

remove_binary() {
  if ! path_exists "$INSTALL_PATH"; then
    log 'binary: already absent'
    return 0
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    log "binary: would remove ${INSTALL_PATH}"
    return 0
  fi

  install_dir=$(dirname "$INSTALL_PATH")
  if [ -w "$install_dir" ]; then
    rm -f -- "$INSTALL_PATH"
    log "binary: removed ${INSTALL_PATH}"
    return 0
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo rm -f -- "$INSTALL_PATH"
    log "binary: removed ${INSTALL_PATH}"
    return 0
  fi

  printf 'error: %s exists but %s is not writable and sudo is unavailable\n' "$INSTALL_PATH" "$install_dir" >&2
  exit 1
}

remove_backups() {
  found=0

  for backup_path in "${CONFIG_PATH}".??????????????.bak; do
    case "$backup_path" in
      "${CONFIG_PATH}".[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9].bak)
        found=1
        ;;
    esac
  done

  if [ "$found" -eq 0 ]; then
    log 'backups: none found'
    return 0
  fi

  count=0
  for backup_path in "${CONFIG_PATH}".??????????????.bak; do
    case "$backup_path" in
      "${CONFIG_PATH}".[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9].bak)
        count=$((count + 1))
        remove_file 'backup' "$backup_path"
        ;;
    esac
  done

  if [ "$DRY_RUN" -eq 1 ]; then
    log "backups: would remove ${count} file(s)"
  else
    log "backups: removed ${count} file(s)"
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    *)
      printf 'error: unsupported argument: %s\n' "$1" >&2
      usage
      ;;
  esac
  shift
done

if [ "$DRY_RUN" -eq 1 ]; then
  log 'mode: dry-run'
else
  log 'mode: live'
fi

remove_binary
remove_file 'config' "$CONFIG_PATH"
remove_backups
