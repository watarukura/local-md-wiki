import { Hono } from "hono";
import { serveStatic } from "@hono/node-server/serve-static";
import fs from "node:fs";
import path from "node:path";
import { marked } from "marked";

export function createApp(options = {}) {
  if (!options.pagesDir) {
    throw new Error("pagesDir is required");
  }
  if (!options.publicDir) {
    throw new Error("publicDir is required");
  }

  const pagesDir = path.resolve(options.pagesDir);
  const publicDir = path.resolve(options.publicDir);
  const uploadsDir = path.join(publicDir, "uploads");

  const app = new Hono();

  function ensureDir(dir) {
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
  }

  function normalizePageName(input) {
    let name = String(input || "").trim();
    name = name.replace(/\\/g, "/");
    name = path.posix.normalize(name);

    if (!name || name === "." || name === ".." || name.startsWith("../") || path.isAbsolute(name)) {
      throw new Error("invalid page name");
    }

    if (!name.endsWith(".md")) {
      name += ".md";
    }

    return name;
  }

  function pagePath(pageName) {
    const normalized = normalizePageName(pageName);
    const full = path.join(pagesDir, normalized);
    const resolved = path.resolve(full);

    if (!resolved.startsWith(pagesDir)) {
      throw new Error("invalid page path");
    }

    return resolved;
  }

  function listMarkdownFiles(dir, base = "") {
    if (!fs.existsSync(dir)) return [];

    const entries = fs.readdirSync(dir, { withFileTypes: true });
    const files = [];

    for (const entry of entries) {
      const rel = path.posix.join(base, entry.name);
      const full = path.join(dir, entry.name);

      if (entry.isDirectory()) {
        files.push(...listMarkdownFiles(full, rel));
      } else if (entry.isFile() && entry.name.endsWith(".md")) {
        files.push(rel);
      }
    }

    return files.sort((a, b) => a.localeCompare(b));
  }

  function extractInternalLinks(markdown, currentPage) {
    const regex = /\[([^\]]*?)\]\(([^)]+)\)/g;
    const links = [];
    let match;

    while ((match = regex.exec(markdown)) !== null) {
      const rawHref = match[2].trim();

      if (!rawHref) continue;
      if (/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(rawHref)) continue;
      if (rawHref.startsWith("#")) continue;

      const hrefWithoutHash = rawHref.split("#")[0];
      const hrefWithoutQuery = hrefWithoutHash.split("?")[0];
      if (!hrefWithoutQuery) continue;

      let decoded;
      try {
        decoded = decodeURIComponent(hrefWithoutQuery);
      } catch {
        decoded = hrefWithoutQuery;
      }

      const currentDir = path.posix.dirname(currentPage);
      const resolved = path.posix.normalize(path.posix.join(currentDir, decoded));

      if (resolved.startsWith("../") || resolved === "..") continue;
      if (!resolved.endsWith(".md")) continue;

      links.push(resolved);
    }

    return [...new Set(links)].sort((a, b) => a.localeCompare(b));
  }

  function buildGraph() {
    const files = listMarkdownFiles(pagesDir);
    const graph = {};

    for (const file of files) {
      const full = path.join(pagesDir, file);
      const content = fs.readFileSync(full, "utf8");
      graph[file] = extractInternalLinks(content, file);
    }

    return graph;
  }

  function backlinksOf(targetPage, graph) {
    return Object.entries(graph)
      .filter(([page, links]) => page !== targetPage && links.includes(targetPage))
      .map(([page]) => page)
      .sort((a, b) => a.localeCompare(b));
  }

  function twoHopOf(targetPage, graph) {
    const outgoing = new Set(graph[targetPage] || []);
    const scores = new Map();

    for (const [page, links] of Object.entries(graph)) {
      if (page === targetPage) continue;

      let shared = 0;
      for (const link of links) {
        if (outgoing.has(link)) shared++;
      }

      if (shared > 0) {
        scores.set(page, shared);
      }
    }

    return [...scores.entries()]
      .sort((a, b) => {
        if (b[1] !== a[1]) return b[1] - a[1];
        return a[0].localeCompare(b[0]);
      })
      .map(([page, score]) => ({ page, score }));
  }

  function safeImageExtension(mimeType) {
    switch (mimeType) {
      case "image/png":
        return ".png";
      case "image/jpeg":
        return ".jpg";
      case "image/gif":
        return ".gif";
      case "image/webp":
        return ".webp";
      default:
        return null;
    }
  }

  function serveIndex(c) {
    const indexPath = path.join(publicDir, "index.html");

    if (!fs.existsSync(indexPath)) {
      return c.text("index.html not found", 500);
    }

    const html = fs.readFileSync(indexPath, "utf8");
    return c.html(html);
  }

  ensureDir(pagesDir);
  ensureDir(publicDir);
  ensureDir(uploadsDir);

  app.use(
    "/static/*",
    serveStatic({
      root: publicDir,
      rewriteRequestPath: (requestPath) => requestPath.replace(/^\/static/, ""),
    }),
  );

  app.get("/api/pages", (c) => {
    return c.json({ pages: listMarkdownFiles(pagesDir) });
  });

  app.get("/api/page", (c) => {
    try {
      const name = normalizePageName(c.req.query("name") || "Home.md");
      const full = pagePath(name);

      if (!fs.existsSync(full)) {
        return c.json({ error: "page not found" }, 404);
      }

      const markdown = fs.readFileSync(full, "utf8");
      const graph = buildGraph();

      return c.json({
        name,
        markdown,
        html: marked.parse(markdown),
        backlinks: backlinksOf(name, graph),
        twoHop: twoHopOf(name, graph),
      });
    } catch (err) {
      return c.json({ error: err.message }, 400);
    }
  });

  app.post("/api/page", async (c) => {
    try {
      const body = await c.req.json();
      const name = normalizePageName(body.name);
      const markdown = String(body.markdown ?? "");
      const full = pagePath(name);

      if (fs.existsSync(full)) {
        return c.json({ error: "page already exists" }, 409);
      }

      fs.mkdirSync(path.dirname(full), { recursive: true });
      fs.writeFileSync(full, markdown, "utf8");

      return c.json({ ok: true, name });
    } catch (err) {
      return c.json({ error: err.message }, 400);
    }
  });

  app.put("/api/page", async (c) => {
    try {
      const body = await c.req.json();
      const name = normalizePageName(body.name);
      const markdown = String(body.markdown ?? "");
      const full = pagePath(name);

      fs.mkdirSync(path.dirname(full), { recursive: true });
      fs.writeFileSync(full, markdown, "utf8");

      const graph = buildGraph();

      return c.json({
        ok: true,
        name,
        html: marked.parse(markdown),
        backlinks: backlinksOf(name, graph),
        twoHop: twoHopOf(name, graph),
      });
    } catch (err) {
      return c.json({ error: err.message }, 400);
    }
  });

  app.post("/api/upload", async (c) => {
    try {
      const formData = await c.req.formData();
      const file = formData.get("file");

      if (!(file instanceof File)) {
        return c.json({ error: "file is required" }, 400);
      }

      const ext = safeImageExtension(file.type);
      if (!ext) {
        return c.json({ error: "unsupported image type" }, 400);
      }

      const bytes = await file.arrayBuffer();
      const buffer = Buffer.from(bytes);

      if (buffer.length > 10 * 1024 * 1024) {
        return c.json({ error: "file too large" }, 400);
      }

      const fileName = `${Date.now()}-${Math.random().toString(36).slice(2, 10)}${ext}`;
      const fullPath = path.join(uploadsDir, fileName);

      fs.writeFileSync(fullPath, buffer);

      return c.json({
        ok: true,
        url: `/static/uploads/${fileName}`,
      });
    } catch (err) {
      return c.json({ error: err.message }, 400);
    }
  });

  app.get("/", serveIndex);
  app.get("/*", serveIndex);

  return app;
}
