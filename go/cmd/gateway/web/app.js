const api = async (path, options = {}) => {
  const response = await fetch(path, {headers: {"Content-Type": "application/json"}, ...options});
  const body = await response.json();
  if (!response.ok) throw new Error(body.message || "Something went wrong");
  return body;
};

const escapeHTML = value => String(value ?? "").replace(/[&<>"']/g, char => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[char]));
const when = value => new Intl.RelativeTimeFormat("en", {numeric: "auto"}).format(Math.round((new Date(value) - Date.now()) / 60000), "minute");
const status = value => `<span class="status ${escapeHTML(value)}">${escapeHTML(value.replace("_", " "))}</span>`;
const toast = message => { const node = document.querySelector("#toast"); node.textContent = message; node.classList.add("show"); setTimeout(() => node.classList.remove("show"), 2400); };

function showView(id) {
  document.querySelectorAll(".view").forEach(node => node.classList.toggle("active", node.id === id));
  document.querySelectorAll(".nav-item").forEach(node => node.classList.toggle("active", node.dataset.view === id));
  const labels = {overview:"Good morning, builder.", projects:"Build with a clear starting point.", pipelines:"Every run, made legible.", catalog:"Reuse what your team knows.", platform:"Connect the production pieces."};
  document.querySelector("#page-title").textContent = labels[id];
}

async function loadDashboard() {
  const data = await api("/api/v1/dashboard");
  document.querySelector("#stat-projects").textContent = data.projects;
  document.querySelector("#stat-runs").textContent = data.active_runs;
  document.querySelector("#stat-health").textContent = `${data.healthy_components}/${data.total_components}`;
  document.querySelector("#progress-ring").style.background = `radial-gradient(circle closest-side,#eaf2e2 80%,transparent 81% 99%),conic-gradient(var(--green) ${data.onboarding_percent}%,#cdd9c6 0)`;
  document.querySelector("#progress-ring strong").textContent = `${data.onboarding_percent}%`;
  document.querySelector("#recent-runs").innerHTML = data.recent_runs.length ? data.recent_runs.map(run => `<div class="run-row"><span class="run-icon">↯</span><div><b>${escapeHTML(run.name)}</b><small>${when(run.created_at)}</small></div>${status(run.status)}</div>`).join("") : `<p class="empty">No runs yet.</p>`;
}

async function loadProjects() {
  const projects = await api("/api/v1/projects");
  document.querySelector("#project-grid").innerHTML = projects.map(project => `<article class="card"><span class="kind">${escapeHTML(project.template)}</span><h3>${escapeHTML(project.name)}</h3><p>${escapeHTML(project.description || "No description yet.")}</p><footer><span class="tag">${escapeHTML(project.namespace)}</span>${status(project.status)}</footer></article>`).join("");
  return projects;
}

async function loadRuns() {
  const runs = await api("/api/v1/pipelines/runs");
  document.querySelector("#run-table").innerHTML = runs.map(run => `<tr><td><b>${escapeHTML(run.name)}</b><br><small>${escapeHTML(run.id)}</small></td><td>${escapeHTML(run.project_id)}</td><td>${status(run.status)}</td><td><div class="bar"><i style="width:${Number(run.progress)}%"></i></div></td><td>${when(run.created_at)}</td></tr>`).join("");
}

async function loadCatalog(kind = "") {
  const items = await api(`/api/v1/catalog${kind ? `?kind=${encodeURIComponent(kind)}` : ""}`);
  document.querySelector("#catalog-grid").innerHTML = items.map(item => `<article class="card"><span class="kind">${escapeHTML(item.kind)} · v${escapeHTML(item.version)}</span><h3>${escapeHTML(item.name)}</h3><div class="tags">${item.metadata.map(meta => `<span class="tag">${escapeHTML(meta)}</span>`).join("")}</div><footer><span></span>${status(item.status)}</footer></article>`).join("");
}

async function loadComponents() {
  const items = await api("/api/v1/components");
  document.querySelector("#component-grid").innerHTML = items.map(item => `<article class="component"><div><span class="category">${escapeHTML(item.category)}</span><h3>${escapeHTML(item.name)}</h3></div>${status(item.status)}<p>${escapeHTML(item.description)}</p></article>`).join("");
}

document.querySelectorAll(".nav-item").forEach(button => button.addEventListener("click", () => showView(button.dataset.view)));
document.querySelectorAll("[data-view-target]").forEach(button => button.addEventListener("click", () => showView(button.dataset.viewTarget)));
document.querySelectorAll("[data-kind]").forEach(button => button.addEventListener("click", () => { document.querySelectorAll("[data-kind]").forEach(n => n.classList.remove("active")); button.classList.add("active"); loadCatalog(button.dataset.kind); }));
const dialog = document.querySelector("#project-dialog");
document.querySelector("#new-project").addEventListener("click", () => dialog.showModal());
document.querySelector(".close").addEventListener("click", () => dialog.close());
document.querySelector("#project-form").addEventListener("submit", async event => {
  event.preventDefault();
  const form = new FormData(event.target);
  const error = document.querySelector("#form-error");
  error.textContent = "";
  try {
    await api("/api/v1/projects", {method:"POST", body:JSON.stringify(Object.fromEntries(form))});
    event.target.reset(); dialog.close(); toast("Project created. Your workspace is ready.");
    await Promise.all([loadDashboard(), loadProjects()]); showView("projects");
  } catch (failure) { error.textContent = failure.message; }
});

Promise.all([loadDashboard(), loadProjects(), loadRuns(), loadCatalog(), loadComponents()]).catch(error => toast(error.message));
