#!/usr/bin/env bash
#
# Build, push, and deploy the payment-service-provider image.
#
# Config is read from the environment, or from scripts/deploy/.env.deploy if
# present. See .env.deploy.example for the full list of variables.
#
# Usage:
#   ./scripts/deploy/deploy.sh --env uat                # build + push + deploy
#   ./scripts/deploy/deploy.sh --env prod --tag 1.2.0   # pin an explicit tag
#   ./scripts/deploy/deploy.sh --env uat --skip-deploy  # build and push only
#   ./scripts/deploy/deploy.sh --env uat --apply-migrations
#   ./scripts/deploy/deploy.sh --help
#
# Unlike a compiled frontend bundle, the Go binary reads all of its config at
# run time, so nothing has to be baked into the image — --env only selects the
# default image tag and the APP_ENV the remote stack is brought up with.
#
# db/migrations/ and the vendor config files (.env.<vendor>.<channel> plus the
# PEMs they point at) are kept out of the build context by .dockerignore and
# bind-mounted by docker-compose.yml, so they are uploaded to the server as
# plain files instead of shipping inside the image.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ENV_FILE="${ENV_FILE:-${SCRIPT_DIR}/.env.deploy}"

# --- options -----------------------------------------------------------------

DO_BUILD=true
DO_PUSH=true
DO_DEPLOY=true
DO_MIGRATIONS=true
DO_APPLY_MIGRATIONS=false
DO_CONFIG=false
PRUNE_MIGRATIONS=false
NO_CACHE="--no-cache"
CLI_APP_ENV=""
CLI_TAG=""

usage() {
  sed -n '2,22p' "${BASH_SOURCE[0]}" | sed 's/^#\s\?//'
  cat <<'EOF'

Options:
  -e, --env <name>   Target environment: uat or prod. Becomes the default image
                     tag and the APP_ENV the remote compose stack runs with.
  -t, --tag <tag>    Explicit image tag, overriding the --env default.
  --platform <p>     Build platform (default linux/amd64, matching the
                     GOARCH=amd64 pinned in the Dockerfile).
  --skip-build       Don't build the image (assumes it already exists locally)
  --skip-push        Don't push the image to the registry
  --skip-deploy      Don't run the remote pull/up step
  --skip-migrations  Don't upload db/migrations to the server
  --apply-migrations Run the `migrate` compose service on the server after the
                     upload, so the schema is up to date before the app starts.
                     Off by default: it writes to the remote database.
  --prune-migrations Delete remote migration files that no longer exist locally
                     (destructive on the server; off by default)
  --config           Also upload the files listed in DEPLOY_CONFIG_FILES (vendor
                     .env.<vendor>.<channel> files and their PEMs). Off by
                     default — these rarely change and carry live credentials.
  --cache            Allow Docker layer cache (default is --no-cache)
  -h, --help         Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -e|--env)
      [[ $# -ge 2 ]] || { echo "error: $1 requires a value (uat or prod)" >&2; exit 2; }
      CLI_APP_ENV="$2"; shift ;;
    --env=*)         CLI_APP_ENV="${1#*=}" ;;
    -t|--tag)
      [[ $# -ge 2 ]] || { echo "error: $1 requires a value" >&2; exit 2; }
      CLI_TAG="$2"; shift ;;
    --tag=*)         CLI_TAG="${1#*=}" ;;
    --platform)
      [[ $# -ge 2 ]] || { echo "error: $1 requires a value" >&2; exit 2; }
      BUILD_PLATFORM="$2"; shift ;;
    --platform=*)    BUILD_PLATFORM="${1#*=}" ;;
    --skip-build)    DO_BUILD=false ;;
    --skip-push)     DO_PUSH=false ;;
    --skip-deploy)   DO_DEPLOY=false ;;
    --skip-migrations)  DO_MIGRATIONS=false ;;
    --apply-migrations) DO_APPLY_MIGRATIONS=true ;;
    --prune-migrations) PRUNE_MIGRATIONS=true ;;
    --config)        DO_CONFIG=true ;;
    --cache)         NO_CACHE="" ;;
    -h|--help)       usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

# --- config ------------------------------------------------------------------

if [[ -f "${ENV_FILE}" ]]; then
  log_env="${ENV_FILE}"
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

# The flags win over anything the config file set.
[[ -n "${CLI_APP_ENV}" ]] && APP_ENV="${CLI_APP_ENV}"
[[ -n "${CLI_TAG}" ]] && TAG="${CLI_TAG}"
APP_ENV="${APP_ENV:-}"

# Defaults for anything not pinned by the environment.
IMAGE_NAME="${IMAGE_NAME:-payment-service-provider}"
# The tag defaults to the environment name, not "latest" — uat and prod images
# live under the same registry path, and a shared :latest would have each
# deploy overwrite the other's image.
TAG="${TAG:-${APP_ENV:-latest}}"
# The service to pull/restart in the server's docker-compose.yml. This is not
# the same string as IMAGE_NAME: compose calls the API container "app".
COMPOSE_SERVICE="${COMPOSE_SERVICE:-app}"
MIGRATE_SERVICE="${MIGRATE_SERVICE:-migrate}"
SSH_PORT="${SSH_PORT:-22}"
REMOTE_DIR="${REMOTE_DIR:-/opt/psp}"
# The Dockerfile cross-compiles with a hardcoded GOARCH=amd64, but the runtime
# stage (alpine) would otherwise inherit the builder's architecture — on an
# arm64 workstation that produces an image the x86 server cannot run.
BUILD_PLATFORM="${BUILD_PLATFORM:-linux/amd64}"

# The migrations are not read from the image (see docker-compose.yml) — the
# migrate/migrate container bind-mounts ./db/migrations, so they have to exist
# on the server as plain files before the stack comes up.
LOCAL_MIGRATIONS_DIR="${LOCAL_MIGRATIONS_DIR:-${ROOT_DIR}/db/migrations}"
REMOTE_MIGRATIONS_DIR="${REMOTE_MIGRATIONS_DIR:-${REMOTE_DIR}/db/migrations}"

# Space-separated, repo-root-relative paths uploaded by --config. These are
# excluded by both .gitignore and .dockerignore (.env.*, *.pem), so the server
# copy is the only one that exists. Empty by default.
DEPLOY_CONFIG_FILES="${DEPLOY_CONFIG_FILES:-}"

require() {
  local missing=()
  for var in "$@"; do
    [[ -n "${!var:-}" ]] || missing+=("${var}")
  done
  if (( ${#missing[@]} > 0 )); then
    echo "error: missing required config: ${missing[*]}" >&2
    echo "       set them in the environment or in ${ENV_FILE}" >&2
    exit 1
  fi
}

require REGISTRY_PATH REGISTRY_USERNAME REGISTRY_PASSWORD
if [[ "${DO_DEPLOY}" == true ]]; then
  require SSH_HOST SSH_USER
fi
if [[ "${DO_CONFIG}" == true ]]; then
  require DEPLOY_CONFIG_FILES
fi

APP_IMAGE="${REGISTRY_PATH}/${IMAGE_NAME}:${TAG}"

# --- helpers -----------------------------------------------------------------

step() { printf '\n\033[1;34m==>\033[0m \033[1m%s\033[0m\n' "$*"; }
info() { printf '    %s\n' "$*"; }
warn() { printf '    \033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\n\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# ssh_run <remote command string>
# Runs the command on the server. Reads stdin, so it can also be the sink of a
# local pipe (used to stream the migrations/config tarballs across).
# Uses sshpass when SSH_PASSWORD is set, otherwise relies on key-based auth.
ssh_run() {
  local cmd="$1"
  local -a base=(ssh -o StrictHostKeyChecking=accept-new -p "${SSH_PORT}" -l "${SSH_USER}" "${SSH_HOST}")
  if [[ -n "${SSH_PASSWORD:-}" ]]; then
    command -v sshpass >/dev/null 2>&1 \
      || die "SSH_PASSWORD is set but sshpass is not installed (apt install sshpass)"
    SSHPASS="${SSH_PASSWORD}" sshpass -e "${base[@]}" "${cmd}"
  else
    "${base[@]}" "${cmd}"
  fi
}

command -v docker >/dev/null 2>&1 || die "docker is not installed"

step "Configuration"
[[ -n "${log_env:-}" ]] && info "config file : ${log_env}"
info "environment : ${APP_ENV:-<none>}"
info "registry    : ${REGISTRY_PATH}"
info "image       : ${APP_IMAGE}"
[[ "${DO_BUILD}"  == true ]] && info "platform    : ${BUILD_PLATFORM}"
[[ "${DO_DEPLOY}" == true ]] && info "server      : ${SSH_USER}@${SSH_HOST}:${SSH_PORT}"
[[ "${DO_DEPLOY}" == true ]] && info "remote dir  : ${REMOTE_DIR}"
if [[ "${DO_DEPLOY}" == true && "${DO_MIGRATIONS}" == true ]]; then
  info "migrations  : ${LOCAL_MIGRATIONS_DIR} -> ${REMOTE_MIGRATIONS_DIR}"
fi
if [[ "${DO_DEPLOY}" == true && "${DO_CONFIG}" == true ]]; then
  info "config      : ${DEPLOY_CONFIG_FILES} -> ${REMOTE_DIR}"
fi

# --- registry login ----------------------------------------------------------

step "Logging in to ${REGISTRY_PATH}"
printf '%s' "${REGISTRY_PASSWORD}" \
  | docker login "${REGISTRY_PATH}" --username "${REGISTRY_USERNAME}" --password-stdin

# --- build & push ------------------------------------------------------------

if [[ "${DO_BUILD}" == true ]]; then
  step "Building ${IMAGE_NAME} (${APP_IMAGE})"
  [[ -f "${ROOT_DIR}/Dockerfile" ]] || die "no Dockerfile in ${ROOT_DIR}"
  # shellcheck disable=SC2086
  docker build ${NO_CACHE} \
    --platform "${BUILD_PLATFORM}" \
    -f "${ROOT_DIR}/Dockerfile" \
    -t "${APP_IMAGE}" \
    "${ROOT_DIR}"
fi

if [[ "${DO_PUSH}" == true ]]; then
  step "Pushing ${APP_IMAGE}"
  docker push "${APP_IMAGE}"
fi

# --- file uploads ------------------------------------------------------------

# Streamed as a tarball over the existing ssh connection rather than via
# scp/rsync: no extra tool has to be installed on either side, and it works the
# same whether auth is key- or password-based.
upload_migrations() {
  step "Uploading migrations to ${REMOTE_MIGRATIONS_DIR}"

  [[ -d "${LOCAL_MIGRATIONS_DIR}" ]] || die "no migrations directory at ${LOCAL_MIGRATIONS_DIR}"

  local count
  count="$(find "${LOCAL_MIGRATIONS_DIR}" -maxdepth 1 -name '*.sql' -type f | wc -l)"
  (( count > 0 )) || die "${LOCAL_MIGRATIONS_DIR} contains no .sql files"
  info "${count} file(s) from ${LOCAL_MIGRATIONS_DIR}"

  local remote="set -euo pipefail; mkdir -p '${REMOTE_MIGRATIONS_DIR}';"
  if [[ "${PRUNE_MIGRATIONS}" == true ]]; then
    # Stale files are not harmless: a migration that was renamed locally would
    # otherwise linger and be applied twice under two different versions.
    info "pruning remote .sql files not present locally"
    remote+=" rm -f '${REMOTE_MIGRATIONS_DIR}'/*.sql;"
  fi
  remote+=" tar -xzf - -C '${REMOTE_MIGRATIONS_DIR}';"

  tar -czf - -C "${LOCAL_MIGRATIONS_DIR}" . | ssh_run "bash -c \"${remote}\""

  info "done"
}

# Vendor configs and their PEMs, uploaded preserving their repo-relative paths
# so the bind mounts in docker-compose.yml resolve. Sent as one tarball for the
# same reason as the migrations above.
upload_config() {
  step "Uploading config files to ${REMOTE_DIR}"

  local -a files=()
  local f
  for f in ${DEPLOY_CONFIG_FILES}; do
    [[ -f "${ROOT_DIR}/${f}" ]] || die "no such config file: ${ROOT_DIR}/${f}"
    # A vendor config pointing at a host path (e.g. an absolute
    # /home/... VENDOR_*_KEY_PATH) resolves inside the container, not on the
    # server, so it has to name a path the container can actually see.
    if [[ "${f}" == .env.* ]] && grep -qE '^VENDOR_(PRIVATE|PUBLIC)_KEY_PATH=/(home|Users)/' "${ROOT_DIR}/${f}"; then
      warn "${f} points VENDOR_*_KEY_PATH at a workstation path — the container will not find it"
    fi
    files+=("${f}")
    info "${f}"
  done

  local remote="set -euo pipefail; mkdir -p '${REMOTE_DIR}'; tar -xzf - -C '${REMOTE_DIR}';"
  tar -czf - -C "${ROOT_DIR}" "${files[@]}" | ssh_run "bash -c \"${remote}\""

  info "done"
}

if [[ "${DO_DEPLOY}" == true && "${DO_MIGRATIONS}" == true ]]; then
  upload_migrations
fi

if [[ "${DO_DEPLOY}" == true && "${DO_CONFIG}" == true ]]; then
  upload_config
fi

# --- remote deploy -----------------------------------------------------------

if [[ "${DO_DEPLOY}" == true ]]; then
  step "Deploying to ${SSH_HOST}"

  # Built on the remote side as a single non-interactive command so that a
  # failure anywhere aborts the whole deploy instead of silently continuing.
  remote_script="set -euo pipefail;"
  remote_script+=" printf '%s' \"\$REGISTRY_PASSWORD\" | docker login ${REGISTRY_PATH} --username ${REGISTRY_USERNAME} --password-stdin;"
  remote_script+=" cd ${REMOTE_DIR};"

  if [[ "${DO_APPLY_MIGRATIONS}" == true ]]; then
    # Run to completion (not -d) before the app restarts, so a failing
    # migration aborts the deploy instead of leaving the new binary talking to
    # an old schema. --rm keeps a one-shot container from piling up.
    remote_script+=" docker compose run --rm ${MIGRATE_SERVICE};"
  fi

  remote_script+=" docker compose pull ${COMPOSE_SERVICE};"
  remote_script+=" docker compose up -d ${COMPOSE_SERVICE};"

  # REGISTRY_PASSWORD is passed through the remote env rather than interpolated
  # into the command line, so it never shows up in the remote process list.
  # APP_ENV reaches the stack the same way, since docker-compose.yml reads it.
  ssh_run "REGISTRY_PASSWORD='${REGISTRY_PASSWORD}' APP_ENV='${APP_ENV}' bash -c '${remote_script}'"
fi

step "Done"
info "image : ${APP_IMAGE}"
exit 0
