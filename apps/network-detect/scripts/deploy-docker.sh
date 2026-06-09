#!/usr/bin/env sh
set -eu

log() {
  printf '[deploy] %s\n' "$*"
}

require_env() {
  name="$1"
  eval "value=\${$name:-}"
  if [ -z "$value" ]; then
    printf '[deploy] missing required environment variable: %s\n' "$name" >&2
    exit 1
  fi
}

require_env SSH_HOST
require_env SSH_USER
require_env DEPLOY_PATH

REGISTRY="${REGISTRY:-ghcr.io}"
APP_IMAGE="${APP_IMAGE:-}"
APP_TAG="${APP_TAG:-}"
if [ -z "$APP_IMAGE" ] && [ -n "${GITHUB_REPOSITORY:-}" ]; then
  APP_IMAGE="$REGISTRY/$GITHUB_REPOSITORY"
fi
if [ -z "$APP_TAG" ] && [ -n "${GITHUB_REF_NAME:-}" ]; then
  APP_TAG="$GITHUB_REF_NAME"
fi
if [ -z "$APP_IMAGE" ] || [ -z "$APP_TAG" ]; then
  printf '[deploy] APP_IMAGE and APP_TAG are required\n' >&2
  exit 1
fi

SSH_PORT="${SSH_PORT:-22}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
REMOTE_COMPOSE_FILE="$(basename "$COMPOSE_FILE")"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-.compose.env}"
COMPOSE_SERVICE_NAME="${COMPOSE_SERVICE_NAME:-network-detact}"
CONTAINER_NAME="${CONTAINER_NAME:-network-detact}"
HOST_BIND="${HOST_BIND:-0.0.0.0}"
HOST_PORT="${HOST_PORT:-8080}"
SSH_TARGET="$SSH_USER@$SSH_HOST"
SSH_KEY_PATH="${SSH_KEY_PATH:-$HOME/.ssh/id_ed25519}"

if [ ! -f "$COMPOSE_FILE" ]; then
  printf '[deploy] compose file not found: %s\n' "$COMPOSE_FILE" >&2
  exit 1
fi

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

SSH_OPTS="-i $SSH_KEY_PATH -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes"

cat > "$COMPOSE_ENV_FILE" <<EOF
APP_IMAGE=$APP_IMAGE
APP_TAG=$APP_TAG
CONTAINER_NAME=$CONTAINER_NAME
HOST_BIND=$HOST_BIND
HOST_PORT=$HOST_PORT
NEWAPI_API_SITE_TOKEN=${NEWAPI_API_SITE_TOKEN:-}
NEWAPI_TOKEN_SITE_TOKEN=${NEWAPI_TOKEN_SITE_TOKEN:-}
NEWAPI_API_SITE_URL=${NEWAPI_API_SITE_URL:-}
NEWAPI_TOKEN_SITE_URL=${NEWAPI_TOKEN_SITE_URL:-}
CONNECTIVITY_API_URLS=${CONNECTIVITY_API_URLS:-}
CONNECTIVITY_TOKEN_URLS=${CONNECTIVITY_TOKEN_URLS:-}
EOF

log "preparing remote directory on $SSH_TARGET"
ssh $SSH_OPTS -p "$SSH_PORT" "$SSH_TARGET" "
  set -eu
  mkdir -p '$DEPLOY_PATH'
"

log "uploading compose files to $SSH_TARGET:$DEPLOY_PATH"
scp $SSH_OPTS -P "$SSH_PORT" "$COMPOSE_FILE" "$SSH_TARGET:$DEPLOY_PATH/$REMOTE_COMPOSE_FILE"
scp $SSH_OPTS -P "$SSH_PORT" "$COMPOSE_ENV_FILE" "$SSH_TARGET:$DEPLOY_PATH/$COMPOSE_ENV_FILE"

REGISTRY_USER_VALUE="${REGISTRY_USER:-${GHCR_USERNAME:-}}"
REGISTRY_PASSWORD_VALUE="${REGISTRY_PASSWORD:-${GHCR_TOKEN:-}}"
if [ -n "$REGISTRY_USER_VALUE" ] && [ -n "$REGISTRY_PASSWORD_VALUE" ]; then
  log "logging remote docker into $REGISTRY"
  printf '%s' "$REGISTRY_PASSWORD_VALUE" | ssh $SSH_OPTS -p "$SSH_PORT" "$SSH_TARGET" \
    "docker login '$REGISTRY' -u '$REGISTRY_USER_VALUE' --password-stdin"
fi

log "deploying $APP_IMAGE:$APP_TAG"
ssh $SSH_OPTS -p "$SSH_PORT" "$SSH_TARGET" "
  set -eu
  cd '$DEPLOY_PATH'
  docker compose --env-file '$COMPOSE_ENV_FILE' -f '$REMOTE_COMPOSE_FILE' pull
  docker compose --env-file '$COMPOSE_ENV_FILE' -f '$REMOTE_COMPOSE_FILE' up -d
  docker compose --env-file '$COMPOSE_ENV_FILE' -f '$REMOTE_COMPOSE_FILE' ps

  CID=\$(docker compose --env-file '$COMPOSE_ENV_FILE' -f '$REMOTE_COMPOSE_FILE' ps -q '$COMPOSE_SERVICE_NAME')
  [ -n \"\$CID\" ]

  i=0
  READY=0
  while [ \$i -lt 30 ]; do
    STATUS=\$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \"\$CID\")
    echo \"container status: \$STATUS\"
    if [ \"\$STATUS\" = 'healthy' ] || [ \"\$STATUS\" = 'running' ]; then
      READY=1
      break
    fi
    i=\$((i + 1))
    sleep 5
  done

  if [ \"\$READY\" != '1' ]; then
    docker compose --env-file '$COMPOSE_ENV_FILE' -f '$REMOTE_COMPOSE_FILE' logs --tail=200
    docker inspect \"\$CID\"
    exit 1
  fi

  docker exec \"\$CID\" node -e \"fetch('http://127.0.0.1:8080/api/newapi/model-status?refresh=1').then(async (response) => { const data = await response.json().catch(() => ({})); const sites = Array.isArray(data.sites) ? data.sites : []; const errors = sites.flatMap((site) => [site.logError, site.channelsError, site.pricingError].filter(Boolean)); const authErrors = errors.filter((message) => /access token|无权|unauthori[sz]ed|forbidden|invalid token/i.test(String(message))); if (!response.ok || !data.configured || sites.length === 0 || authErrors.length) { console.error(authErrors[0] || data.configError || data.error || 'NewAPI model status is not ready'); process.exit(1); } console.log('configured NewAPI model sites: ' + sites.length); }).catch((error) => { console.error(error.message); process.exit(1); })\"

  exit 0
"
