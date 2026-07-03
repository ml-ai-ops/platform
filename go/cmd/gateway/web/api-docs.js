const operations = document.querySelector("#operations");
const template = document.querySelector("#operation-template");
const search = document.querySelector("#search");
let entries = [];

const curlFor = (method, path) => {
  const target = `${location.origin}${path}`;
  return method === "GET" ? `curl '${target}'` : `curl -X ${method} -H 'Content-Type: application/json' -d '{}' '${target}'`;
};

function render(query = "") {
  const term = query.trim().toLowerCase();
  const matches = entries.filter(item => `${item.method} ${item.path} ${item.summary}`.toLowerCase().includes(term));
  operations.replaceChildren();
  if (!matches.length) {
    const empty = document.createElement("p");
    empty.className = "empty";
    empty.textContent = `No endpoints match “${query}”.`;
    operations.append(empty);
    return;
  }
  matches.forEach(item => {
    const fragment = template.content.cloneNode(true);
    const root = fragment.querySelector(".operation");
    const method = fragment.querySelector(".method");
    method.textContent = item.method;
    method.classList.add(item.method.toLowerCase());
    fragment.querySelector(".path").textContent = item.path;
    fragment.querySelector(".summary").textContent = item.summary;
    fragment.querySelector(".description").textContent = item.description || `${item.method} ${item.path}`;
    fragment.querySelector(".request-url").textContent = curlFor(item.method, item.path);
    fragment.querySelector(".copy").addEventListener("click", async event => {
      await navigator.clipboard.writeText(curlFor(item.method, item.path));
      event.currentTarget.textContent = "Copied";
      setTimeout(() => { event.currentTarget.textContent = "Copy cURL"; }, 1200);
    });
    operations.append(root);
  });
}

async function loadSpec() {
  try {
    const response = await fetch("/api/openapi.json", {headers: {Accept: "application/json"}});
    if (!response.ok) throw new Error(`OpenAPI request failed (${response.status})`);
    const spec = await response.json();
    entries = Object.entries(spec.paths || {}).flatMap(([path, methods]) =>
      Object.entries(methods).map(([method, operation]) => ({
        method: method.toUpperCase(),
        path,
        summary: operation.summary || "Untitled operation",
        description: operation.description || "",
      }))
    ).sort((a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method));
    document.querySelector("#api-version").textContent = `API ${spec.info?.version || "unversioned"}`;
    document.querySelector("#operation-count").textContent = `${entries.length} operations`;
    render();
  } catch (error) {
    document.querySelector("#api-error").textContent = error.message;
  }
}

search.addEventListener("input", event => render(event.target.value));
loadSpec();
