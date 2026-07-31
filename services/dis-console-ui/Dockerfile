# syntax=docker/dockerfile:1

# --- Build the static assets with Bun + Vite ---
FROM oven/bun:1 AS builder
WORKDIR /app
# Install from the lockfile first so dependency layers cache across source edits.
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
COPY . .
RUN bun run build

# --- Serve with the Bun BFF (static + optional /api proxy) on :8080 ---
# The server has no runtime dependencies (Bun.serve is built in), so the runtime
# image carries only dist/ + the server, no node_modules.
FROM oven/bun:1-alpine AS runtime
# Pick up Alpine security fixes published after the base image was built
# (the Trivy gate blocks known HIGH/CRITICAL CVEs in the final image).
RUN apk --no-cache upgrade
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/server ./server
USER bun
ENV PORT=8080
EXPOSE 8080
CMD ["bun", "server/index.ts"]
