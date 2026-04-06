#!/bin/bash

# =============================================================================
# AkôFlow — Installation & Management Script
# Usage: curl -fsSL https://akoflow.com/run | bash
# =============================================================================

set -euo pipefail

IMAGE_NAME="akoflow/akoflow"
CONTAINER_NAME="akoflow"
DEFAULT_PORT=8080
INSTALL_PATH="/usr/local/bin/akoflow"
FALLBACK_INSTALL_PATH="$HOME/.local/bin/akoflow"
VERSION="latest"

# ── Colors ────────────────────────────────────────────────────────────────────
BOLD="\033[1m"
DIM="\033[2m"
RESET="\033[0m"
GREEN="\033[32m"
YELLOW="\033[33m"
RED="\033[31m"
CYAN="\033[36m"
WHITE="\033[37m"

# ── Banner ────────────────────────────────────────────────────────────────────
banner() {
  printf "\n"
  printf "  ${BOLD}${WHITE} █████╗ ██╗  ██╗ ██████╗ ███████╗██╗      ██████╗ ██╗    ██╗${RESET}\n"
  printf "  ${BOLD}${WHITE}██╔══██╗██║ ██╔╝██╔═══██╗██╔════╝██║     ██╔═══██╗██║    ██║${RESET}\n"
  printf "  ${BOLD}${WHITE}███████║█████╔╝ ██║   ██║█████╗  ██║     ██║   ██║██║ █╗ ██║${RESET}\n"
  printf "  ${BOLD}${WHITE}██╔══██║██╔═██╗ ██║   ██║██╔══╝  ██║     ██║   ██║██║███╗██║${RESET}\n"
  printf "  ${BOLD}${WHITE}██║  ██║██║  ██╗╚██████╔╝██║     ███████╗╚██████╔╝╚███╔███╔╝${RESET}\n"
  printf "  ${BOLD}${WHITE}╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝     ╚══════╝ ╚═════╝  ╚══╝╚══╝${RESET}\n"
  printf "\n"
  printf "  ${DIM}Open Source Engine for Containerized Scientific Workflows${RESET}\n"
  printf "  ${DIM}Open Source · IC/UFF e-Science Research Group${RESET}\n"
  printf "  ${DIM}For help: akoflow --help${RESET}\n"
  printf "\n"
}

# ── Helpers ───────────────────────────────────────────────────────────────────
step()    { printf "\n  ${BOLD}${WHITE}%s${RESET}\n" "$1"; }
ok()      { printf "  ${GREEN}ok${RESET}    %s\n" "$1"; }
info()    { printf "  ${CYAN}info${RESET}  %s\n" "$1"; }
warn()    { printf "  ${YELLOW}warn${RESET}  %s\n" "$1"; }
fail()    { printf "\n  ${RED}error${RESET} %s\n\n" "$1" >&2; }
dim()     { printf "  ${DIM}%s${RESET}\n" "$1"; }
sep()     { printf "  %s\n" "────────────────────────────────────────"; }

# ── Spinner ───────────────────────────────────────────────────────────────────
_spin_pid=""

spin_start() {
  local msg="$1"
  printf "  ${DIM}%s${RESET} " "$msg"
  (while true; do
    for c in '.' '..' '...'; do
      printf "\r  ${DIM}%s %s   ${RESET}" "$msg" "$c"
      sleep 0.4
    done
  done) &
  _spin_pid=$!
}

spin_stop() {
  if [[ -n "$_spin_pid" ]]; then
    kill "$_spin_pid" 2>/dev/null || true
    wait "$_spin_pid" 2>/dev/null || true
    _spin_pid=""
    printf "\r\033[2K"
  fi
}

trap 'spin_stop' EXIT

# ── Docker check ──────────────────────────────────────────────────────────────
check_docker() {
  if ! command -v docker &>/dev/null; then
    fail "Docker is not installed."
    cat >&2 <<EOF
  Install Docker and try again:

    macOS / Windows  https://docs.docker.com/desktop/
    Linux (Ubuntu)   https://docs.docker.com/engine/install/ubuntu/

  After installing Docker, re-run:
    curl -fsSL https://akoflow.com/run | bash
EOF
    exit 1
  fi

  if ! docker info &>/dev/null; then
    fail "Docker is installed but not running."
    cat >&2 <<EOF
  Start Docker and try again:

    macOS / Windows  Open Docker Desktop
    Linux            sudo systemctl start docker

  If you see a permission error:
    sudo usermod -aG docker \$USER
    (then log out and back in)
EOF
    exit 1
  fi
}

# ── Port check ────────────────────────────────────────────────────────────────
port_in_use() {
  lsof -i :"$1" &>/dev/null 2>&1 || \
  ss -tlnp 2>/dev/null | grep -q ":$1 " || \
  netstat -tlnp 2>/dev/null | grep -q ":$1 "
}

# ── Get mapped port ───────────────────────────────────────────────────────────
get_port() {
  docker inspect -f \
    '{{ (index (index .NetworkSettings.Ports "80/tcp") 0).HostPort }}' \
    "$CONTAINER_NAME" 2>/dev/null || echo "unknown"
}

# ── Container state ───────────────────────────────────────────────────────────
container_running() {
  docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${CONTAINER_NAME}$"
}

container_exists() {
  docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^${CONTAINER_NAME}$"
}

# ── CLI binary install ────────────────────────────────────────────────────────
# Default: always installs to ~/.local/bin and informs the user.
# install-cli command: installs to /usr/local/bin (system-wide) if possible.
install_cli_binary() {
  local SYSTEM=false
  [[ "${1:-}" == "--system" ]] && SYSTEM=true

  # Resolve script path
  local SCRIPT_PATH=""
  if [[ -n "${BASH_SOURCE[0]:-}" && -f "${BASH_SOURCE[0]}" ]]; then
    SCRIPT_PATH="$(realpath "${BASH_SOURCE[0]}" 2>/dev/null || readlink -f "${BASH_SOURCE[0]}")"
  elif [[ -f "$0" ]]; then
    SCRIPT_PATH="$(realpath "$0" 2>/dev/null || readlink -f "$0")"
  fi

  # When piped via curl | bash, no file exists on disk — download directly
  if [[ -z "$SCRIPT_PATH" ]]; then
    local DEST
    if [[ "$SYSTEM" == true ]]; then
      DEST="$INSTALL_PATH"
    else
      DEST="$FALLBACK_INSTALL_PATH"
    fi
    mkdir -p "$(dirname "$DEST")"
    if command -v curl &>/dev/null; then
      curl -fsSL https://akoflow.com/run -o "$DEST" 2>/dev/null
    elif command -v wget &>/dev/null; then
      wget -qO "$DEST" https://akoflow.com/run 2>/dev/null
    else
      warn "Could not install CLI — curl and wget not found."
      return 0
    fi
    chmod +x "$DEST"
    warn "CLI installed at $DEST"
    if [[ "$SYSTEM" != true ]]; then
      dim "To use 'akoflow' from any terminal, add this to your ~/.bashrc or ~/.zshrc:"
      printf "\n"
      printf "    export PATH=\"\$HOME/.local/bin:\$PATH\"\n"
      printf "\n"
      dim "To install system-wide (all users): sudo akoflow install-cli"
    fi
    return 0
  fi

  if [[ "$SYSTEM" == true ]]; then
    # System-wide: requires sudo
    if sudo cp "$SCRIPT_PATH" "$INSTALL_PATH" && sudo chmod +x "$INSTALL_PATH" 2>/dev/null; then
      ok "CLI installed at $INSTALL_PATH"
      dim "Available system-wide as: akoflow"
    else
      warn "sudo required for system-wide install. Try: sudo akoflow install-cli"
      dim "Falling back to user install..."
      install_cli_binary
    fi
    return 0
  fi

  # Default: user-local install (~/.local/bin)
  mkdir -p "$(dirname "$FALLBACK_INSTALL_PATH")"
  if cp "$SCRIPT_PATH" "$FALLBACK_INSTALL_PATH" && chmod +x "$FALLBACK_INSTALL_PATH" 2>/dev/null; then
    warn "CLI installed at $FALLBACK_INSTALL_PATH"
    dim "To use 'akoflow' from any terminal, add this to your ~/.bashrc or ~/.zshrc:"
    printf "\n"
    printf "    export PATH=\"\$HOME/.local/bin:\$PATH\"\n"
    printf "\n"
    dim "To install system-wide (all users): sudo akoflow install-cli"
  else
    warn "Could not write to $FALLBACK_INSTALL_PATH — skipping CLI install."
  fi
}

# =============================================================================
# AUTO CLI INSTALL — runs on every execution if akoflow is not in PATH
# =============================================================================
if ! command -v akoflow &>/dev/null; then
  install_cli_binary
fi

# =============================================================================
# HELP
# =============================================================================
if [[ "${1:-}" == "help" || "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  printf "\n"
  printf "  %bAkôFlow CLI%b\n\n" "$BOLD" "$RESET"
  printf "  Open source middleware for containerized scientific workflows.\n\n"
  printf "  %bUsage%b\n    akoflow [command] [options]\n\n" "$BOLD" "$RESET"
  printf "  %bCommands%b\n" "$BOLD" "$RESET"
  printf "    (none)              Install AkôFlow and start the container\n"
  printf "    start               Start or resume the AkôFlow container\n"
  printf "    stop                Stop the running container\n"
  printf "    restart             Stop and start the container again\n"
  printf "    status              Show current state and access URL\n"
  printf "    logs                Stream container logs (Ctrl+C to exit)\n"
  printf "    update              Pull the latest image and restart\n"
  printf "    reset               Remove container and start fresh\n"
  printf "    remove              Remove the container completely\n"
  printf "    install-cli         Install the akoflow binary to your system\n"
  printf "    help                Show this help message\n\n"
  printf "  %bOptions%b\n    --port <port>       Use a custom port (default: %s)\n\n" "$BOLD" "$RESET" "$DEFAULT_PORT"
  printf "  %bExamples%b\n" "$BOLD" "$RESET"
  printf "    curl -fsSL https://akoflow.com/run | bash\n"
  printf "    akoflow start --port 9090\n"
  printf "    akoflow status\n"
  printf "    akoflow logs\n"
  printf "    akoflow restart\n"
  printf "    akoflow reset\n\n"
  printf "  %bAfter install%b\n" "$BOLD" "$RESET"
  printf "    AkôFlow runs at  http://localhost:%s\n" "$DEFAULT_PORT"
  printf "    Stop anytime     akoflow stop\n"
  printf "    Remove entirely  akoflow remove\n\n"
  printf "  During installation the script will ask if you want to install the akoflow CLI to a system path (e.g. /usr/local/bin).\n"
  printf "  If you don't have root, the installer will offer to install to a user-local path (~/.local/bin).\n\n"
  exit 0
fi

# =============================================================================
# INSTALL-CLI  (interactive — install binary to system)
# =============================================================================
if [[ "${1:-}" == "install-cli" || "${1:-}" == "--install" ]]; then
  banner
  install_cli_binary --system
  printf "\n"
  dim "Run 'akoflow --help' to see all available commands."
  printf "\n"
  exit 0
fi

# =============================================================================
# STATUS
# =============================================================================
if [[ "${1:-}" == "status" ]]; then
  check_docker
  banner
  sep
  if container_running; then
    PORT=$(get_port)
    CONTAINER_ID=$(docker inspect --format '{{.Id}}' "$CONTAINER_NAME" 2>/dev/null | cut -c1-12)
    UPTIME=$(docker inspect --format '{{.State.StartedAt}}' "$CONTAINER_NAME" 2>/dev/null)
    printf "  ${BOLD}${GREEN}Running${RESET}\n\n"
    printf "  Access      ${BOLD}http://localhost:${PORT}${RESET}\n"
    printf "  Container   ${DIM}%s  (%s)${RESET}\n" "$CONTAINER_NAME" "$CONTAINER_ID"
    printf "  Image       ${DIM}%s${RESET}\n" "$IMAGE_NAME"
    printf "  Started     ${DIM}%s${RESET}\n" "$UPTIME"
  elif container_exists; then
    printf "  ${YELLOW}Stopped${RESET}\n\n"
    printf "  Container   ${DIM}%s${RESET}\n" "$CONTAINER_NAME"
    printf "  Image       ${DIM}%s${RESET}\n" "$IMAGE_NAME"
    printf "\n"
    dim "Run: akoflow start"
  else
    printf "  ${DIM}Not installed${RESET}\n\n"
    dim "Run: akoflow start"
  fi
  sep
  exit 0
fi

# =============================================================================
# LOGS
# =============================================================================
if [[ "${1:-}" == "logs" ]]; then
  check_docker
  if ! container_exists; then
    fail "Container '$CONTAINER_NAME' not found. Run: akoflow start"
    exit 1
  fi
  info "Streaming logs from '$CONTAINER_NAME' (Ctrl+C to exit)"
  sep
  docker logs -f "$CONTAINER_NAME"
  exit 0
fi

# =============================================================================
# STOP
# =============================================================================
if [[ "${1:-}" == "stop" ]]; then
  check_docker
  banner
  if container_running; then
    spin_start "Stopping"
    docker stop "$CONTAINER_NAME" &>/dev/null
    spin_stop
    ok "AkôFlow stopped"
    dim "Run 'akoflow start' to start it again"
  else
    warn "AkôFlow is not running"
    dim "Run 'akoflow status' to check current state"
  fi
  exit 0
fi

# =============================================================================
# REMOVE
# =============================================================================
if [[ "${1:-}" == "remove" ]]; then
  check_docker
  if container_exists; then
    spin_start "Removing container"
    docker rm -f "$CONTAINER_NAME" &>/dev/null
    spin_stop
    ok "Container removed"
    dim "Image '$IMAGE_NAME' was kept. To also remove it: docker rmi $IMAGE_NAME"
  else
    warn "No container found"
  fi
  exit 0
fi

# =============================================================================
# RESTART
# =============================================================================
if [[ "${1:-}" == "restart" ]]; then
  check_docker
  banner
  if container_running; then
    spin_start "Stopping"
    docker stop "$CONTAINER_NAME" &>/dev/null
    spin_stop
  fi
  if container_exists; then
    spin_start "Starting"
    docker start "$CONTAINER_NAME" &>/dev/null
    spin_stop
    PORT=$(get_port)
    CONTAINER_ID=$(docker inspect --format '{{.Id}}' "$CONTAINER_NAME" 2>/dev/null | cut -c1-12)
    sep
    printf "  ${BOLD}${GREEN}AkôFlow restarted${RESET}\n\n"
    printf "  Access      ${BOLD}http://localhost:${PORT}${RESET}\n"
    printf "  Container   ${DIM}%s  (%s)${RESET}\n" "$CONTAINER_NAME" "$CONTAINER_ID"
    printf "  Image       ${DIM}%s${RESET}\n" "$IMAGE_NAME"
    sep
  else
    fail "Container not found. Run: akoflow start"
    exit 1
  fi
  exit 0
fi

# =============================================================================
# RESET  (remove container entirely and start fresh)
# =============================================================================
if [[ "${1:-}" == "reset" ]]; then
  check_docker

  step "Resetting AkôFlow"
  warn "This will remove the container and all runtime state."
  printf "  Continue? [y/N] "
  read -r CONFIRM
  if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
    dim "Aborted."
    exit 0
  fi

  if container_exists; then
    spin_start "Removing container"
    docker rm -f "$CONTAINER_NAME" &>/dev/null
    spin_stop
    ok "Container removed"
  fi

  # fall through to fresh start below
  info "Starting fresh..."
fi

# =============================================================================
# UPDATE  (pull latest image and restart)
# =============================================================================
if [[ "${1:-}" == "update" ]]; then
  check_docker
  step "Updating AkôFlow"

  spin_start "Pulling latest image"
  docker pull "$IMAGE_NAME" &>/dev/null
  spin_stop
  ok "Image updated"

  if container_exists; then
    spin_start "Recreating container"
    docker rm -f "$CONTAINER_NAME" &>/dev/null
    spin_stop
  fi

  info "Starting updated container..."
  # fall through to fresh start below
fi

# =============================================================================
# START  (default / after reset / after update)
# =============================================================================

# Parse --port flag
PORT=$DEFAULT_PORT
ARGS=("${@:-}")
i=0
while [[ $i -lt ${#ARGS[@]} ]]; do
  case "${ARGS[$i]}" in
    --port)
      i=$((i+1))
      PORT="${ARGS[$i]}"
      ;;
  esac
  i=$((i+1))
done

check_docker

# Already running
if container_running; then
  EXISTING_PORT=$(get_port)
  CONTAINER_ID=$(docker inspect --format '{{.Id}}' "$CONTAINER_NAME" 2>/dev/null | cut -c1-12)
  UPTIME=$(docker inspect --format '{{.State.StartedAt}}' "$CONTAINER_NAME" 2>/dev/null)
  banner
  sep
  printf "  ${BOLD}${GREEN}AkôFlow is running${RESET}\n\n"
  printf "  Access      ${BOLD}http://localhost:${EXISTING_PORT}${RESET}\n"
  printf "  Container   ${DIM}%s  (%s)${RESET}\n" "$CONTAINER_NAME" "$CONTAINER_ID"
  printf "  Image       ${DIM}%s${RESET}\n" "$IMAGE_NAME"
  printf "  Started     ${DIM}%s${RESET}\n" "$UPTIME"
  printf "\n"
  printf "  ${DIM}akoflow stop      stop the server${RESET}\n"
  printf "  ${DIM}akoflow restart   restart the container${RESET}\n"
  printf "  ${DIM}akoflow logs      stream live logs${RESET}\n"
  printf "  ${DIM}akoflow reset     remove and start fresh${RESET}\n"
  printf "  ${DIM}akoflow --help    show all commands${RESET}\n"
  sep
  printf "\n"
  exit 0
fi

# Stopped container — just resume it
if container_exists; then
  banner
  spin_start "Resuming AkôFlow"
  docker start "$CONTAINER_NAME" &>/dev/null
  spin_stop
  EXISTING_PORT=$(get_port)
  CONTAINER_ID=$(docker inspect --format '{{.Id}}' "$CONTAINER_NAME" 2>/dev/null | cut -c1-12)
  sep
  printf "  ${BOLD}${GREEN}AkôFlow is running${RESET}\n\n"
  printf "  Access      ${BOLD}http://localhost:${EXISTING_PORT}${RESET}\n"
  printf "  Container   ${DIM}%s  (%s)${RESET}\n" "$CONTAINER_NAME" "$CONTAINER_ID"
  printf "  Image       ${DIM}%s${RESET}\n" "$IMAGE_NAME"
  printf "\n"
  printf "  ${DIM}akoflow stop      stop the server${RESET}\n"
  printf "  ${DIM}akoflow restart   restart the container${RESET}\n"
  printf "  ${DIM}akoflow logs      stream live logs${RESET}\n"
  printf "  ${DIM}akoflow reset     remove and start fresh${RESET}\n"
  printf "  ${DIM}akoflow --help    show all commands${RESET}\n"
  sep
  printf "\n"
  exit 0
fi

# ── Fresh install ─────────────────────────────────────────────────────────────

banner
step "Installing AkôFlow"

# Port conflict check
if port_in_use "$PORT"; then
  fail "Port $PORT is already in use."
  cat >&2 <<EOF
  Try a different port:
    akoflow start --port 9090

  Or find what is using it:
    lsof -i :$PORT
EOF
  exit 1
fi

# Pull image
spin_start "Pulling image ($IMAGE_NAME)"
if ! docker pull "$IMAGE_NAME" &>/dev/null; then
  spin_stop
  fail "Could not pull image '$IMAGE_NAME'."
  cat >&2 <<EOF
  Possible causes:
    - No internet connection
    - Image does not exist on Docker Hub
    - Docker Hub rate limit reached (try: docker login)
EOF
  exit 1
fi
spin_stop
ok "Image ready"

# Run container
spin_start "Starting container"
if ! docker run -d \
    --name "$CONTAINER_NAME" \
    --restart unless-stopped \
    -p "${PORT}:80" \
    "$IMAGE_NAME" &>/dev/null; then
  spin_stop
  fail "Failed to start container."
  cat >&2 <<EOF
  Try running with verbose output to diagnose:
    docker run --name $CONTAINER_NAME -p ${PORT}:80 $IMAGE_NAME

  Common issues:
    - Port $PORT already in use:   akoflow start --port 9090
    - Permission denied:           sudo usermod -aG docker \$USER
EOF
  exit 1
fi
spin_stop
ok "Container started"

# ── Summary ───────────────────────────────────────────────────────────────────
printf "\n"
sep
printf "  ${BOLD}${GREEN}AkôFlow is running${RESET}\n\n"
printf "  Access      ${BOLD}http://localhost:${PORT}${RESET}\n"
printf "  Container   ${DIM}%s${RESET}\n" "$CONTAINER_NAME"
printf "  Image       ${DIM}%s${RESET}\n" "$IMAGE_NAME"
printf "\n"
printf "  ${DIM}akoflow stop     stop the server${RESET}\n"
printf "  ${DIM}akoflow logs     stream logs${RESET}\n"
printf "  ${DIM}akoflow restart  restart the container${RESET}\n"
printf "  ${DIM}akoflow reset    remove and start fresh${RESET}\n"
printf "  ${DIM}akoflow help     show all commands${RESET}\n"
sep
printf "\n"
