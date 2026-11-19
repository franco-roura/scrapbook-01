/// <reference types="vite/client" />

// Build-time variables besides SST's VITE_API_URL (typed in sst-env.d.ts, which SST regenerates).
// All optional: leave VITE_SENTRY_DSN empty and Sentry is never loaded.
interface ImportMetaEnv {
  readonly VITE_SENTRY_DSN?: string;
  /** 0 to 1. Defaults to 1. */
  readonly VITE_SENTRY_TRACES_SAMPLE_RATE?: string;
}
