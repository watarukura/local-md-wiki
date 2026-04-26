import { EditorView, basicSetup } from "https://esm.sh/codemirror";
import { Compartment } from "https://esm.sh/@codemirror/state";
import { oneDark } from "https://esm.sh/@codemirror/theme-one-dark";
import { Vim, vim } from "https://esm.sh/@replit/codemirror-vim";
import { markdown } from "https://esm.sh/@codemirror/lang-markdown";
import hljs from "https://esm.sh/highlight.js/lib/core";
import diff from "https://esm.sh/highlight.js/lib/languages/diff";

hljs.registerLanguage("diff", diff);

const pageListEl = document.getElementById("page-list");
const pageNameEl = document.getElementById("page-name");
const viewerEl = document.getElementById("viewer");
const editorEl = document.getElementById("editor");
const backlinksEl = document.getElementById("backlinks");
const twoHopEl = document.getElementById("twohop");
const searchInputEl = document.getElementById("search-input");
const searchResultsEl = document.getElementById("search-results");
const gitLogEl = document.getElementById("git-log");
const gitDiffViewerEl = document.getElementById("git-diff-viewer");
const gitDiffContentEl = document.getElementById("git-diff-content");
const closeDiffButton = document.getElementById("close-diff-button");
const checkoutButton = document.getElementById("checkout-button");

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
    EditorView.domEventHandlers({
      paste: (e, view) => {
        const items = e.clipboardData?.items;
        if (!items) return;

        const imageItem = Array.from(items).find((item) => item.type.startsWith("image/"));
        if (!imageItem) return;

        const file = imageItem.getAsFile();
        if (!file) return;

        e.preventDefault();

        uploadImage(file)
          .then((url) => {
            insertTextAtCursor(view, `![pasted image](${url})`);
          })
          .catch((err) => {
            alert(err.message);
          });
        return true;
      },
    }),
    themeConfig.of(window.matchMedia("(prefers-color-scheme: dark)").matches ? oneDark : []),
  ],
  parent: editorEl,
});

window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (e) => {
  editorView.dispatch({
    effects: themeConfig.reconfigure(e.matches ? oneDark : []),
  });
});

// Enable clipboard support for yank/paste
Vim.defineOption("clipboard", "unnamed", "string", ["unnamed"], (value) => {
  if (value === "unnamed") {
    // Optional: add some logic here if needed
  }
});
Vim.setOption("clipboard", "unnamed");

// Bridge Vim's internal register with the system clipboard
if (navigator.clipboard) {
  try {
    const controller = Vim.getRegisterController();
    const unnamed = controller.unnamedRegister;
    if (unnamed && typeof unnamed.setText === "function") {
      const originalSetText = unnamed.setText;
      unnamed.setText = function (text, type) {
        originalSetText.call(this, text, type);
        if (text) {
          navigator.clipboard.writeText(text).catch((err) => {
            console.error("Failed to write to clipboard:", err);
          });
        }
      };
    }
  } catch (e) {
    console.warn("Vim.getRegisterController is not available", e);
  }
}

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

async function openPage(name, showConfirm = true) {
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

    fetchGitLog(currentPage);

    renderSideList(backlinksEl, data.backlinks, (page) => linkItem(page));
    renderSideList(twoHopEl, data.twoHop, (item) => linkItem(item.page, `shared: ${item.score}`));

    setEditing(false);
    // Close sidebars on navigation (useful for mobile)
    layoutEl.classList.remove("sidebar-open");
    layoutEl.classList.remove("meta-open");

    searchResultsEl.hidden = true;
    searchResultsEl.innerHTML = "";
    searchInputEl.value = "";

    const url = new URL(window.location.href);
    url.searchParams.set("page", currentPage);
    history.replaceState({}, "", url);
  } catch (err) {
    if (err.message.includes("page not found")) {
      if (!showConfirm) {
        viewerEl.innerHTML = `<div class="not-found">
          <p>"${name}" は存在しません。</p>
          <button class="btn btn-primary" onclick="createNewPage('${name}')">新しく作成する</button>
        </div>`;
        return;
      }
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
      fetchGitLog(currentPage);
      setEditing(true);

      const url = new URL(window.location.href);
      url.searchParams.set("page", currentPage);
      history.replaceState({}, "", url);
      return;
    }

    alert(err.message);
  }
}

window.createNewPage = async (name) => {
  const normalizedName = name.endsWith(".md") ? name : `${name}.md`;
  try {
    await createPage(normalizedName, `# ${normalizedName.replace(/\.md$/, "")}\n`);
    await openPage(normalizedName);
    setEditing(true);
  } catch (err) {
    alert(err.message);
  }
};

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

searchInputEl.addEventListener("input", async (e) => {
  const query = e.target.value.trim();
  if (!query) {
    searchResultsEl.hidden = true;
    searchResultsEl.innerHTML = "";
    return;
  }

  try {
    const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
    if (!res.ok) return;
    const results = await res.json();

    if (results.length === 0) {
      searchResultsEl.innerHTML = "<li>No results</li>";
    } else {
      searchResultsEl.innerHTML = "";
      const grouped = results.reduce((acc, curr) => {
        if (!acc[curr.file]) acc[curr.file] = [];
        acc[curr.file].push(curr);
        return acc;
      }, {});

      for (const [file, matches] of Object.entries(grouped)) {
        const li = document.createElement("li");
        const a = document.createElement("a");
        a.href = "#";
        a.className = "file-name";
        a.textContent = file;
        a.addEventListener("click", (e) => {
          e.preventDefault();
          openPage(file);
        });
        li.appendChild(a);

        matches.forEach((match) => {
          const span = document.createElement("span");
          span.className = "match-content";
          span.textContent = `L${match.line}: ${match.content}`;
          li.appendChild(span);
        });
        searchResultsEl.appendChild(li);
      }
    }
    searchResultsEl.hidden = false;
  } catch (err) {
    console.error("Search error:", err);
  }
});

let currentDiffHash = "";

async function fetchGitLog(file = "") {
  try {
    const res = await fetch(`/api/git/log${file ? `?file=${encodeURIComponent(file)}` : ""}`);
    const data = await res.json();
    gitLogEl.innerHTML = "";
    (data.commits || []).forEach((commit) => {
      const li = document.createElement("li");
      li.innerHTML = `
        <span class="commit-subject">${commit.subject}</span>
        <span class="commit-hash">${commit.hash.substring(0, 7)} - ${commit.author} - ${new Date(commit.date).toLocaleString()}</span>
      `;
      li.addEventListener("click", () => showDiff(commit.hash));
      gitLogEl.appendChild(li);
    });
  } catch (err) {
    console.error("Failed to fetch git log:", err);
  }
}

async function showDiff(hash) {
  try {
    const res = await fetch(`/api/git/diff?hash=${hash}&file=${encodeURIComponent(currentPage)}`);
    const data = await res.json();
    const highlighted = hljs.highlight(data.diff, { language: "diff" }).value;
    gitDiffContentEl.innerHTML = highlighted;
    gitDiffViewerEl.hidden = false;
    currentDiffHash = hash;
  } catch (err) {
    console.error("Failed to fetch git diff:", err);
  }
}

closeDiffButton.addEventListener("click", () => {
  gitDiffViewerEl.hidden = true;
});

checkoutButton.addEventListener("click", async () => {
  if (!confirm(`Are you sure you want to checkout commit ${currentDiffHash.substring(0, 7)}?`)) {
    return;
  }
  try {
    const res = await fetch("/api/git/checkout", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ hash: currentDiffHash }),
    });
    if (res.ok) {
      alert("Checked out successfully. Reloading page...");
      window.location.reload();
    } else {
      const data = await res.json();
      alert(`Failed to checkout: ${data.error}`);
    }
  } catch (err) {
    console.error("Failed to checkout commit:", err);
  }
});

(async () => {
  const data = await fetchPages();
  const pages = data.pages || [];
  const queryPage = new URL(window.location.href).searchParams.get("page");
  const initialPage = queryPage || (pages.length > 0 ? pages[0].name : "Home.md");

  fetchGitLog(initialPage);

  try {
    await refreshPageList(initialPage);
    // Only show confirm if page was explicitly requested via URL
    await openPage(initialPage, !!queryPage);
  } catch (err) {
    viewerEl.textContent = err.message;
  }
})();

window.addEventListener("keydown", (e) => {
  if (!viewerEl.hidden && (e.target === document.body || viewerEl.contains(e.target))) {
    if (e.key === "i" || e.key === "a" || e.key === "e") {
      e.preventDefault();
      setEditing(true);
    }
  }
});
