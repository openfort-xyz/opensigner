import express from "express";
import cors from "cors";

// import bodyParser from "body-parser";
// import cors from "cors";
import { betterAuth } from "better-auth";
import { username } from "better-auth/plugins/username";
import { bearer } from "better-auth/plugins";
import { jwt } from "better-auth/plugins/jwt";
import { toNodeHandler } from "better-auth/node";
import { Pool } from "pg";
import type { Request, Response } from "express";

const jwtSecret = process.env.JWT_SECRET;
if (!jwtSecret) {
  console.error("FATAL: JWT_SECRET environment variable must be set");
  process.exit(1);
}

const dbhost = process.env.DB_HOST || "localhost";
const dbport = process.env.DB_PORT || "5432";
const dbname = process.env.DB_NAME || "authservice";
const dbuser = process.env.DB_USER || "postgres";
const dbpass = process.env.DB_PASS || "postgres";
const dbsslmode = process.env.DB_SSLMODE || "require";
const dsn = `postgres://${dbuser}:${dbpass}@${dbhost}:${dbport}/${dbname}?sslmode=${dbsslmode}`;
const baseURL = process.env.BETTER_AUTH_BASE_URL

const allowedOrigins = process.env.ALLOWED_ORIGINS
  ? process.env.ALLOWED_ORIGINS.split(",")
  : ["http://localhost:7050", "http://localhost:7051"];

export const auth = betterAuth({
  baseURL,
  database: new Pool({
    connectionString: dsn,
  }),
  plugins: [
    bearer(),
    jwt(),
    username({
      maxUsernameLength: 30,
      usernameValidator: (username: string) => {
        if (username === "admin") {
          return false;
        }
        return true;
      },
    }),
  ],
  jwt: {
    secret: jwtSecret,
  },
  emailAndPassword: {
    enabled: true,
  },
  trustedOrigins: allowedOrigins
});

const app = express();

app.use(
  cors({
    origin: allowedOrigins,
    methods: ["GET", "POST", "PUT", "DELETE"],
    credentials: true,
  }),
);

app.all("/api/auth/*splat", toNodeHandler(auth));

app.get("/.well-known/jwks.json", (req: Request, res: Response) => {
  req.url = "/api/auth/jwks";
  toNodeHandler(auth)(req, res);
});

app.use(express.json());

// Origin validation endpoint for nginx auth_request subrequest.
// Nginx sends X-Request-Origin header; returns 200 if allowed, 403 if not.
// Also returns X-Allowed-Origins header for CSP frame-ancestors.
app.get("/v1/projects/validate-origin", (req: Request, res: Response) => {
  // Default missing origin to a safe value that won't match any allowed origin,
  // matching the backend's pattern (ProjectController.ts defaults to "openfort.io").
  const requestOrigin = (req.headers["x-request-origin"] as string | undefined) || "openfort.io";
  const allowedOriginsStr = allowedOrigins.join(" ");

  const isAllowed = allowedOrigins.some(
    (origin) => requestOrigin === origin
  );

  if (isAllowed) {
    res.set("X-Allowed-Origins", allowedOriginsStr);
    res.sendStatus(200);
  } else {
    res.sendStatus(403);
  }
});

app.get("/health", (req: Request, res: Response) => {
  res.json({ status: "ok" });
});

const HOST = process.env.HOST || "localhost";
const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`Better Auth server running on http://${HOST}:${PORT}`);
});
