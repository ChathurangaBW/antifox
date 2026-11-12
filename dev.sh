#!/usr/bin/env bash
# Development mode: Backend(:8787) + Flow agent(:8788) With frontend next dev(:5173) Run together.
# Frontend /api Inverse to backend;Ctrl-C Quit together.
#
# See single binary (front end embedded) mode README"Single binary section, do not take this script.
set -euo pipefail
cd "$(dirname "$0")"

# End all subprocesses in this process group on exit (backend) + Frontend.
cleanup() { kill 0 2>/dev/null || true; }
trap cleanup EXIT INT TERM

# Backend (ordinary) go run,Do not embed frontend; co-op work agent Numbers are configured in System Settings.
go run ./cmd/antifox -addr :8787 -proxy :8788 &

# Frontend Thermal Updates (Vite/Next dev server,/api Inversely:8787).
( cd web && npm run dev ) &

echo "[dev] Backend :8787 / Proxy :8788 / Frontend http://localhost:5173  (Ctrl-C Exit)"
wait
