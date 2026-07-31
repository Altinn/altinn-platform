/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// When set (e.g. to a port-forwarded dis-console server), the dev server proxies
// /api to it. The browser then stays same-origin, so there is no CORS to handle
// and no backend change needed. Unset (the default) leaves mock mode untouched.
const apiTarget = process.env.DIS_CONSOLE_API;

export default defineConfig({
  plugins: [react()],
  server: apiTarget ? { proxy: { '/api': { target: apiTarget, changeOrigin: true } } } : undefined,
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
});
