#!/usr/bin/env bash
# Auto-bootstrap Go dev tooling for every service that declares `setup-local-env`,
# so a fresh worktree is ready for `make lint`/`make test` without ad-hoc downloads.
# Wired to the SessionStart (startup) hook in .claude/settings.json.
# Idempotent (make skips installed tools), non-fatal (never blocks the session),
# quiet (all output to stderr so nothing is injected into the model's context).
set -u
log() { printf '[setup-local-env] %s\n' "$*" >&2; }

# Emit the one and only line on stdout: a SessionStart JSON object whose
# `systemMessage` is shown to the user and whose `additionalContext` is injected
# into the model's context. All other output goes to stderr, so only this is
# surfaced. Args: $1=systemMessage, $2=additionalContext. Values are plain
# service names/words (no quotes, backslashes, or newlines), so no JSON escaping
# is needed and we avoid a jq dependency.
emit_session_json() {
  printf '{"systemMessage":"%s","hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"%s"}}\n' "$1" "$2"
}

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
ran=0; ok=""; fail=""
for mk in services/*/Makefile; do
  grep -qE '^setup-local-env:' "${mk}" || continue
  svc="$(dirname "${mk}")"; name="$(basename "${svc}")"; ran=1
  log "bootstrapping ${svc} …"
  if make -C "${svc}" setup-local-env >&2 2>&1; then
    log "ok: ${svc}"; ok="${ok:+${ok}, }${name}"
  else
    log "WARNING: ${svc} bootstrap failed (continuing; rerun: make -C ${svc} setup-local-env)"
    fail="${fail:+${fail}, }${name}"
  fi
done

if [ "${ran}" -eq 0 ]; then
  log "no services declare setup-local-env; nothing to do"
elif [ -z "${fail}" ]; then
  emit_session_json \
    "Dev tooling ready (auto-bootstrapped): ${ok}" \
    "A SessionStart hook bootstrapped Go dev tooling for: ${ok}. make lint/test use the installed bin/ binaries, so no ad-hoc download is needed."
else
  emit_session_json \
    "Dev tooling bootstrap finished with errors -- ok: [${ok:-none}], FAILED: [${fail}]" \
    "A SessionStart hook ran. Bootstrapped: ${ok:-none}. Failed: ${fail}. Rerun 'make -C services/<svc> setup-local-env' for the failed ones before make lint/test."
fi
exit 0   # always succeed: SessionStart must never block the session
