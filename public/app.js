const pageListEl = document.getElementById("page-list");
const pageNameEl = document.getElementById("page-name");
const viewerEl = document.getElementById("viewer");
const editorEl = document.getElementById("editor");
const backlinksEl = document.getElementById("backlinks");
const twoHopEl = document.getElementById("twohop");

const editButton = document.getElementById("edit-button");
const saveButton = document.getElementById("save-button");
const cancelButton = document.getElementById("cancel-button");
const newPageButton = document.getElementById("new-page-button");

let currentPage = "Home.md";
let currentMarkdown = "";

function linkItem(page, extra = "") {
  const li = document.createElement("li");
  const a = document.createElement("a");
  a.href = `/?page=${encodeURIComponent(page)}`;
  a.dataset.page = page;
  a.textContent = extra ? `${page} (${extra})` : page;
  li.appendChild(a);
  return li;
}

function setEditing(editing) {
  viewerEl.hidden = editing;
  editorEl.hidden = !editing;
  editButton.hidden = editing;
  saveButton.hidden = !editing;
  cancelButton.hidden = !editing;
  pageNameEl.readOnly = !editing;
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
    body: JSON.stringify({ name, markdown })
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "failed to save");
  return data;
}

async function createPage(name, markdown = "") {
  const res = await fetch("/api/page", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, markdown })
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "failed to create");
  return data;
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

    if (!href.endsWith(".md") && !href.includes(".md#")) continue;

    a.addEventListener("click", async (e) => {
      e.preventDefault();
      const target = href.split("#")[0];
      await openPage(target);
    });
  }
}

function renderSideList(el, items, mapper) {
  el.innerHTML = "";
  for (const item of items) {
    el.appendChild(mapper(item));
  }
}

async function refreshPageList(selectedPage = currentPage) {
  const data = await fetchPages();
  pageListEl.innerHTML = "";

  for (const page of data.pages) {
    const li = document.createElement("li");
    const a = document.createElement("a");
    a.href = `/?page=${encodeURIComponent(page)}`;
    a.dataset.page = page;
    a.textContent = page;
    if (page === selectedPage) a.classList.add("active");
    li.appendChild(a);
    pageListEl.appendChild(li);
  }
}

async function openPage(name) {
  const data = await fetchPage(name);

  currentPage = data.name;
  currentMarkdown = data.markdown;

  pageNameEl.value = data.name;
  editorEl.value = data.markdown;
  viewerEl.innerHTML = data.html;
  rewriteInternalLinks(viewerEl);

  renderSideList(backlinksEl, data.backlinks, (page) => linkItem(page));
  renderSideList(twoHopEl, data.twoHop, (item) => linkItem(item.page, `shared: ${item.score}`));

  setEditing(false);
  await refreshPageList(currentPage);

  const url = new URL(window.location.href);
  url.searchParams.set("page", currentPage);
  history.replaceState({}, "", url);
}

pageListEl.addEventListener("click", async (e) => {
  const a = e.target.closest("a[data-page]");
  if (!a) return;
  e.preventDefault();
  await openPage(a.dataset.page);
});

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

editButton.addEventListener("click", () => {
  editorEl.value = currentMarkdown;
  setEditing(true);
});

cancelButton.addEventListener("click", () => {
  editorEl.value = currentMarkdown;
  pageNameEl.value = currentPage;
  setEditing(false);
});

saveButton.addEventListener("click", async () => {
  try {
    const name = pageNameEl.value.trim() || currentPage;
    const markdown = editorEl.value;
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

const initialPage = new URL(window.location.href).searchParams.get("page") || "Home.md";
refreshPageList(initialPage)
  .then(() => openPage(initialPage))
  .catch((err) => {
    viewerEl.textContent = err.message;
  });
