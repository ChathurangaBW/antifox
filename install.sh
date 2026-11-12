#!/usr/bin/env bash
# AntiFox Installation script:① All Docker  ② Local compiled running
set -euo pipefail
cd "$(cd "$(dirname "$0")" && pwd)"

info(){ printf '\033[36m[*]\033[0m %s\n' "$*"; }
ok(){   printf '\033[32m[+]\033[0m %s\n' "$*"; }
warn(){ printf '\033[33m[!]\033[0m %s\n' "$*"; }
die(){  printf '\033[31m[x]\033[0m %s\n' "$*" >&2; exit 1; }
ask(){  local p="$1" d="${2:-}" a; read -rp "$p${d:+ [$d]}: " a; echo "${a:-$d}"; }
rand(){ head -c 18 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24; }

# ── docker Environmental testing / Auto Install ───────────────────
ensure_docker(){
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    ok "Detected docker and docker compose"; return
  fi
  warn "Not detected docker / docker compose"
  case "$(uname -s)" in
    Linux)
      if [ "$(ask 'Auto Install Docker? (y/n)' y)" = y ]; then
        curl -fsSL https://get.docker.com | sh
        sudo usermod -aG docker "$USER" || true
        ok "Docker Installation complete (re-entry required for user group changes sudo)"
      else
        die "Please install yourself. docker Try again after"
      fi ;;
    Darwin) die "macOS Please install Docker Desktop:https://www.docker.com/products/docker-desktop/" ;;
    *)      die "Please install yourself. docker Try again after" ;;
  esac
}

# ── ① All Docker ───────────────────────────────
install_docker(){
  ensure_docker
  if [ ! -f .env ]; then
    cp .env.example .env 2>/dev/null || true
    local pw key
    pw="$(ask 'Postgres Password (random generation of return vehicles)' "$(rand)")"
    key="$(ask 'ANTHROPIC_API_KEY(We can leave room for later. UI Matching)' '')"
    sed -i.bak "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${pw}|" .env
    sed -i.bak "s|^ANTHROPIC_API_KEY=.*|ANTHROPIC_API_KEY=${key}|" .env
    rm -f .env.bak
    ok "Generated .env(POSTGRES_PASSWORD Set)"
  else
    info "Continue existing .env"
  fi
  info "Pull mirrors and start...."
  docker compose pull || true
  docker compose up -d
  ok "Start complete. → http://localhost:8787"
  info "View log:docker compose logs -f antifox"
}

# ── ② Local compiled running ──────────────────────────────
install_local(){
  echo "Database installation:"
  echo "  1) Connection already exists PostgreSQL"
  echo "  2) Start a PostgreSQL with Docker (docker required)"
  case "$(ask 'Selection' 1)" in
    2)
      ensure_docker
      local pw; pw="$(ask 'Postgres Password (go-back random)' "$(rand)")"
      docker run -d --name antifox-pg -p 5432:5432 \
        -e POSTGRES_USER=antifox -e POSTGRES_PASSWORD="$pw" -e POSTGRES_DB=antifox \
        -v antifox-pg:/var/lib/postgresql/data postgres:16-alpine
      DB_HOST=127.0.0.1 DB_PORT=5432 DB_USER=antifox DB_PASS="$pw" DB_NAME=antifox DB_SSL=disable ;;
    *)
      DB_HOST="$(ask 'Database Address' 127.0.0.1)"
      DB_PORT="$(ask 'Port' 5432)"
      DB_USER="$(ask 'Account' antifox)"
      DB_PASS="$(ask 'Password' '')"
      DB_NAME="$(ask 'Database Name' antifox)"
      DB_SSL="$(ask 'sslmode (disable/require)' disable)" ;;
  esac

  # Generate config.json
  cat > config.json <<JSON
{
  "database": {
    "host": "${DB_HOST}",
    "port": ${DB_PORT},
    "user": "${DB_USER}",
    "password": "${DB_PASS}",
    "dbname": "${DB_NAME}",
    "sslmode": "${DB_SSL}"
  }
}
JSON
  ok "Generated config.json"

  # go Environmental inspection
  command -v go >/dev/null 2>&1 || die "Not detected Go,Please install first. Go(>=1.26):https://go.dev/dl/"
  ok "Go: $(go version)"

  # Embedded frontend needs node Static products
  if command -v npm >/dev/null 2>&1; then
    info "Build front-end static product..."
    ( cd web && npm ci && npm run build:static )
    rm -rf server/webui/dist && cp -r web/out server/webui/dist
    info "Compile Embedded Monochrome..."
    CGO_ENABLED=0 go build -tags embedui -trimpath -o antifox ./cmd/antifox
  else
    warn "Not detected npm:To be compiled**Do not embed frontend**Backend (first end to run) npm run dev)"
    CGO_ENABLED=0 go build -o antifox ./cmd/antifox
  fi
  ok "Compiled → ./antifox"

  info "Start...(Ctrl-C Quit)"
  ./antifox
}

echo "=============================="
echo "  AntiFox Install"
echo "  1) All Docker Install"
echo "  2) Run locally (go Compiled)"
echo "=============================="
case "$(ask 'Selection' 1)" in
  1) install_docker ;;
  2) install_local ;;
  *) die "Invalid Selection" ;;
esac
