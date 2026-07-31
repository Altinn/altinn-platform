/// <reference types="vite/client" />

// Side-effect-only stylesheet entrypoints exposed by their package `exports`.
declare module '@digdir/designsystemet-css';
declare module '@digdir/designsystemet-css/theme';
declare module '@fontsource/inter';

interface ImportMetaEnv {
  readonly VITE_USE_MOCK?: string;
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_AZURE_PORTAL_TENANT?: string;
  readonly VITE_GRAFANA_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
