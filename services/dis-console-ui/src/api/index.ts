import type { FleetApi } from './client';
import { HttpFleetApi } from './http';
import { MockFleetApi } from './mock';

interface RuntimeConfig {
  useMock?: boolean;
}

// The runtime config wins: the BFF serves /config.js from its environment
// (live whenever DIS_CONSOLE_API is set), so one image serves every mode.
// The Vite env is the dev-server fallback; mock stays the default.
const runtime = (globalThis as { __DIS_CONSOLE__?: RuntimeConfig }).__DIS_CONSOLE__;
export const useMock = runtime?.useMock ?? import.meta.env.VITE_USE_MOCK !== 'false';

export const api: FleetApi = useMock ? new MockFleetApi() : new HttpFleetApi();

export type { FleetApi, ResourceFilters } from './client';
