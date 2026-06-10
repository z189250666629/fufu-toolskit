#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[deploy:%s] %s\n' "${APP_NAME:-app}" "$*"
}

require_env() {
  local name="$1"
  local value="${!name:-}"
  if [ -z "$value" ]; then
    printf '[deploy] missing required environment variable: %s\n' "$name" >&2
    exit 1
  fi
}

require_port() {
  local name="$1"
  local value="${!name:-}"
  if [ -z "$value" ]; then
    printf '[deploy] %s is required and must be a number between 1 and 65535\n' "$name" >&2
    exit 1
  fi
  if [[ ! "$value" =~ ^[0-9]+$ ]]; then
    printf '[deploy] %s must be a number between 1 and 65535\n' "$name" >&2
    exit 1
  fi
  local number=$((10#$value))
  if [ "$number" -lt 1 ] || [ "$number" -gt 65535 ]; then
    printf '[deploy] %s must be a number between 1 and 65535\n' "$name" >&2
    exit 1
  fi
}

write_env_var() {
  local name="$1"
  local value="${!name:-}"
  # Docker Compose env files are line-oriented. Strip CR/LF to avoid corrupting
  # the file while keeping the secret out of logs.
  value="$(printf '%s' "$value" | tr -d '\r' | tr '\n' ' ')"
  printf '%s=%s\n' "$name" "$value" >> "$COMPOSE_ENV_FILE"
}

cleanup_compose_env_file() {
  rm -f -- "$COMPOSE_ENV_FILE"
}

require_env APP_NAME
require_env APP_IMAGE
require_env APP_TAG
require_env SSH_HOST
require_env SSH_USER
require_env DEPLOY_PATH
require_env COMPOSE_FILE
require_env COMPOSE_SERVICE_NAME

if [ ! -f "$COMPOSE_FILE" ]; then
  printf '[deploy] compose file not found: %s\n' "$COMPOSE_FILE" >&2
  exit 1
fi

REGISTRY="${REGISTRY:-ghcr.io}"
SSH_PORT="${SSH_PORT:-22}"
SSH_TARGET="$SSH_USER@$SSH_HOST"
SSH_KEY_PATH="${SSH_KEY_PATH:-$HOME/.ssh/id_ed25519}"
SSH_OPTS=(-i "$SSH_KEY_PATH" -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes)
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-.deploy.env}"
REMOTE_COMPOSE_FILE="$(basename "$COMPOSE_FILE")"
CONTAINER_NAME="${CONTAINER_NAME:-$APP_NAME}"
HOST_BIND="${HOST_BIND:-0.0.0.0}"
HOST_PORT="${HOST_PORT:-}"
require_port HOST_PORT

mkdir -p "$HOME/.ssh"
chmod 700 "$HOME/.ssh"

if [ -n "${SSH_PRIVATE_KEY_B64:-}" ]; then
  printf '%s' "$SSH_PRIVATE_KEY_B64" | base64 -d | tr -d '\r' > "$SSH_KEY_PATH"
elif [ -n "${SSH_PRIVATE_KEY:-}" ]; then
  if [ -f "$SSH_PRIVATE_KEY" ]; then
    tr -d '\r' < "$SSH_PRIVATE_KEY" > "$SSH_KEY_PATH"
  else
    printf '%s' "$SSH_PRIVATE_KEY" | tr -d '\r' > "$SSH_KEY_PATH"
  fi
fi

if [ ! -f "$SSH_KEY_PATH" ]; then
  printf '[deploy] SSH_PRIVATE_KEY, SSH_PRIVATE_KEY_B64, or SSH_KEY_PATH is required\n' >&2
  exit 1
fi
chmod 600 "$SSH_KEY_PATH"
if ! ssh-keygen -y -f "$SSH_KEY_PATH" >/dev/null 2>&1; then
  printf '[deploy] SSH private key is not valid\n' >&2
  exit 1
fi

if [ -n "${SSH_KNOWN_HOSTS:-}" ]; then
  printf '%s\n' "$SSH_KNOWN_HOSTS" > "$HOME/.ssh/known_hosts"
else
  ssh-keyscan -p "$SSH_PORT" -H "$SSH_HOST" > "$HOME/.ssh/known_hosts"
fi
chmod 644 "$HOME/.ssh/known_hosts"

: > "$COMPOSE_ENV_FILE"
trap cleanup_compose_env_file EXIT
write_env_var APP_IMAGE
write_env_var APP_TAG
write_env_var CONTAINER_NAME
write_env_var HOST_BIND
write_env_var HOST_PORT

CONFIG_JSON_FILE=""
case "$APP_NAME" in
  fufu-act)
    for name in \
      FUFU_API_BASE_URL FUFU_API_TOKEN FUFU_API_USER_ID FUFU_QUOTA_UNIT \
      NEWAPI_API_SITE_URL NEWAPI_API_SITE_TOKEN NEWAPI_TOKEN_SITE_TOKEN \
      MCY_BASE_URL MCY_COOKIE MCY_USERNAME MCY_PASSWORD MCY_LOGIN_ENDPOINT MCY_UPLOAD_ENDPOINT ADMIN_TOKEN
    do
      write_env_var "$name"
    done
    ;;
  y2k-nav)
    ;;
  network-detect)
    for name in \
      NEWAPI_MANAGED_API_SITES NEWAPI_MANAGED_API_CONFIG \
      NEWAPI_API_SITE_TOKEN NEWAPI_TOKEN_SITE_TOKEN NEWAPI_API_SITE_URL NEWAPI_TOKEN_SITE_URL \
      CONNECTIVITY_API_URLS CONNECTIVITY_TOKEN_URLS
    do
      write_env_var "$name"
    done
    ;;
  *)
    printf '[deploy] unsupported APP_NAME: %s\n' "$APP_NAME" >&2
    exit 1
    ;;
esac

log "preparing remote directory $SSH_TARGET:$DEPLOY_PATH"
ssh "${SSH_OPTS[@]}" -p "$SSH_PORT" "$SSH_TARGET" "set -eu; mkdir -p '$DEPLOY_PATH' '$DEPLOY_PATH/data'"

log "uploading compose and env files"
scp "${SSH_OPTS[@]}" -P "$SSH_PORT" "$COMPOSE_FILE" "$SSH_TARGET:$DEPLOY_PATH/$REMOTE_COMPOSE_FILE"
scp "${SSH_OPTS[@]}" -P "$SSH_PORT" "$COMPOSE_ENV_FILE" "$SSH_TARGET:$DEPLOY_PATH/.env"
if [ -n "$CONFIG_JSON_FILE" ]; then
  scp "${SSH_OPTS[@]}" -P "$SSH_PORT" "$CONFIG_JSON_FILE" "$SSH_TARGET:$DEPLOY_PATH/config.json"
fi

REGISTRY_USER_VALUE="${REGISTRY_USER:-${GHCR_USERNAME:-}}"
REGISTRY_PASSWORD_VALUE="${REGISTRY_PASSWORD:-${GHCR_TOKEN:-}}"
if [ -n "$REGISTRY_USER_VALUE" ] && [ -n "$REGISTRY_PASSWORD_VALUE" ]; then
  log "logging remote docker into $REGISTRY"
  printf '%s' "$REGISTRY_PASSWORD_VALUE" | ssh "${SSH_OPTS[@]}" -p "$SSH_PORT" "$SSH_TARGET" \
    "docker login '$REGISTRY' -u '$REGISTRY_USER_VALUE' --password-stdin"
fi

log "deploying $APP_IMAGE:$APP_TAG"
ssh "${SSH_OPTS[@]}" -p "$SSH_PORT" "$SSH_TARGET" "
  set -eu
  cd '$DEPLOY_PATH'
  docker compose --env-file .env -f '$REMOTE_COMPOSE_FILE' config --quiet
  docker compose --env-file .env -f '$REMOTE_COMPOSE_FILE' pull
  docker compose --env-file .env -f '$REMOTE_COMPOSE_FILE' up -d
  docker compose --env-file .env -f '$REMOTE_COMPOSE_FILE' ps

  CID=\$(docker compose --env-file .env -f '$REMOTE_COMPOSE_FILE' ps -q '$COMPOSE_SERVICE_NAME')
  [ -n \"\$CID\" ]

  i=0
  READY=0
  while [ \$i -lt 30 ]; do
    STATUS=\$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \"\$CID\" 2>/dev/null || true)
    if [ -z \"\$STATUS\" ]; then
      echo \"container status: inspect_failed\"
      break
    fi
    echo \"container status: \$STATUS\"
    if [ \"\$STATUS\" = 'healthy' ] || [ \"\$STATUS\" = 'running' ]; then
      READY=1
      break
    fi
    i=\$((i + 1))
    sleep 5
  done

  if [ \"\$READY\" != '1' ]; then
    docker compose --env-file .env -f '$REMOTE_COMPOSE_FILE' logs --tail=200
    docker inspect \"\$CID\"
    exit 1
  fi
"

log "done"
