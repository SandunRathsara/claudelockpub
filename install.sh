#!/bin/sh
set -eu

REPO="SandunRathsara/claudelockpub"
DEFAULT_SERVER_URL="https://claudelock.vps.digisglobal.com"
CONFIG_DIR="${HOME}/.config"
CONFIG_PATH="${CONFIG_DIR}/claudelock.yaml"
INSTALL_PATH="/usr/local/bin/claudelock"
CLAUDE_ALIAS_START="# claudelock managed start"
CLAUDE_ALIAS_LINE='alias claude="claudelock run -- claude"'
CLAUDE_ALIAS_END="# claudelock managed end"
TTY_STATE=""
TTY_RESTORE_NEEDED=0
tmpdir=""

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'error: required command not found: %s\n' "$1" >&2
    exit 1
  }
}

detect_suffix() {
  os=$(uname -s 2>/dev/null || printf 'unknown')
  arch=$(uname -m 2>/dev/null || printf 'unknown')

  case "$os:$arch" in
    Darwin:arm64)
      printf 'darwin-arm64\n'
      return 0
      ;;
    Linux:x86_64|Linux:amd64)
      printf 'linux-amd64\n'
      return 0
      ;;
  esac

  if [ "$os" = "Linux" ] && [ -r /proc/version ]; then
    proc_version=$(tr '[:upper:]' '[:lower:]' </proc/version 2>/dev/null || printf '')
    case "$proc_version:$arch" in
      *microsoft*:x86_64|*microsoft*:amd64)
        printf 'linux-amd64\n'
        return 0
        ;;
    esac
  fi

  printf 'unsupported\n'
}

latest_release_json() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest"
}

can_use_tty() {
  [ -r /dev/tty ] && [ -w /dev/tty ] && (: >/dev/tty) 2>/dev/null
}

prompt_value() {
  prompt="$1"
  default_value="${2-}"

  if [ -n "$default_value" ]; then
    if can_use_tty; then
      printf '%s [%s]: ' "$prompt" "$default_value" >/dev/tty
    else
      printf '%s [%s]: ' "$prompt" "$default_value" >&2
    fi
  else
    if can_use_tty; then
      printf '%s: ' "$prompt" >/dev/tty
    else
      printf '%s: ' "$prompt" >&2
    fi
  fi

  if can_use_tty; then
    IFS= read -r value </dev/tty
  else
    IFS= read -r value
  fi

  if [ -n "$value" ]; then
    printf '%s' "$value"
  else
    printf '%s' "$default_value"
  fi
}

prompt_password() {
  if can_use_tty; then
    printf 'Password: ' >/dev/tty
  else
    printf 'Password: ' >&2
  fi

  if can_use_tty && command -v stty >/dev/null 2>&1; then
    TTY_STATE=$(stty -g </dev/tty)
    TTY_RESTORE_NEEDED=1
    stty -echo </dev/tty
    IFS= read -r password </dev/tty
    stty "$TTY_STATE" </dev/tty >/dev/null 2>&1
    TTY_RESTORE_NEEDED=0
    printf '\n' >/dev/tty
  else
    IFS= read -r password
  fi

  printf '%s' "$password"
}

extract_json_field() {
  key="$1"
  json="$2"
  printf '%s' "$json" | sed -n "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p"
}

try_bcrypt_hash() {
  password="$1"

  if command -v htpasswd >/dev/null 2>&1; then
    htpasswd -bnBC 10 '' "$password" 2>/dev/null | sed 's/^://;q'
    return 0
  fi

  if command -v mkpasswd >/dev/null 2>&1; then
    mkpasswd -m bcrypt "$password" 2>/dev/null | sed -n '1p'
    return 0
  fi

  return 1
}

yaml_quote() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

current_shell_name() {
  shell_path=${SHELL-}
  printf '%s\n' "${shell_path##*/}"
}

target_rc_path() {
  case "$(current_shell_name)" in
    zsh)
      printf '%s\n' "$HOME/.zshrc"
      ;;
    bash)
      printf '%s\n' "$HOME/.bashrc"
      ;;
    *)
      printf '%s\n' ""
      ;;
  esac
}

has_managed_claude_alias() {
  rc_path="$1"
  awk -v start="$CLAUDE_ALIAS_START" -v line="$CLAUDE_ALIAS_LINE" -v ending="$CLAUDE_ALIAS_END" '
    $0 == start {
      if (getline next_line <= 0) {
        next
      }
      if (next_line != line) {
        next
      }
      if (getline end_line <= 0) {
        next
      }
      if (end_line == ending) {
        found = 1
        exit 0
      }
    }
    END {
      exit(found ? 0 : 1)
    }
  ' "$rc_path"
}

append_managed_claude_alias() {
  rc_path="$1"
  {
    printf '\n%s\n' "$CLAUDE_ALIAS_START"
    printf '%s\n' "$CLAUDE_ALIAS_LINE"
    printf '%s\n' "$CLAUDE_ALIAS_END"
  } >>"$rc_path"
}

configure_claude_alias() {
  rc_path=$(target_rc_path)
  if [ -z "$rc_path" ]; then
    printf 'alias_status: skipped (unsupported shell: %s)\n' "$(current_shell_name)"
    return 0
  fi

  if [ -f "$rc_path" ]; then
    :
  else
    : >"$rc_path"
  fi

  if has_managed_claude_alias "$rc_path"; then
    printf 'alias_status: unchanged (%s already contains ClaudeLock managed alias)\n' "$rc_path"
  elif grep -Eq '^[[:space:]]*alias[[:space:]]+claude=' "$rc_path"; then
    printf 'alias_status: warning (%s already defines claude; left unchanged)\n' "$rc_path"
  else
    append_managed_claude_alias "$rc_path"
    printf 'alias_status: added (%s)\n' "$rc_path"
  fi

  printf 'shell_reload: open a new shell or source %s\n' "$rc_path"
}

install_binary() {
  source_path="$1"
  install_dir=$(dirname "$INSTALL_PATH")

  if [ -d "$install_dir" ]; then
    :
  elif [ -w "$(dirname "$install_dir")" ]; then
    install -d "$install_dir"
  elif command -v sudo >/dev/null 2>&1; then
    sudo install -d "$install_dir"
  else
    printf 'error: %s does not exist and sudo is unavailable to create it\n' "$install_dir" >&2
    exit 1
  fi

  if [ -w "$install_dir" ]; then
    install -m 0755 "$source_path" "$INSTALL_PATH"
    return 0
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 0755 "$source_path" "$INSTALL_PATH"
    return 0
  fi

  printf 'error: %s is not writable and sudo is unavailable\n' "$install_dir" >&2
  exit 1
}

for cmd in curl tar mktemp install; do
  require_cmd "$cmd"
done

suffix=$(detect_suffix)
if [ "$suffix" = "unsupported" ]; then
  printf 'error: unsupported platform. Supported platforms are macOS arm64 and Linux amd64 (including WSL).\n' >&2
  exit 1
fi

release_json=$(latest_release_json | tr -d '\n')
version=$(extract_json_field tag_name "$release_json")

if [ -z "$version" ]; then
  printf 'error: could not determine the latest release version from GitHub\n' >&2
  exit 1
fi

archive_name="claudelock-cli-${version}-${suffix}.tar.gz"
release_compact=$(printf '%s' "$release_json" | tr -d '[:space:]')
case "$release_compact" in
  *"\"name\":\"${archive_name}\""*)
    :
    ;;
  *)
    printf 'error: latest release %s does not include asset %s\n' "$version" "$archive_name" >&2
    exit 1
    ;;
esac

download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"
tmpdir=$(mktemp -d)
cleanup() {
  if [ "$TTY_RESTORE_NEEDED" -eq 1 ] && [ -n "$TTY_STATE" ] && can_use_tty && command -v stty >/dev/null 2>&1; then
    stty "$TTY_STATE" </dev/tty >/dev/null 2>&1 || true
    TTY_RESTORE_NEEDED=0
  fi

  if [ -n "$tmpdir" ]; then
    rm -rf "$tmpdir"
  fi
}
trap cleanup EXIT HUP INT TERM

archive_path="${tmpdir}/${archive_name}"
curl -fsSL "$download_url" -o "$archive_path"
tar -xzf "$archive_path" -C "$tmpdir"

binary_name="claudelock-cli-${version}-${suffix}"
binary_path=""
for candidate in "${tmpdir}/${binary_name}" "${tmpdir}/${binary_name}"*; do
  if [ -f "$candidate" ] && [ -x "$candidate" ]; then
    binary_path="$candidate"
    break
  fi
done

if [ -z "$binary_path" ]; then
  printf 'error: extracted archive does not contain an installable claudelock binary\n' >&2
  exit 1
fi

username=$(prompt_value "Username")
password=$(prompt_password)
server_url=$(prompt_value "Server URL" "$DEFAULT_SERVER_URL")

if [ -z "$username" ]; then
  printf 'error: username is required\n' >&2
  exit 1
fi

if [ -z "$password" ]; then
  printf 'error: password is required\n' >&2
  exit 1
fi

mkdir -p "$CONFIG_DIR"
backup_path=""
if [ -f "$CONFIG_PATH" ]; then
  timestamp=$(date +%Y%m%d%H%M%S)
  backup_path="${CONFIG_PATH}.${timestamp}.bak"
  mv "$CONFIG_PATH" "$backup_path"
fi

cat >"$CONFIG_PATH" <<EOF
server_url: "$(yaml_quote "$server_url")"
username: "$(yaml_quote "$username")"
password: "$(yaml_quote "$password")"
EOF

password_hash=""
if password_hash=$(try_bcrypt_hash "$password"); then
  :
else
  password_hash=""
fi

install_binary "$binary_path"
configure_claude_alias

printf '\nClaudeLock installation complete.\n'
printf 'installed_version: %s\n' "$version"
printf 'installed_path: %s\n' "$INSTALL_PATH"
printf 'config_path: %s\n' "$CONFIG_PATH"
if [ -n "$backup_path" ]; then
  printf 'backup_path: %s\n' "$backup_path"
else
  printf 'backup_path: none\n'
fi
printf 'username: %s\n' "$username"

if [ -n "$password_hash" ]; then
  printf 'password_hash: %s\n' "$password_hash"
  printf '\nShare these with Sandun:\n'
  printf 'username: %s\n' "$username"
  printf 'password_hash: %s\n' "$password_hash"
else
  printf '\nNo local bcrypt tool was found.\n'
  printf 'Generate a bcrypt hash for your chosen password using a trusted online tool, then send Sandun:\n'
  printf 'username: %s\n' "$username"
  printf 'password_hash: <generated bcrypt hash>\n'
fi
