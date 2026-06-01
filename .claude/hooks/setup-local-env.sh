#!/usr/bin/env bash
# Auto-bootstrap Go dev tooling for every service that declares `setup-local-env`,
# so a fresh worktree is ready for `make lint`/`make test` without ad-hoc downloads.
# Wired to the SessionStart (startup) hook in .claude/settings.json.
# Idempotent (make skips installed tools), non-fatal (never blocks the session),
# quiet (all output to stderr so nothing is injected into the model's context).
set -u
log() { printf '[setup-local-env] %s\n' "$*" >&2; }

# Resolve the worktree/repo root, cwd-independent and worktree-aware.
root="${CLAUDE_PROJECT_DIR:-}"
if [ -z "${root}" ] || { [ ! -e "${root}/.git" ]; }; then
  root="$(git rev-parse --show-toplevel 2>/dev/null || echo "${PWD}")"
fi
cd "${root}" 2>/dev/null || { log "cannot cd to '${root}', skipping"; exit 0; }

# Single-flight guard so parallel sessions don't double-bootstrap.
# Resolve the real git dir: in a linked worktree, "${root}/.git" is a
# `gitdir:` pointer *file*, so a lock dir can't be created under it. Using the
# resolved git dir also scopes the lock per worktree (each has its own bin/).
gitdir="$(git rev-parse --git-dir 2>/dev/null || echo "${root}/.git")"
lock="${gitdir}/.setup-local-env.lock"
if ! mkdir "${lock}" 2>/dev/null; then log "bootstrap already running, skipping"; exit 0; fi
trap 'rmdir "${lock}" 2>/dev/null || true' EXIT

shopt -s nullglob
ran=0
for mk in services/*/Makefile; do
  grep -qE '^setup-local-env:' "${mk}" || continue
  svc="$(dirname "${mk}")"; ran=1
  log "bootstrapping ${svc} …"
  if make -C "${svc}" setup-local-env >&2 2>&1; then
    log "ok: ${svc}"
  else
    log "WARNING: ${svc} bootstrap failed (continuing; rerun: make -C ${svc} setup-local-env)"
  fi
done
[ "${ran}" -eq 0 ] && log "no services declare setup-local-env; nothing to do"
exit 0   # always succeed: SessionStart must never block the session
