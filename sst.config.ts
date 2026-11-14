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
      providers: { aws: { region: "us-east-1" } },
    };
  },
  async run() {
    const secret = {
      // Neon's pooled ("-pooler") connection string.
      databaseUrl: new sst.Secret("DatabaseUrl"),
      workosClientId: new sst.Secret("WorkosClientId"),
      workosAuthkitDomain: new sst.Secret("WorkosAuthkitDomain"),
      // Optional AI keys: an empty value turns that step off.
      typesafeApiKey: new sst.Secret("TypesafeApiKey", ""),
      embeddingsApiKey: new sst.Secret("EmbeddingsApiKey", ""),
      anthropicApiKey: new sst.Secret("AnthropicApiKey", ""),
      expoAccessToken: new sst.Secret("ExpoAccessToken", ""),
    };

    // Settings both Lambdas read through config.Load().
    const shared = {
      DATABASE_URL: secret.databaseUrl.value,
      MIGRATE_ON_START: "false", // `bun run deploy` migrates first
      RUN_WORKERS: "false",
      WORKOS_CLIENT_ID: secret.workosClientId.value,
      WORKOS_AUTHKIT_DOMAIN: secret.workosAuthkitDomain.value,
      TYPESAFE_API_KEY: secret.typesafeApiKey.value,
      EMBEDDINGS_API_KEY: secret.embeddingsApiKey.value,
      ANTHROPIC_API_KEY: secret.anthropicApiKey.value,
      EXPO_ACCESS_TOKEN: secret.expoAccessToken.value,
    };
    const goFunction = {
      runtime: "go" as const,
      architecture: "arm64" as const,
      memory: "512 MB" as const,
      logging: { retention: "1 week" as const },
    };

    const worker = new sst.aws.Function("Worker", {
      ...goFunction,
      handler: "./server/cmd/lambda-worker",
      timeout: "5 minutes",
      environment: shared,
    });

    const api = new sst.aws.ApiGatewayV2("Api", {
      cors: false, // the Go API answers CORS itself (CORS_ORIGINS)
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
      },
    });

    api.route("$default", {
      ...goFunction,
      handler: "./server/cmd/lambda-api",
      timeout: "29 seconds",
      environment: {
        ...shared,
        PUBLIC_URL: api.url,
        CORS_ORIGINS: web.url,
        WORKER_FUNCTION_NAME: worker.name,
      },
      permissions: [{ actions: ["lambda:InvokeFunction"], resources: [worker.arn] }],
    });

    // The landing page and waitlist (Vite+, React, Module/01).
    const landing = new sst.aws.StaticSite("Landing", {
      path: "apps/website",
      build: { command: "bun run build", output: "dist" },
    });

    // Catches items whose classification was missed or waited for credits.
    // Hourly, so Neon (which wakes for 5 minutes per query) mostly sleeps.
    new sst.aws.CronV2("Sweep", {
      schedule: "rate(1 hour)",
      function: worker,
      event: { sweep: true },
    });

    return { api: api.url, web: web.url, landing: landing.url };
  },
});
