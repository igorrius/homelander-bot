// Template of the Railway infrastructure for one Homelander deployment.
//
// Copy it to .railway/railway.ts (that path is git-ignored: your deployment's
// project name and domain stay on your computer) and fill in the two constants
// below. See docs/deploy-railway.md.
//
// Secrets and personal data (APP_SECRET, TELEGRAM_BOT_TOKEN,
// TELEGRAM_WEBHOOK_SECRET, BOOTSTRAP_ADMIN_PHONE) are set in the Railway UI and
// kept with preserve(): their values never enter this file.
// Always run `railway config plan` before `railway config apply`.
import { defineRailway, github, preserve, project, service, volume } from "railway/iac";

// The Railway project name, exactly as in the Railway UI.
const projectName = "homelander";
// The service domain once Railway generated it, e.g. "something.up.railway.app"; "" until then.
const domain = "";
const region = "europe-west4-drams3a";

export default defineRailway(() => {
  // The SQLite database. Never remove it from this file: Railway would delete the data.
  const data = volume("homelander-data", {
    alerts: { usage: { "80": {}, "95": {}, "100": {} } },
    allowOnlineResize: true,
    region,
    sizeMB: 1000,
  });

  const app = service("homelander", {
    // Deploys every push to main, only after the GitHub checks (.github/workflows/ci.yml) pass.
    source: github("igorrius/homelander-bot", { branch: "main", checkSuites: true }),
    build: { buildEnvironment: "V3", builder: "DOCKERFILE", dockerfilePath: "Dockerfile" },
    deploy: {
      healthcheckPath: "/healthz",
      healthcheckTimeout: 30,
      // Restart on failure is Railway's default and stored as null; setting
      // restartPolicyType makes `plan` report a false diff.
      restartPolicyMaxRetries: 5,
    },
    // Exactly one replica: SQLite has one writer and the rate limits live in memory.
    replicas: { [region]: 1 },
    networking: domain ? { serviceDomains: { [domain]: {} } } : undefined,
    volumeMounts: { "/data": data },
    variables: {
      // Secrets and personal data: set in the Railway UI, preserved here.
      APP_SECRET: preserve(),
      TELEGRAM_BOT_TOKEN: preserve(),
      TELEGRAM_WEBHOOK_SECRET: preserve(),
      BOOTSTRAP_ADMIN_PHONE: preserve(),
      // Plain settings.
      PUBLIC_URL: "https://${{RAILWAY_PUBLIC_DOMAIN}}",
      TELEGRAM_UPDATES: "webhook",
      TRUST_PROXY: "true",
      // Railway mounts volumes owned by root; the image runs as nonroot.
      RAILWAY_RUN_UID: "0",
      DATABASE_PATH: "/data/homelander.db",
      DEFAULT_LOCALE: "uk",
      DEFAULT_PHONE_REGION: "UA",
      LOG_LEVEL: "info",
    },
  });

  return project(projectName, {
    resources: [app, data],
  });
});
