// The only module that imports the Sentry SDK, and only `./sentry` loads it, via dynamic import.
// Named re-exports (rather than `import("@sentry/react")`) let the bundler tree-shake the SDK down
// to what we use: no replay, feedback or profiling code ships.
export { browserTracingIntegration, captureException, init, startSpan } from "@sentry/react";
