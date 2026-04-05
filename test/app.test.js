import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createApp } from "../app.js";

function makeTempDir(prefix) {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

function writeFile(baseDir, relativePath, content) {
  const full = path.join(baseDir, relativePath);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, content, "utf8");
}

async function readJson(res) {
  return await res.json();
}

test("GET /api/pages returns markdown files only", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");

  writeFile(pagesDir, "Home.md", "# Home");
  writeFile(pagesDir, "AWS.md", "# AWS");
  writeFile(pagesDir, "ignore.txt", "ignore");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/pages");

  assert.equal(res.status, 200);
  const body = await readJson(res);
  assert.deepEqual(body.pages, ["AWS.md", "Home.md"]);
});

test("GET /api/page returns page content, backlinks, and 2-hop links", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");

  writeFile(
    pagesDir,
    "Home.md",
    "# Home\n\n- [AWS](AWS.md)\n- [ECS](ECS.md)\n"
  );
  writeFile(
    pagesDir,
    "AWS.md",
    "# AWS\n\n- [Home](Home.md)\n- [ECS](ECS.md)\n"
  );
  writeFile(
    pagesDir,
    "Notes.md",
    "# Notes\n\n- [AWS](AWS.md)\n- [Other](Other.md)\n"
  );
  writeFile(
    pagesDir,
    "ECS.md",
    "# ECS\n\n- [Home](Home.md)\n"
  );
  writeFile(
    pagesDir,
    "Other.md",
    "# Other\n"
  );

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/page?name=Home.md");

  assert.equal(res.status, 200);
  const body = await readJson(res);

  assert.equal(body.name, "Home.md");
  assert.match(body.markdown, /# Home/);
  assert.match(body.html, /Home/);
  assert.deepEqual(body.backlinks, ["AWS.md", "ECS.md"]);
  assert.deepEqual(body.twoHop, [
    { page: "AWS.md", score: 1 },
    { page: "Notes.md", score: 1 }
  ]);
});

test("GET /api/page returns 404 for missing page", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/page?name=Missing.md");

  assert.equal(res.status, 404);
  const body = await readJson(res);
  assert.equal(body.error, "page not found");
});

test("GET /api/page rejects invalid traversal", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/page?name=../secret.md");

  assert.equal(res.status, 400);
  const body = await readJson(res);
  assert.match(body.error, /invalid page/i);
});

test("POST /api/page creates a new page", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/page", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      name: "NewPage.md",
      markdown: "# NewPage\n"
    })
  });

  assert.equal(res.status, 200);
  const body = await readJson(res);

  assert.equal(body.ok, true);
  assert.equal(body.name, "NewPage.md");

  const saved = fs.readFileSync(path.join(pagesDir, "NewPage.md"), "utf8");
  assert.equal(saved, "# NewPage\n");
});

test("POST /api/page returns 409 when page already exists", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");
  writeFile(pagesDir, "Home.md", "# Home\n");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/page", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      name: "Home.md",
      markdown: "# Changed\n"
    })
  });

  assert.equal(res.status, 409);
  const body = await readJson(res);
  assert.equal(body.error, "page already exists");
});

test("PUT /api/page updates a page and recalculates backlinks", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");

  writeFile(pagesDir, "Home.md", "# Home\n");
  writeFile(pagesDir, "AWS.md", "# AWS\n- [Home](Home.md)\n");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/page", {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      name: "Home.md",
      markdown: "# Home\n\n- [AWS](AWS.md)\n"
    })
  });

  assert.equal(res.status, 200);
  const body = await readJson(res);

  assert.equal(body.ok, true);
  assert.equal(body.name, "Home.md");
  assert.deepEqual(body.backlinks, ["AWS.md"]);
  assert.deepEqual(body.twoHop, []);

  const saved = fs.readFileSync(path.join(pagesDir, "Home.md"), "utf8");
  assert.equal(saved, "# Home\n\n- [AWS](AWS.md)\n");
});

test("PUT /api/page normalizes name without .md", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/page", {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      name: "Daily/2026-04-05",
      markdown: "# Daily\n"
    })
  });

  assert.equal(res.status, 200);
  const body = await readJson(res);
  assert.equal(body.name, "Daily/2026-04-05.md");

  const full = path.join(pagesDir, "Daily", "2026-04-05.md");
  assert.equal(fs.existsSync(full), true);
});

test("graph ignores external links and anchors", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>ok</body></html>");

  writeFile(
    pagesDir,
    "Home.md",
    "# Home\n\n- [Google](https://google.com)\n- [Section](#section)\n- [AWS](AWS.md)\n"
  );
  writeFile(pagesDir, "AWS.md", "# AWS\n");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/api/page?name=Home.md");

  assert.equal(res.status, 200);
  const body = await readJson(res);
  assert.deepEqual(body.backlinks, []);
  assert.deepEqual(body.twoHop, []);
});

test("GET * serves index.html", async () => {
  const pagesDir = makeTempDir("wiki-pages-");
  const publicDir = makeTempDir("wiki-public-");
  writeFile(publicDir, "index.html", "<!doctype html><html><body>INDEX</body></html>");

  const app = createApp({ pagesDir, publicDir });
  const res = await app.request("/some/path");

  assert.equal(res.status, 200);
  const text = await res.text();
  assert.match(text, /INDEX/);
});
