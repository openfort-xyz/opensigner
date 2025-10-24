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

const dbhost = process.env.DB_HOST || "localhost";
const dbport = process.env.DB_PORT || "5432";
const dbname = process.env.DB_NAME || "authservice";
const dbuser = process.env.DB_USER || "postgres";
const dbpass = process.env.DB_PASS || "postgres";
const dsn = `postgres://${dbuser}:${dbpass}@${dbhost}:${dbport}/${dbname}?sslmode=disable`;

export const auth = betterAuth({
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
    secret: process.env.JWT_SECRET || "secret",
  },
  emailAndPassword: {
    enabled: true,
  },
});

const app = express();

app.use(cors());

app.all("/api/auth/*splat", toNodeHandler(auth));

app.get("/.well-known/jwks.json", (req: Request, res: Response) => {
  req.url = "/api/auth/jwks";
  toNodeHandler(auth)(req, res);
});

app.use(express.json());

// app.use(
//   cors({
//     origin: "http://localhost:7051",
//     methods: ["GET", "POST", "PUT", "DELETE"],
//     credentials: true, // Allow credentials (cookies, authorization headers, etc.)
//   }),
// );

app.get("/health", (req: Request, res: Response) => {
  res.json({ status: "ok" });
});

const HOST = process.env.HOST || "localhost";
const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`Better Auth server running on http://${HOST}:${PORT}`);
});
