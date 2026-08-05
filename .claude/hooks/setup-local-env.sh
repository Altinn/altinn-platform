#!/usr/bin/env bash
# Auto-bootstrap Go dev tooling for every service that declares `setup-local-env`,
# so a fresh worktree is ready for `make lint`/`make test` without ad-hoc downloads.
# Wired to the SessionStart (startup) hook in .claude/settings.json.
#
# NON-BLOCKING: the launcher detaches the work and returns immediately, so the
# session opens without waiting; the bootstrap runs in a detached worker that
# logs to <git-dir>/setup-local-env.log.
# FAST: within each service the independent tools (kustomize, controller-gen,
# golangci-lint, setup-envtest) are pre-built in parallel with `make -j`; the
# service's own `setup-local-env` then runs as the authoritative finalizer (a
# no-op for the now-installed tools), so behaviour stays identical to running it
# directly while the slow compile (golangci-lint dominates) is parallelised.
# Idempotent, non-fatal, single-flight (lock dir). Services run sequentially
# because they share a root go.work that `make workspace` rewrites.
set -u
log() { printf '[setup-local-env] %s\n' "$*" >&2; }

# Resolve the worktree/repo root, cwd-independent and worktree-aware.
root="${CLAUDE_PROJECT_DIR:-}"
if [ -z "${root}" ] || { [ ! -e "${root}/.git" ]; }; then
  root="$(git rev-parse --show-toplevel 2>/dev/null || echo "${PWD}")"
fi
cd "${root}" 2>/dev/null || { log "cannot cd to '${root}', skipping"; exit 0; }

# Resolve the real git dir (a real directory even in linked worktrees, where
# "${root}/.git" is a `gitdir:` pointer file) for the lock and log paths.
gitdir="$(git rev-parse --git-dir 2>/dev/null || echo "${root}/.git")"
lock="${gitdir}/.setup-local-env.lock"
logfile="${gitdir}/setup-local-env.log"

# --- worker mode: the actual bootstrap, run detached in the background --------
if [ "${1:-}" = "--worker" ]; then
  # Single-flight: hold the lock for the worker's whole lifetime.
  if ! mkdir "${lock}" 2>/dev/null; then log "bootstrap already running, skipping"; exit 0; fi
  trap 'rmdir "${lock}" 2>/dev/null || true' EXIT

  jobs="$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)"
  shopt -s nullglob
  ran=0
  for mk in services/*/Makefile; do
    grep -qE '^setup-local-env:' "${mk}" || continue
    svc="$(dirname "${mk}")"; ran=1
    log "bootstrapping ${svc} (pre-build -j${jobs}) …"
    # Best-effort parallel pre-build of the independent tool targets when the
    # service uses the standard ones. Failures are ignored on purpose — the
    # authoritative `make setup-local-env` below still runs and is the source of
    # truth (so this can never skip a step the real target performs).
    if grep -qE '^kustomize:' "${mk}" && grep -qE '^controller-gen:' "${mk}" \
       && grep -qE '^golangci-lint:' "${mk}" && grep -qE '^setup-envtest:' "${mk}"; then
      make -C "${svc}" -j"${jobs}" kustomize controller-gen golangci-lint setup-envtest >&2 2>&1 || true
    fi
    if make -C "${svc}" setup-local-env >&2 2>&1; then
      log "ok: ${svc}"
    else
      log "WARNING: ${svc} bootstrap failed (rerun: make -C ${svc} setup-local-env)"
    fi
  done
  [ "${ran}" -eq 0 ] && log "no services declare setup-local-env; nothing to do"
  log "bootstrap complete"
  exit 0
fi

# --- launcher mode (what the hook invokes): detach the worker, return now -----
[ -d "${lock}" ] && exit 0   # a bootstrap is already in flight; nothing to launch
# Detach so the bootstrap survives the hook returning; worker output -> log file.
nohup bash "$0" --worker </dev/null >"${logfile}" 2>&1 &
disown 2>/dev/null || true
exit 0   # return immediately: SessionStart must never block the session
