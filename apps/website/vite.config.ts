import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, lazyPlugins } from "vite-plus";

// With SENTRY_AUTH_TOKEN set, source maps are built "hidden" (no sourceMappingURL comment in the
// bundle), uploaded to SENTRY_ORG/SENTRY_PROJECT by the Sentry plugin and deleted from dist, so
// they're never served. Without it there are no source maps and the plugin isn't even loaded.
const sentryAuthToken = process.env.SENTRY_AUTH_TOKEN;

// Vite+ (https://viteplus.dev): one config for dev, build, test, lint and format.
export default defineConfig({
  fmt: {},
  lint: {
    plugins: ["react", "typescript", "oxc"],
    rules: {
      "react/rules-of-hooks": "error",
      "react/only-export-components": [
        "warn",
        {
          allowConstantExport: true,
        },
      ],
      "vite-plus/prefer-vite-plus-imports": "error",
    },
    options: {
      typeAware: true,
      typeCheck: true,
    },
    jsPlugins: [
      {
        name: "vite-plus",
        specifier: "vite-plus/oxlint-plugin",
      },
    ],
  },
  build: { sourcemap: sentryAuthToken ? "hidden" : false },
  plugins: lazyPlugins(async () => [
    react(),
    tailwindcss(),
    // Last, per Sentry's docs.
    ...(sentryAuthToken
      ? (await import("@sentry/vite-plugin")).sentryVitePlugin({
          authToken: sentryAuthToken,
          org: process.env.SENTRY_ORG,
          project: process.env.SENTRY_PROJECT,
          telemetry: false,
          sourcemaps: { filesToDeleteAfterUpload: ["./dist/**/*.map"] },
        })
      : []),
  ]),
});
