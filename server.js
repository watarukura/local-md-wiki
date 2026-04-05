import { serve } from "@hono/node-server";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createApp } from "./app.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = createApp({
  pagesDir: path.join(__dirname, "pages"),
  publicDir: path.join(__dirname, "public")
});

serve({
  fetch: app.fetch,
  port: 3000
});

console.log("http://localhost:3000");