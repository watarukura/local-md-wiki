import "dotenv/config";

import { createApp } from "./app.js";

const port = Number(process.env.LOCAL_MD_WIKI_PORT) || 3000;

const app = createApp();

if (typeof Bun !== "undefined") {
  console.log(`Bun server running at http://localhost:${port}`);
  Bun.serve({
    fetch: app.fetch,
    port,
  });
} else {
  const { serve } = await import("@hono/node-server");
  console.log(`Node server running at http://localhost:${port}`);
  serve({
    fetch: app.fetch,
    port,
  });
}
