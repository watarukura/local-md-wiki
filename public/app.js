import { EditorView, basicSetup } from "https://esm.sh/codemirror";
import { Compartment } from "https://esm.sh/@codemirror/state";
import { oneDark } from "https://esm.sh/@codemirror/theme-one-dark";
import { Vim, vim } from "https://esm.sh/@replit/codemirror-vim";
import { markdown } from "https://esm.sh/@codemirror/lang-markdown";

const pageListEl = document.getElementById("page-list");
const pageNameEl = document.getElementById("page-name");
const viewerEl = document.getElementById("viewer");
const editorEl = document.getElementById("editor");
const backlinksEl = document.getElementById("backlinks");
const twoHopEl = document.getElementById("twohop");

const saveButton = document.getElementById("save-button");
const editButton = document.getElementById("edit-button");
const cancelButton = document.getElementById("cancel-button");
const newPageButton = document.getElementById("new-page-button");
const menuButton = document.getElementById("menu-button");
const metaButton = document.getElementById("meta-button");
const layoutEl = document.querySelector(".layout");

menuButton.addEventListener("click", () => {
  layoutEl.classList.remove("meta-open");
  layoutEl.classList.toggle("sidebar-open");
});

metaButton.addEventListener("click", () => {
  layoutEl.classList.remove("sidebar-open");
  layoutEl.classList.toggle("meta-open");
});

let currentPage = "Home.md";
let currentMarkdown = "";
let knownPages = new Set();

const themeConfig = new Compartment();

const editorView = new EditorView({
  doc: "",
  extensions: [
    basicSetup,
    vim(),
    markdown(),
    EditorView.lineWrapping,
    themeConfig.of(window.matchMedia("(prefers-color-scheme: dark)").matches ? oneDark : []),
  ],
  parent: editorEl,
});

window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (e) => {
  editorView.dispatch({
    effects: themeConfig.reconfigure(e.matches ? oneDark : []),
  });
});

Vim.defineEx("write", "w", () => {
  saveButton.click();
});
Vim.defineEx("quit", "q", () => {
  cancelButton.click();
});
Vim.defineEx("wq", "wq", () => {
  saveButton.click();
});
Vim.defineEx("x", "x", () => {
  saveButton.click();
});

function linkItem(page, extra = "") {
  const li = document.createElement("li");
  const a = document.createElement("a");
  a.href = `/?page=${encodeURIComponent(page)}`;
  a.dataset.page = page;
  a.textContent = extra ? `${page} (${extra})` : page;
  li.appendChild(a);
  return li;
}

function normalizeInternalLinkTarget(href, currentPage) {
  if (!href) return null;
  if (/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(href)) return null;
  if (href.startsWith("#")) return null;

  const hrefWithoutHash = href.split("#")[0];
  const hrefWithoutQuery = hrefWithoutHash.split("?")[0];
  if (!hrefWithoutQuery) return null;

  let decoded;
  try {
    decoded = decodeURIComponent(hrefWithoutQuery);
  } catch {
    decoded = hrefWithoutQuery;
  }

  const currentDir = currentPage.includes("/")
    ? currentPage.slice(0, currentPage.lastIndexOf("/"))
    : ".";

  const joined = currentDir === "." ? decoded : `${currentDir}/${decoded}`;

  const parts = [];
  for (const part of joined.split("/")) {
    if (!part || part === ".") continue;
    if (part === "..") {
      if (parts.length === 0) return null;
      parts.pop();
      continue;
    }
    parts.push(part);
  }

  let normalized = parts.join("/");
  if (!normalized.endsWith(".md")) {
    normalized += ".md";
  }

  return normalized;
}

function setEditing(editing) {
  viewerEl.hidden = editing;
  editorEl.hidden = !editing;
  saveButton.hidden = !editing;
  editButton.hidden = editing;
  cancelButton.hidden = !editing;
  pageNameEl.readOnly = !editing;

  // Hide menu buttons when editing to save space on mobile
  menuButton.hidden = editing;
  metaButton.hidden = editing;

  if (editing) {
    // Close sidebars when entering edit mode on mobile
    layoutEl.classList.remove("sidebar-open");
    layoutEl.classList.remove("meta-open");
    setTimeout(() => editorView.focus(), 0);
  }
}

async function fetchPages() {
  const res = await fetch("/api/pages");
  if (!res.ok) throw new Error("failed to load pages");
  return res.json();
}

async function fetchPage(name) {
  const res = await fetch(`/api/page?name=${encodeURIComponent(name)}`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "failed to load page");
  return data;
}

async function savePage(name, markdown) {
  const res = await fetch("/api/page", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      name,
      markdown,
    }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "failed to save");
  return data;
}

async function createPage(name, markdown = "") {
  const res = await fetch("/api/page", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      name,
      markdown,
    }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "failed to create");
  return data;
}

async function uploadImage(file) {
  const formData = new FormData();
  formData.append("file", file);

  const res = await fetch("/api/upload", {
    method: "POST",
    body: formData,
  });

  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "failed to upload image");
  return data.url;
}

function insertTextAtCursor(view, text) {
  const { state } = view;
  const range = state.selection.main;
  view.dispatch({
    changes: { from: range.from, to: range.to, insert: text },
    selection: { anchor: range.from + text.length },
    scrollIntoView: true,
  });
  view.focus();
}

function rewriteInternalLinks(container) {
  for (const a of container.querySelectorAll("a")) {
    const href = a.getAttribute("href");
    if (!href) continue;

    if (/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(href)) {
      a.target = "_blank";
      a.rel = "noreferrer";
      continue;
    }

    const target = normalizeInternalLinkTarget(href, currentPage);
    if (!target) continue;

    a.dataset.page = target;

    if (!knownPages.has(target)) {
      a.classList.add("missing-link");
      a.title = "Page does not exist yet";
    } else {
      a.classList.remove("missing-link");
      a.removeAttribute("title");
    }

    a.addEventListener("click", async (e) => {
      e.preventDefault();
      await openPage(target);
    });
  }
}

function renderSideList(el, items, mapper) {
  el.innerHTML = "";
  if (!items || !Array.isArray(items)) return;
  for (const item of items) {
    el.appendChild(mapper(item));
  }
}

async function refreshPageList(selectedPage = currentPage) {
  const data = await fetchPages();
  const pages = data.pages || [];
  knownPages = new Set(pages.map((p) => p.name));
  pageListEl.innerHTML = "";

  for (const page of pages) {
    const li = document.createElement("li");
    const a = document.createElement("a");
    a.href = `/?page=${encodeURIComponent(page.name)}`;
    a.dataset.page = page.name;
    a.textContent = page.title;
    if (page.name === selectedPage) a.classList.add("active");
    li.appendChild(a);
    pageListEl.appendChild(li);
  }
}

async function openPage(name) {
  try {
    const data = await fetchPage(name);

    currentPage = data.name;
    currentMarkdown = data.markdown;

    pageNameEl.value = data.name;
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: data.markdown },
    });

    let html = "";
    if (data.frontmatter?.title) {
      html += `<h1>${data.frontmatter.title}</h1>`;
    }
    if (
      data.frontmatter?.tags &&
      Array.isArray(data.frontmatter.tags) &&
      data.frontmatter.tags.length > 0
    ) {
      html += `<div class="tags">${data.frontmatter.tags.map((t) => `<span class="tag">#${t}</span>`).join(" ")}</div>`;
    }
    html += data.html;
    viewerEl.innerHTML = html;

    await refreshPageList(currentPage);
    rewriteInternalLinks(viewerEl);

    renderSideList(backlinksEl, data.backlinks, (page) => linkItem(page));
    renderSideList(twoHopEl, data.twoHop, (item) => linkItem(item.page, `shared: ${item.score}`));

    setEditing(false);
    // Close sidebars on navigation (useful for mobile)
    layoutEl.classList.remove("sidebar-open");
    layoutEl.classList.remove("meta-open");

    const url = new URL(window.location.href);
    url.searchParams.set("page", currentPage);
    history.replaceState({}, "", url);
  } catch (err) {
    if (err.message.includes("page not found")) {
      const ok = confirm(`"${name}" は存在しません。作成しますか？`);
      if (!ok) return;

      const normalizedName = name.endsWith(".md") ? name : `${name}.md`;
      await createPage(normalizedName, `# ${normalizedName.replace(/\.md$/, "")}\n`);

      currentPage = normalizedName;
      currentMarkdown = `# ${normalizedName.replace(/\.md$/, "")}\n`;

      pageNameEl.value = currentPage;
      editorView.dispatch({
        changes: { from: 0, to: editorView.state.doc.length, insert: currentMarkdown },
      });
      viewerEl.innerHTML = "";

      await refreshPageList(currentPage);
      setEditing(true);

      const url = new URL(window.location.href);
      url.searchParams.set("page", currentPage);
      history.replaceState({}, "", url);
      return;
    }

    alert(err.message);
  }
}

backlinksEl.addEventListener("click", async (e) => {
  const a = e.target.closest("a[data-page]");
  if (!a) return;
  e.preventDefault();
  await openPage(a.dataset.page);
});

twoHopEl.addEventListener("click", async (e) => {
  const a = e.target.closest("a[data-page]");
  if (!a) return;
  e.preventDefault();
  await openPage(a.dataset.page);
});

cancelButton.addEventListener("click", () => {
  editorView.dispatch({
    changes: { from: 0, to: editorView.state.doc.length, insert: currentMarkdown },
  });
  pageNameEl.value = currentPage;
  setEditing(false);
});

editButton.addEventListener("click", () => {
  setEditing(true);
});

saveButton.addEventListener("click", async () => {
  try {
    const name = pageNameEl.value.trim() || currentPage;
    const markdown = editorView.state.doc.toString();
    await savePage(name, markdown);
    await openPage(name);
  } catch (err) {
    alert(err.message);
  }
});

newPageButton.addEventListener("click", async () => {
  const name = prompt("New page name (.md optional)", "NewPage.md");
  if (!name) return;

  try {
    await createPage(name, `# ${name.replace(/\.md$/, "")}\n`);
    await openPage(name);
    setEditing(true);
  } catch (err) {
    alert(err.message);
  }
});

editorEl.addEventListener(
  "paste",
  async (e) => {
    const items = e.clipboardData?.items;
    if (!items) return;

    const imageItem = Array.from(items).find((item) => item.type.startsWith("image/"));
    if (!imageItem) return;

    const file = imageItem.getAsFile();
    if (!file) return;

    e.preventDefault();

    try {
      const url = await uploadImage(file);
      insertTextAtCursor(editorView, `![pasted image](${url})`);
    } catch (err) {
      alert(err.message);
    }
  },
  true,
);

const initialPage = new URL(window.location.href).searchParams.get("page") || "Home.md";

window.addEventListener("keydown", (e) => {
  if (!viewerEl.hidden && (e.target === document.body || viewerEl.contains(e.target))) {
    if (e.key === "i" || e.key === "a" || e.key === "e") {
      e.preventDefault();
      setEditing(true);
    }
  }
});

refreshPageList(initialPage)
  .then(() => openPage(initialPage))
  .catch((err) => {
    viewerEl.textContent = err.message;
  });
