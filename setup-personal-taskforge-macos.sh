#!/usr/bin/env bash
set -Eeuo pipefail

# TaskForge local development bootstrap for macOS.
# Installs tools, starts Colima, provisions a local PostgreSQL container,
# and adds the initial Go dependencies to the existing modules.
# It does not delete existing containers/volumes or overwrite project files.

PROJECT_ROOT="${PROJECT_ROOT:-$PWD}"
PG_CONTAINER="${PG_CONTAINER:-taskforge-postgres}"
PG_VOLUME="${PG_VOLUME:-taskforge-postgres-data}"
PG_USER="${PG_USER:-taskforge}"
PG_PASSWORD="${PG_PASSWORD:-taskforge}"
PG_DATABASE="${PG_DATABASE:-taskforge}"
PG_PORT="${PG_PORT:-5432}"
PG_IMAGE="${PG_IMAGE:-postgres:17}"
COLIMA_CPU="${COLIMA_CPU:-4}"
COLIMA_MEMORY="${COLIMA_MEMORY:-8}"
COLIMA_DISK="${COLIMA_DISK:-60}"
K3D_CLUSTER="${K3D_CLUSTER:-k8s-lab}"
K3D_SERVERS="${K3D_SERVERS:-1}"
K3D_AGENTS="${K3D_AGENTS:-2}"
K3S_IMAGE="${K3S_IMAGE:-rancher/k3s:v1.35.5-k3s1}"

say() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
die() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }

[[ "$(uname -s)" == "Darwin" ]] || die "This script is intended for macOS."
command -v brew >/dev/null 2>&1 || die "Homebrew is required. Install it from https://brew.sh, then rerun this script."

say "Installing development tools with Homebrew"
brew update
brew list --versions go >/dev/null 2>&1 || brew install go
brew list --versions colima >/dev/null 2>&1 || brew install colima
# brew list --versions docker >/dev/null 2>&1 || brew install docker
brew list --versions kubectl >/dev/null 2>&1 || brew install kubectl
brew list --versions k3d >/dev/null 2>&1 || brew install k3d
brew list --versions helm >/dev/null 2>&1 || brew install helm
brew list --versions k9s >/dev/null 2>&1 || brew install k9s
brew list --versions sqlc >/dev/null 2>&1 || brew install sqlc
brew list --versions golang-migrate >/dev/null 2>&1 || brew install golang-migrate

say "Checking Go and Docker CLI"
go version
docker --version
kubectl version --client
k3d version
helm version --short
k9s version --short || true
sqlc version
migrate -version

say "Starting Colima if needed"
if ! colima status >/dev/null 2>&1; then
  colima start --cpu "$COLIMA_CPU" --memory "$COLIMA_MEMORY" --disk "$COLIMA_DISK"
else
  printf "Colima is already running; keeping its existing CPU/memory/disk settings.\n"
fi

say "Creating persistent PostgreSQL volume if missing"
if ! docker volume inspect "$PG_VOLUME" >/dev/null 2>&1; then
  docker volume create "$PG_VOLUME" >/dev/null
fi

say "Creating PostgreSQL container if missing"
if docker container inspect "$PG_CONTAINER" >/dev/null 2>&1; then
  docker start "$PG_CONTAINER" >/dev/null
else
  docker run -d \
    --name "$PG_CONTAINER" \
    --restart unless-stopped \
    -e "POSTGRES_USER=$PG_USER" \
    -e "POSTGRES_PASSWORD=$PG_PASSWORD" \
    -e "POSTGRES_DB=$PG_DATABASE" \
    -p "${PG_PORT}:5432" \
    -v "${PG_VOLUME}:/var/lib/postgresql/data" \
    "$PG_IMAGE" >/dev/null
fi

say "Waiting for PostgreSQL to become ready"
ready=0
for _ in $(seq 1 40); do
  if docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" -d "$PG_DATABASE" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
[[ "$ready" == "1" ]] || die "PostgreSQL did not become ready. Inspect with: docker logs $PG_CONTAINER"

say "Creating the personal-laptop-equivalent k3d cluster if missing"
if k3d cluster list --no-headers 2>/dev/null | awk '{print $1}' | grep -Fxq "$K3D_CLUSTER"; then
  printf 'k3d cluster %s already exists; leaving it unchanged.\n' "$K3D_CLUSTER"
else
  k3d cluster create "$K3D_CLUSTER" \
    --servers "$K3D_SERVERS" \
    --agents "$K3D_AGENTS" \
    --image "$K3S_IMAGE"
fi

say "Verifying Kubernetes nodes"
kubectl config use-context "k3d-$K3D_CLUSTER" >/dev/null
kubectl wait --for=condition=Ready nodes --all --timeout=180s
kubectl get nodes

say "Checking expected TaskForge modules"
if [[ ! -f "$PROJECT_ROOT/go.mod" ]]; then
  printf 'Note: no root go.mod found under %s; skipping Go dependency setup.\n' "$PROJECT_ROOT"
else
  # Add dependencies only when the relevant module already exists.
  if [[ -d "$PROJECT_ROOT/domain" ]]; then
    (cd "$PROJECT_ROOT" && go get github.com/google/uuid@v1.6.0)
  fi
  if [[ -f "$PROJECT_ROOT/sqlc/go.mod" ]]; then
    (cd "$PROJECT_ROOT/sqlc" && go get github.com/jackc/pgx/v5)
  else
    printf 'Note: %s/sqlc/go.mod not found; skipped sqlc module dependency setup.\n' "$PROJECT_ROOT"
  fi
  if [[ -f "$PROJECT_ROOT/bun/go.mod" ]]; then
    (cd "$PROJECT_ROOT/bun" && go get github.com/uptrace/bun github.com/uptrace/bun/dialect/pgdialect github.com/jackc/pgx/v5)
  else
    printf 'Note: %s/bun/go.mod not found; skipped Bun dependency setup.\n' "$PROJECT_ROOT"
  fi
fi

say "Setup complete"
cat <<SUMMARY
PostgreSQL container: $PG_CONTAINER
Database:             $PG_DATABASE
User:                 $PG_USER
Port:                 $PG_PORT
Connection URL:       postgres://$PG_USER:$PG_PASSWORD@localhost:$PG_PORT/$PG_DATABASE?sslmode=disable
Persistent volume:    $PG_VOLUME

Useful commands:
  docker ps
  docker logs $PG_CONTAINER
  kubectl get nodes
  kubectl config get-contexts
  k9s                         # interactive Kubernetes TUI
  helm list -A
  k3d cluster delete $K3D_CLUSTER   # when you intentionally want to remove the lab cluster
  docker exec -it $PG_CONTAINER psql -U $PG_USER -d $PG_DATABASE
  sqlc generate                 # run from the directory containing sqlc.yaml

Kubernetes cluster:  $K3D_CLUSTER
K3d topology:        $K3D_SERVERS server, $K3D_AGENTS agents
K3s image:           $K3S_IMAGE
Colima resources:    $COLIMA_CPU CPUs, $COLIMA_MEMORY GiB RAM, $COLIMA_DISK GiB disk
Tools installed:     Go, Docker CLI, Colima, kubectl, k3d, Helm, k9s, sqlc, golang-migrate
The database volume persists when the container is restarted or removed.
Do not run 'docker volume rm $PG_VOLUME' unless you intentionally want to delete its data.
SUMMARY
