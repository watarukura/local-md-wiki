import "dotenv/config"; // ← これを一番上に追加

import { serve } from "@hono/node-server";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createApp } from "./app.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = createApp({
  pagesDir: path.join(__dirname, "pages"),
  publicDir: path.join(__dirname, "public"),
  port: 3000,
});

const port = Number(process.env.LOCAL_MD_WIKI_PORT) || 3000;

serve({
  fetch: app.fetch,
  port,
});

console.log(`http://localhost:${port}`);
