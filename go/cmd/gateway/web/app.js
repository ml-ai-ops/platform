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
  const labels = {overview:"Good morning, builder.", projects:"Build with a clear starting point.", pipelines:"Every run, made legible.", models:"Promote with confidence.", agents:"Understand every agent turn.", catalog:"Reuse what your team knows.", platform:"Connect the production pieces."};
  document.querySelector("#page-title").textContent = labels[id];
}

async function loadDashboard() {
  const data = await api("/api/v1/dashboard");
  document.querySelector("#stat-projects").textContent = data.projects;
  document.querySelector("#stat-runs").textContent = data.active_runs;
  document.querySelector("#stat-health").textContent = `${data.healthy_components}/${data.total_components}`;
  document.querySelector("#progress-ring").style.background = `conic-gradient(#0071e3 ${data.onboarding_percent}%,rgba(118,118,128,.15) 0)`;
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
  document.querySelector("#run-table").innerHTML = runs.map(run => `<tr class="clickable" data-run-id="${escapeHTML(run.id)}"><td><b>${escapeHTML(run.name)}</b><br><small>${escapeHTML(run.id)}</small></td><td>${escapeHTML(run.project_id)}</td><td>${status(run.status)}</td><td><div class="bar"><i style="width:${Number(run.progress)}%"></i></div></td><td>${when(run.created_at)}</td></tr>`).join("");
}

async function showRun(runId) {
  const run = await api(`/api/v1/pipelines/runs/${encodeURIComponent(runId)}`);
  const steps = (run.steps || []).map((step, index) => `<div class="dag-step"><span>${index + 1}</span><div><b>${escapeHTML(step.name)}</b><small>${escapeHTML(step.image)}</small></div>${status(step.status)}</div>`).join("");
  const logs = (run.logs || []).map(log => `<div class="log-line"><time>${new Date(log.timestamp).toLocaleTimeString()}</time><b>${escapeHTML(log.step || "system")}</b><span>${escapeHTML(log.message)}</span></div>`).join("") || `<p class="empty">No logs have arrived yet.</p>`;
  document.querySelector("#run-detail").innerHTML = `<p class="eyebrow">PIPELINE RUN</p><h2>${escapeHTML(run.name)}</h2><div class="detail-meta">${status(run.status)}<span>${escapeHTML(run.id)}</span><span>${when(run.created_at)}</span></div><h3>Execution graph</h3><div class="dag">${steps}</div><h3>Logs</h3><div class="logs">${logs}</div><div class="sheet-actions"><button data-run-action="cancel" data-run-id="${escapeHTML(run.id)}">Cancel</button><button class="primary" data-run-action="retry" data-run-id="${escapeHTML(run.id)}">Retry run</button></div>`;
  document.querySelector("#run-dialog").showModal();
}

async function loadModels() {
  const data = await api("/api/v1/models");
  document.querySelector("#model-grid").innerHTML = data.items.length ? data.items.map(model => `<article class="card model-card"><span class="kind">${escapeHTML(model.stage)} · v${escapeHTML(model.version)}</span><h3>${escapeHTML(model.name)}</h3><p>${escapeHTML(model.artifact_uri)}</p><div class="metric-row"><span>Quality gate <b class="${model.gate_status === "passed" ? "good" : "bad"}">${escapeHTML(model.gate_status || "pending")}</b></span><span>Deployment <b>${escapeHTML(model.deployment_status || "not deployed")}</b></span></div><div class="tags">${Object.entries(model.metrics || {}).map(([key,value]) => `<span class="tag">${escapeHTML(key)} ${Number(value).toFixed(3)}</span>`).join("")}</div><footer><button data-model-action="promote" data-model-id="${escapeHTML(model.id)}">Promote</button><button data-model-action="deploy" data-model-id="${escapeHTML(model.id)}">Canary 10%</button><button data-model-action="rollback" data-model-id="${escapeHTML(model.id)}">Rollback</button></footer></article>`).join("") : `<p class="empty">No models registered yet.</p>`;
}

async function loadAgents() {
  const data = await api("/api/v1/agents");
  const sessionGroups = await Promise.all(data.items.map(agent => api(`/api/v1/agents/${encodeURIComponent(agent.id)}/sessions`)));
  const sessions = sessionGroups.flatMap(group => group.items);
  const tokens = sessions.reduce((sum, item) => sum + item.input_tokens + item.output_tokens, 0);
  const cost = sessions.reduce((sum, item) => sum + item.cost_usd, 0);
  document.querySelector("#agent-summary").innerHTML = `<article><span>Deployed agents</span><strong>${data.total}</strong><small>registered versions</small></article><article><span>Active sessions</span><strong>${sessions.filter(item => item.status === "running").length}</strong><small>${sessions.length} total sessions</small></article><article><span>Usage</span><strong>${tokens.toLocaleString()}</strong><small>$${cost.toFixed(4)} estimated cost</small></article>`;
  document.querySelector("#agent-grid").innerHTML = data.items.length ? data.items.map(agent => `<article class="card"><span class="kind">${escapeHTML(agent.llm_backend)} · v${escapeHTML(agent.version)}</span><h3>${escapeHTML(agent.name)}</h3><p>${escapeHTML(agent.graph_module)}</p><div class="tags">${(agent.tools || []).map(tool => `<span class="tag">${escapeHTML(tool)}</span>`).join("")}</div><footer>${status(agent.status)}<span class="tag">${agent.replicas} replicas · ${agent.canary_weight}% canary</span></footer></article>`).join("") : `<p class="empty">No agents deployed yet.</p>`;
  document.querySelector("#session-table").innerHTML = sessions.map(session => `<tr><td>${escapeHTML(session.id)}</td><td>${escapeHTML(session.agent_id)}</td><td>${escapeHTML(session.current_node)}</td><td>${status(session.status)}</td><td>${session.turns}</td><td>${(session.input_tokens + session.output_tokens).toLocaleString()}</td><td>$${session.cost_usd.toFixed(4)}</td></tr>`).join("");
}

async function loadCatalog(kind = "") {
  const items = await api(`/api/v1/catalog${kind ? `?kind=${encodeURIComponent(kind)}` : ""}`);
  document.querySelector("#catalog-grid").innerHTML = items.map(item => `<article class="card"><span class="kind">${escapeHTML(item.kind)} · v${escapeHTML(item.version)}</span><h3>${escapeHTML(item.name)}</h3><div class="tags">${item.metadata.map(meta => `<span class="tag">${escapeHTML(meta)}</span>`).join("")}</div><footer><span></span>${status(item.status)}</footer></article>`).join("");
}

async function loadComponents() {
  const [items, readiness, connections] = await Promise.all([api("/api/v1/components"), api("/api/v1/onboarding/readiness"), api("/api/v1/connections")]);
  document.querySelector("#component-grid").innerHTML = items.map(item => `<article class="component"><div><span class="category">${escapeHTML(item.category)}</span><h3>${escapeHTML(item.name)}</h3></div>${status(item.status)}<p>${escapeHTML(item.description)}</p></article>`).join("");
  document.querySelector("#readiness-percent").textContent = `${readiness.percent}%`;
  document.querySelector("#readiness-list").innerHTML = readiness.items.map(item => `<li class="${item.status === "ready" ? "done" : ""}"><span>${item.status === "ready" ? "✓" : "○"}</span><div><b>${escapeHTML(item.label)}</b><small>${escapeHTML(item.description)}</small></div></li>`).join("");
  document.querySelector("#connection-grid").innerHTML = connections.items.length ? connections.items.map(item => `<article class="card connection-card"><span class="kind">${escapeHTML(item.type)}</span><h3>${escapeHTML(item.name)}</h3><p>${escapeHTML(item.endpoint)}</p><footer>${status(item.status)}<button data-connection-test="${escapeHTML(item.id)}">Test</button></footer>${item.message ? `<small>${escapeHTML(item.message)}</small>` : ""}</article>`).join("") : `<div class="empty-state"><b>No services connected</b><span>Add Kubernetes, MLflow, storage and Kafka to complete onboarding.</span></div>`;
}

document.querySelectorAll(".nav-item").forEach(button => button.addEventListener("click", () => showView(button.dataset.view)));
document.querySelectorAll("[data-view-target]").forEach(button => button.addEventListener("click", () => showView(button.dataset.viewTarget)));
document.querySelectorAll("[data-kind]").forEach(button => button.addEventListener("click", () => { document.querySelectorAll("[data-kind]").forEach(n => n.classList.remove("active")); button.classList.add("active"); loadCatalog(button.dataset.kind); }));
const dialog = document.querySelector("#project-dialog");
document.querySelector("#new-project").addEventListener("click", () => dialog.showModal());
document.querySelectorAll("dialog .close").forEach(button => button.addEventListener("click", () => button.closest("dialog").close()));
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

document.querySelector("#add-connection").addEventListener("click", () => document.querySelector("#connection-dialog").showModal());
document.querySelector("#connection-form").addEventListener("submit", async event => {
  event.preventDefault(); const error = document.querySelector("#connection-error"); error.textContent = "";
  try {
    const connection = await api("/api/v1/connections", {method:"POST", body:JSON.stringify(Object.fromEntries(new FormData(event.target)))});
    await api(`/api/v1/connections/${encodeURIComponent(connection.id)}/test`, {method:"POST", body:"{}"});
    event.target.reset(); document.querySelector("#connection-dialog").close(); toast("Connection saved and checked."); await Promise.all([loadDashboard(), loadComponents()]);
  } catch (failure) { error.textContent = failure.message; }
});
document.addEventListener("click", async event => {
  const runRow = event.target.closest("[data-run-id]");
  if (runRow && !runRow.dataset.runAction) { await showRun(runRow.dataset.runId); return; }
  const runAction = event.target.closest("[data-run-action]");
  if (runAction) { await api(`/api/v1/pipelines/runs/${encodeURIComponent(runAction.dataset.runId)}/${runAction.dataset.runAction}`, {method:"POST", body:"{}"}); document.querySelector("#run-dialog").close(); toast(`Run ${runAction.dataset.runAction} requested.`); await Promise.all([loadRuns(),loadDashboard()]); return; }
  const modelAction = event.target.closest("[data-model-action]");
  if (modelAction) {
    const action = modelAction.dataset.modelAction; const id = encodeURIComponent(modelAction.dataset.modelId);
    if (action === "promote") await api(`/api/v1/models/${id}/promote`, {method:"POST", body:JSON.stringify({stage:"production"})});
    if (action === "deploy") await api(`/api/v1/models/${id}/deploy`, {method:"POST", body:JSON.stringify({canary_weight:10})});
    if (action === "rollback") await api(`/api/v1/models/${id}/rollback`, {method:"POST", body:"{}"});
    toast(`Model ${action} requested.`); await loadModels(); return;
  }
  const connectionTest = event.target.closest("[data-connection-test]");
  if (connectionTest) { await api(`/api/v1/connections/${encodeURIComponent(connectionTest.dataset.connectionTest)}/test`, {method:"POST", body:"{}"}); toast("Connection check completed."); await loadComponents(); }
});

Promise.all([loadDashboard(), loadProjects(), loadRuns(), loadModels(), loadAgents(), loadCatalog(), loadComponents()]).catch(error => toast(error.message));
