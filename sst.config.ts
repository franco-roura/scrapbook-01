/// <reference path="./.sst/platform/config.d.ts" />

// Scrapbook/01 on AWS, built to cost about $0 at hobby scale:
// - API + MCP: one Go Lambda behind an API Gateway HTTP API.
// - Classification: a Go worker Lambda, invoked asynchronously by the API
//   for each save, plus an hourly sweep. No queue service, nothing idle.
// - Web app (Expo web export) and landing page: S3 + CloudFront.
// - Database: Neon (free tier, outside AWS). No VPC, so no NAT gateway.
// Locally nothing here is needed: `go run ./cmd/api` in server/.
export default $config({
  app(input) {
    return {
      name: "scrapbook01",
      home: "aws",
      removal: input?.stage === "prod" ? "retain" : "remove",
      protect: input?.stage === "prod",
      // São Paulo, next to the Neon database and closest to Uruguay.
      providers: { aws: { region: "sa-east-1" } },
    };
  },
  async run() {
    const secret = {
      // Neon's pooled ("-pooler") connection string.
      databaseUrl: new sst.Secret("DatabaseUrl"),
      workosClientId: new sst.Secret("WorkosClientId"),
      workosAuthkitDomain: new sst.Secret("WorkosAuthkitDomain"),
      // Secret key (sk_...): turns on the landing page's waitlist form.
      workosApiKey: new sst.Secret("WorkosApiKey", ""),
      // Optional AI keys: an empty value turns that step off.
      typesafeApiKey: new sst.Secret("TypesafeApiKey", ""),
      embeddingsApiKey: new sst.Secret("EmbeddingsApiKey", ""),
      anthropicApiKey: new sst.Secret("AnthropicApiKey", ""),
      expoAccessToken: new sst.Secret("ExpoAccessToken", ""),
      // Optional Sentry (errors and performance), one project per surface.
      // An empty DSN turns reporting off for that surface.
      sentryDsnServer: new sst.Secret("SentryDsnServer", ""),
      sentryDsnMobile: new sst.Secret("SentryDsnMobile", ""),
      sentryDsnWebsite: new sst.Secret("SentryDsnWebsite", ""),
      // Uploads source maps at build time; optional even with a DSN.
      sentryAuthToken: new sst.Secret("SentryAuthToken", ""),
      sentryOrg: new sst.Secret("SentryOrg", ""),
    };
    // Sentry project slugs, for the source map upload (override with
    // SENTRY_PROJECT_MOBILE / SENTRY_PROJECT_WEBSITE when deploying).
    const sentryProject = {
      mobile: process.env.SENTRY_PROJECT_MOBILE ?? "scrapbook01-mobile",
      website: process.env.SENTRY_PROJECT_WEBSITE ?? "scrapbook01-website",
    };
    // Releases in Sentry match the repo version (package.json).
    const { readFileSync } = await import("node:fs");
    const version: string = JSON.parse(readFileSync(`${$cli.paths.root}/package.json`, "utf8")).version;

    // Settings both Lambdas read through config.Load().
    const shared = {
      DATABASE_URL: secret.databaseUrl.value,
      MIGRATE_ON_START: "false", // `bun run deploy` migrates first
      RUN_WORKERS: "false",
      WORKOS_CLIENT_ID: secret.workosClientId.value,
      WORKOS_AUTHKIT_DOMAIN: secret.workosAuthkitDomain.value,
      WORKOS_API_KEY: secret.workosApiKey.value,
      TYPESAFE_API_KEY: secret.typesafeApiKey.value,
      EMBEDDINGS_API_KEY: secret.embeddingsApiKey.value,
      ANTHROPIC_API_KEY: secret.anthropicApiKey.value,
      EXPO_ACCESS_TOKEN: secret.expoAccessToken.value,
      SENTRY_DSN: secret.sentryDsnServer.value,
      SENTRY_ENVIRONMENT: $app.stage,
    };
    const goFunction = {
      runtime: "provided.al2023" as const,
      architecture: "arm64" as const,
      memory: "512 MB" as const,
      logging: { retention: "1 week" as const },
      handler: "bootstrap",
    };

    // Each Lambda is compiled here into .build/<name>/bootstrap and handed to
    // SST as a prebuilt bundle. SST's own "go" runtime builds the package
    // path without its "./" on Windows, so Go looks for it in the standard
    // library ("package cmd/lambda-worker is not in std").
    const { execFileSync } = await import("node:child_process");
    const buildGo = (name: string) => {
      execFileSync(
        "go",
        ["build", "-tags", "lambda.norpc", "-trimpath", "-ldflags",
         `-s -w -X github.com/franco-roura/scrapbook-01/server/internal/telemetry.Version=${version}`,
         "-o", `../.build/${name}/bootstrap`, `./cmd/${name}`],
        {
          cwd: `${$cli.paths.root}/server`,
          stdio: "inherit",
          env: { ...process.env, GOOS: "linux", GOARCH: "arm64", CGO_ENABLED: "0" },
        },
      );
      return `.build/${name}`;
    };

    // Our copies of link preview images (platform image links are signed and
    // expire), served through CloudFront at <previewCdn>/previews/<item>.jpg.
    const previews = new sst.aws.Bucket("Previews", { access: "cloudfront" });
    const previewCdn = new sst.aws.Router("PreviewCdn");
    previewCdn.routeBucket("/previews", previews);

    const worker = new sst.aws.Function("Worker", {
      ...goFunction,
      bundle: buildGo("lambda-worker"),
      timeout: "5 minutes",
      environment: {
        ...shared,
        PREVIEW_BUCKET: previews.name,
        PREVIEW_BASE_URL: previewCdn.url,
      },
      permissions: [
        { actions: ["s3:PutObject"], resources: [$interpolate`${previews.arn}/previews/*`] },
      ],
    });

    const api = new sst.aws.ApiGatewayV2("Api", {
      // The Go API answers CORS itself (CORS_ORIGINS). SST's `cors: false`
      // still sets an empty CORS configuration, and with any configuration
      // API Gateway answers preflights itself and strips the API's CORS
      // headers, so drop it entirely.
      transform: {
        api: (args) => {
          args.corsConfiguration = undefined;
        },
      },
      accessLog: { retention: "1 week" },
    });
    // Created between the API and its route: the web build needs the API's
    // URL, and the API's CORS setting needs the web app's.
    const web = new sst.aws.StaticSite("Web", {
      path: "apps/mobile",
      build: { command: "bun x expo export --platform web", output: "dist" },
      // Expo's single-page build: every route is served by index.html.
      errorPage: "index.html",
      environment: {
        EXPO_PUBLIC_API_URL: api.url,
        EXPO_PUBLIC_WORKOS_CLIENT_ID: secret.workosClientId.value,
        EXPO_PUBLIC_SENTRY_DSN: secret.sentryDsnMobile.value,
        SENTRY_AUTH_TOKEN: secret.sentryAuthToken.value,
        SENTRY_ORG: secret.sentryOrg.value,
        SENTRY_PROJECT: sentryProject.mobile,
      },
    });

    // The landing page (Vite+, React, Module/01). Its waitlist form posts to
    // the API, which forwards to the WorkOS waitlist.
    const landing = new sst.aws.StaticSite("Landing", {
      path: "apps/website",
      build: { command: "bun run build", output: "dist" },
      environment: {
        VITE_API_URL: api.url,
        VITE_SENTRY_DSN: secret.sentryDsnWebsite.value,
        SENTRY_AUTH_TOKEN: secret.sentryAuthToken.value,
        SENTRY_ORG: secret.sentryOrg.value,
        SENTRY_PROJECT: sentryProject.website,
      },
    });

    api.route("$default", {
      ...goFunction,
      bundle: buildGo("lambda-api"),
      timeout: "29 seconds",
      environment: {
        ...shared,
        PUBLIC_URL: api.url,
        CORS_ORIGINS: $interpolate`${web.url},${landing.url}`,
        WORKER_FUNCTION_NAME: worker.name,
      },
      permissions: [{ actions: ["lambda:InvokeFunction"], resources: [worker.arn] }],
    });

    // Catches items whose classification was missed or waited for credits.
    // Hourly, so Neon (which wakes for 5 minutes per query) mostly sleeps.
    new sst.aws.CronV2("Sweep", {
      schedule: "rate(1 hour)",
      function: worker,
      event: { sweep: true },
    });

    return { api: api.url, web: web.url, landing: landing.url, previews: previewCdn.url };
  },
});
