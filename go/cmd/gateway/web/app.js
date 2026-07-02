const api = async (path, options = {}) => {
  const response = await fetch(path, {headers: {"Content-Type": "application/json"}, ...options});
  const body = await response.json();
  if (!response.ok) throw new Error(body.message || body.error || "Something went wrong");
  return body;
};

const escapeHTML = value => String(value ?? "").replace(/[&<>"']/g, char => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[char]));
const when = value => new Intl.RelativeTimeFormat("en", {numeric: "auto"}).format(Math.round((new Date(value) - Date.now()) / 60000), "minute");
const status = value => `<span class="status ${escapeHTML(value)}">${escapeHTML(String(value).replace("_", " "))}</span>`;
const toast = message => { const node = document.querySelector("#toast"); node.textContent = message; node.classList.add("show"); setTimeout(() => node.classList.remove("show"), 2400); };
const bytes = size => size > 1048576 ? `${(size/1048576).toFixed(1)} MB` : size > 1024 ? `${(size/1024).toFixed(1)} KB` : `${size} B`;

let activeView = "overview";
const viewLoaders = {};

function showView(id) {
  activeView = id;
  document.querySelectorAll(".view").forEach(node => node.classList.toggle("active", node.id === id));
  document.querySelectorAll(".nav-item").forEach(node => node.classList.toggle("active", node.dataset.view === id));
  const labels = {overview:"Good morning, builder.", projects:"Build with a clear starting point.", pipelines:"Every run, made legible.", models:"Promote with confidence.", agents:"Understand every agent turn.", features:"One source of truth for features.", storage:"Artifacts and live endpoints.", realtime:"Score events as they happen.", catalog:"Reuse what your team knows.", platform:"Connect the production pieces."};
  document.querySelector("#page-title").textContent = labels[id] || "Workspace";
  if (viewLoaders[id]) viewLoaders[id]().catch(error => toast(error.message));
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
  document.querySelector("#project-grid").innerHTML = projects.length ? projects.map(project => `<article class="card"><span class="kind">${escapeHTML(project.template)}</span><h3>${escapeHTML(project.name)}</h3><p>${escapeHTML(project.description || "No description yet.")}</p><footer><span class="tag">${escapeHTML(project.namespace)}</span>${status(project.status)}</footer></article>`).join("") : `<p class="empty">No projects yet — create one to begin.</p>`;
  const select = document.querySelector("#submit-project");
  select.innerHTML = projects.map(project => `<option value="${escapeHTML(project.id)}">${escapeHTML(project.name)}</option>`).join("");
  return projects;
}

async function loadRuns() {
  const runs = await api("/api/v1/pipelines/runs");
  document.querySelector("#run-table").innerHTML = runs.length ? runs.map(run => `<tr class="clickable" data-run-id="${escapeHTML(run.id)}"><td><b>${escapeHTML(run.name)}</b><br><small>${escapeHTML(run.id)}</small></td><td>${escapeHTML(run.project_id)}</td><td>${status(run.status)}</td><td><div class="bar"><i style="width:${Number(run.progress)}%"></i></div></td><td>${when(run.created_at)}</td></tr>`).join("") : `<tr><td colspan="5" class="empty">No runs yet — submit one.</td></tr>`;
}

// dagSVG lays out steps in dependency layers and draws the run DAG.
function dagSVG(steps) {
  if (!steps || !steps.length) return "";
  const depth = {};
  const layerOf = step => {
    if (depth[step.name] !== undefined) return depth[step.name];
    const parents = (step.depends_on || []).map(name => steps.find(item => item.name === name)).filter(Boolean);
    depth[step.name] = parents.length ? Math.max(...parents.map(layerOf)) + 1 : 0;
    return depth[step.name];
  };
  steps.forEach(layerOf);
  const layers = {};
  steps.forEach(step => { (layers[depth[step.name]] ||= []).push(step); });
  const colWidth = 190, rowHeight = 74, boxW = 160, boxH = 52;
  const columns = Object.keys(layers).length;
  const rows = Math.max(...Object.values(layers).map(list => list.length));
  const position = {};
  Object.entries(layers).forEach(([layer, list]) => list.forEach((step, index) => { position[step.name] = {x: layer * colWidth + 10, y: index * rowHeight + 12}; }));
  const colors = {succeeded:"#30d158", running:"#0a84ff", failed:"#ff453a", pending:"#8e8e93", skipped:"#8e8e93", cancelled:"#ff9f0a"};
  const edges = steps.flatMap(step => (step.depends_on || []).map(parent => {
    const from = position[parent], to = position[step.name];
    if (!from || !to) return "";
    return `<path d="M ${from.x + boxW} ${from.y + boxH/2} C ${from.x + boxW + 24} ${from.y + boxH/2}, ${to.x - 24} ${to.y + boxH/2}, ${to.x} ${to.y + boxH/2}" class="dag-edge"/>`;
  })).join("");
  const nodes = steps.map(step => {
    const at = position[step.name];
    return `<g transform="translate(${at.x},${at.y})"><rect width="${boxW}" height="${boxH}" rx="10" class="dag-node"/><circle cx="16" cy="${boxH/2}" r="5" fill="${colors[step.status] || "#8e8e93"}"/><text x="30" y="${boxH/2 - 2}" class="dag-name">${escapeHTML(step.name)}</text><text x="30" y="${boxH/2 + 14}" class="dag-status">${escapeHTML(step.status)}</text></g>`;
  }).join("");
  return `<svg class="dag-svg" viewBox="0 0 ${columns * colWidth + 20} ${rows * rowHeight + 24}" role="img" aria-label="Pipeline DAG">${edges}${nodes}</svg>`;
}

async function showRun(runId) {
  const run = await api(`/api/v1/pipelines/runs/${encodeURIComponent(runId)}`);
  const logs = (run.logs || []).map(log => `<div class="log-line"><time>${new Date(log.timestamp).toLocaleTimeString()}</time><b>${escapeHTML(log.step || "system")}</b><span>${escapeHTML(log.message)}</span></div>`).join("") || `<p class="empty">No logs have arrived yet.</p>`;
  const engine = run.engine_run_id ? `<span class="tag">engine ${escapeHTML(run.engine_run_id)}</span>` : "";
  document.querySelector("#run-detail").innerHTML = `<p class="eyebrow">PIPELINE RUN</p><h2>${escapeHTML(run.name)}</h2><div class="detail-meta">${status(run.status)}<span>${escapeHTML(run.id)}</span><span>${when(run.created_at)}</span>${engine}</div><h3>Execution graph</h3>${dagSVG(run.steps)}<h3>Logs</h3><div class="logs">${logs}</div><div class="sheet-actions"><button data-run-action="cancel" data-run-id="${escapeHTML(run.id)}">Cancel</button><button class="primary" data-run-action="retry" data-run-id="${escapeHTML(run.id)}">Retry run</button></div>`;
  document.querySelector("#run-dialog").showModal();
}

// metricChart draws a per-version bar chart for the selected metric.
function metricChart(models, metric) {
  const canvas = document.querySelector("#metric-chart");
  const context = canvas.getContext("2d");
  const points = models.filter(model => model.metrics && model.metrics[metric] !== undefined)
    .map(model => ({label: `${model.name} v${model.version}`, value: Number(model.metrics[metric])}));
  canvas.width = canvas.clientWidth * devicePixelRatio;
  canvas.height = 160 * devicePixelRatio;
  context.scale(devicePixelRatio, devicePixelRatio);
  context.clearRect(0, 0, canvas.clientWidth, 160);
  if (!points.length) { context.fillStyle = "#8e8e93"; context.font = "13px -apple-system, sans-serif"; context.fillText("No metric data yet — run a training pipeline.", 12, 80); return; }
  const max = Math.max(...points.map(point => point.value), 1);
  const barWidth = Math.min(72, (canvas.clientWidth - 24) / points.length - 16);
  points.forEach((point, index) => {
    const x = 12 + index * (barWidth + 16);
    const height = (point.value / max) * 110;
    context.fillStyle = "#0071e3";
    context.beginPath(); context.roundRect(x, 128 - height, barWidth, height, 6); context.fill();
    context.fillStyle = "#1d1d1f"; context.font = "11px -apple-system, sans-serif";
    context.fillText(point.value.toFixed(3), x, 122 - height);
    context.fillStyle = "#8e8e93";
    context.fillText(point.label.slice(0, Math.ceil(barWidth / 6)), x, 148);
  });
}

let cachedModels = [];
async function loadModels() {
  const data = await api("/api/v1/models");
  data.items = data.items || [];
  cachedModels = data.items;
  const metricNames = [...new Set(data.items.flatMap(model => Object.keys(model.metrics || {})))];
  const select = document.querySelector("#metric-select");
  const chosen = select.value || metricNames[0] || "";
  select.innerHTML = metricNames.map(name => `<option ${name === chosen ? "selected" : ""}>${escapeHTML(name)}</option>`).join("");
  metricChart(data.items, chosen);
  document.querySelector("#model-grid").innerHTML = data.items.length ? data.items.map(model => {
    const live = model.endpoint_url && model.endpoint_url.startsWith("http");
    return `<article class="card model-card"><span class="kind">${escapeHTML(model.stage)} · v${escapeHTML(model.version)}</span><h3>${escapeHTML(model.name)}</h3><p>${escapeHTML(model.artifact_uri)}</p><div class="metric-row"><span>Quality gate <b class="${model.gate_status === "passed" ? "good" : "bad"}">${escapeHTML(model.gate_status || "pending")}</b></span><span>Deployment <b>${escapeHTML(model.deployment_status || "not deployed")}</b></span></div><div class="tags">${Object.entries(model.metrics || {}).map(([key,value]) => `<span class="tag">${escapeHTML(key)} ${Number(value).toFixed(3)}</span>`).join("")}${live ? `<span class="tag live">● live</span>` : ""}</div><footer><button data-model-action="promote" data-model-id="${escapeHTML(model.id)}">Promote</button><button data-model-action="deploy" data-model-id="${escapeHTML(model.id)}">Deploy</button><button data-model-action="rollback" data-model-id="${escapeHTML(model.id)}">Rollback</button>${live ? `<button class="primary" data-model-test="${escapeHTML(model.id)}">Test</button>` : ""}</footer></article>`;
  }).join("") : `<p class="empty">No models registered yet — run the training pipeline.</p>`;
}

async function loadAgents() {
  const [data, prompts] = await Promise.all([api("/api/v1/agents"), api("/api/v1/prompts")]);
  const sessionGroups = await Promise.all(data.items.map(agent => api(`/api/v1/agents/${encodeURIComponent(agent.id)}/sessions`)));
  const sessions = sessionGroups.flatMap(group => group.items);
  const tokens = sessions.reduce((sum, item) => sum + item.input_tokens + item.output_tokens, 0);
  const cost = sessions.reduce((sum, item) => sum + item.cost_usd, 0);
  document.querySelector("#agent-summary").innerHTML = `<article><span>Deployed agents</span><strong>${data.total}</strong><small>registered versions</small></article><article><span>Active sessions</span><strong>${sessions.filter(item => item.status === "running").length}</strong><small>${sessions.length} total sessions</small></article><article><span>LLM cost</span><strong>$${cost.toFixed(4)}</strong><small>${tokens.toLocaleString()} tokens</small></article>`;
  document.querySelector("#agent-grid").innerHTML = data.items.length ? data.items.map(agent => `<article class="card"><span class="kind">${escapeHTML(agent.llm_backend)} · v${escapeHTML(agent.version)}</span><h3>${escapeHTML(agent.name)}</h3><p>${escapeHTML(agent.graph_module)}</p><div class="tags">${(agent.tools || []).map(tool => `<span class="tag">${escapeHTML(tool)}</span>`).join("")}</div><footer>${status(agent.status)}<span class="tag">${agent.canary_weight}% canary</span><button data-agent-traffic="${escapeHTML(agent.id)}">Traffic</button><button class="primary" data-agent-chat="${escapeHTML(agent.id)}" data-agent-name="${escapeHTML(agent.name)}">Chat</button></footer></article>`).join("") : `<p class="empty">No agents deployed yet.</p>`;
  document.querySelector("#session-table").innerHTML = sessions.length ? sessions.map(session => `<tr><td>${escapeHTML(session.id)}</td><td>${escapeHTML(session.agent_id)}</td><td>${escapeHTML(session.current_node)}</td><td>${status(session.status)}</td><td>${session.turns}</td><td>${(session.input_tokens + session.output_tokens).toLocaleString()}</td><td>$${session.cost_usd.toFixed(4)}</td></tr>`).join("") : `<tr><td colspan="7" class="empty">No sessions yet — chat with an agent.</td></tr>`;
  document.querySelector("#prompt-list").innerHTML = prompts.configured
    ? (prompts.items.length ? prompts.items.map(prompt => `<div class="prompt-row"><b>${escapeHTML(prompt.name)}</b><span class="tag">v${escapeHTML(prompt.version ?? "?")}</span>${(prompt.labels || []).map(label => `<span class="tag">${escapeHTML(label)}</span>`).join("")}</div>`).join("") : `<p class="empty">Langfuse is connected — no prompts stored yet.</p>`)
    : `<p class="empty">Langfuse not configured. Prompts appear here once connected.</p>`;
}

let featureCache = [];
function renderFeatures(query = "") {
  const lowered = query.toLowerCase();
  const filtered = featureCache.filter(view => !lowered || view.name.toLowerCase().includes(lowered) || (view.tags || []).some(tag => tag.toLowerCase().includes(lowered)));
  document.querySelector("#feature-grid").innerHTML = filtered.length ? filtered.map(view => `<article class="card feature-card"><span class="kind">entity: ${escapeHTML(view.entity)}</span><h3>${escapeHTML(view.name)}</h3><table class="schema"><thead><tr><th>Field</th><th>Type</th></tr></thead><tbody>${(view.fields || []).map(field => `<tr><td>${escapeHTML(field.name)}</td><td>${escapeHTML(field.type)}</td></tr>`).join("")}</tbody></table><div class="tags">${(view.tags || []).map(tag => `<span class="tag">${escapeHTML(tag)}</span>`).join("")}${view.ttl_seconds ? `<span class="tag">TTL ${view.ttl_seconds}s</span>` : ""}</div><footer>${status(view.status)}<span class="tag">${view.online_entity_count || 0} entities online</span>${view.materialized_at ? `<small>${when(view.materialized_at)}</small>` : ""}</footer></article>`).join("") : `<p class="empty">No feature views${query ? " match your search" : " applied yet — run the materializer"}.</p>`;
}
async function loadFeatures() {
  const data = await api("/api/v1/features");
  featureCache = data.items || [];
  renderFeatures(document.querySelector("#feature-search").value);
}

const storageState = {bucket: "", prefix: ""};
async function loadStorage() {
  const browser = document.querySelector("#storage-browser");
  const title = document.querySelector("#storage-title");
  const up = document.querySelector("#storage-up");
  try {
    if (!storageState.bucket) {
      const data = await api("/api/v1/storage/buckets");
      title.textContent = "Buckets";
      up.hidden = true;
      browser.innerHTML = (data.buckets || []).map(bucket => `<button class="storage-row" data-bucket="${escapeHTML(bucket.name)}"><span>🪣</span><b>${escapeHTML(bucket.name)}</b></button>`).join("") || `<p class="empty">No buckets found.</p>`;
    } else {
      const data = await api(`/api/v1/storage/objects?bucket=${encodeURIComponent(storageState.bucket)}&prefix=${encodeURIComponent(storageState.prefix)}`);
      title.textContent = `${storageState.bucket}/${storageState.prefix}`;
      up.hidden = false;
      const prefixes = (data.prefixes || []).map(prefix => `<button class="storage-row" data-prefix="${escapeHTML(prefix)}"><span>📁</span><b>${escapeHTML(prefix.slice(storageState.prefix.length))}</b></button>`).join("");
      const objects = (data.objects || []).map(object => `<button class="storage-row" data-object="${escapeHTML(object.key)}"><span>📄</span><b>${escapeHTML(object.key.slice(storageState.prefix.length))}</b><small>${bytes(object.size)}</small></button>`).join("");
      browser.innerHTML = prefixes + objects || `<p class="empty">Empty prefix.</p>`;
    }
  } catch (error) {
    browser.innerHTML = `<p class="empty">Object store unavailable: ${escapeHTML(error.message)}</p>`;
  }
  const [models, functions] = await Promise.all([api("/api/v1/models"), api("/api/v1/functions")]);
  const live = models.items.filter(model => model.endpoint_url && model.endpoint_url.startsWith("http"));
  document.querySelector("#endpoint-table").innerHTML = live.length ? live.map(model => `<tr><td><b>${escapeHTML(model.name)}</b> v${escapeHTML(model.version)}</td><td><code>${escapeHTML(model.endpoint_url)}</code></td><td>${status(model.deployment_status)}</td><td><button data-model-test="${escapeHTML(model.id)}">Test</button></td></tr>`).join("") : `<tr><td colspan="4" class="empty">No live endpoints — deploy a gated model.</td></tr>`;
  document.querySelector("#function-table").innerHTML = functions.configured
    ? ((functions.items || []).length ? functions.items.map(fn => `<tr><td><b>${escapeHTML(fn.name)}</b></td><td><code>${escapeHTML(fn.image)}</code></td><td>${fn.replicas}</td><td><button data-function-invoke="${escapeHTML(fn.name)}">Invoke</button></td></tr>`).join("") : `<tr><td colspan="4" class="empty">OpenFaaS connected — no functions yet.</td></tr>`)
    : `<tr><td colspan="4" class="empty">Serverless not configured (set OPENFAAS_URL).</td></tr>`;
}

async function loadRealtime() {
  const data = await api("/api/v1/realtime");
  const demos = [
    {key: "fraud", title: "Fraud detection", detail: "transaction → features → model score → alert"},
    {key: "callcenter", title: "Call-center analysis", detail: "transcript → support agent → sentiment + intent"},
    {key: "recommendations", title: "Recommendations", detail: "activity → profile features → ranked items"},
  ];
  document.querySelector("#realtime-grid").innerHTML = demos.map(demo => {
    const stats = (data.demos || {})[demo.key];
    const body = stats
      ? `<div class="metric-row"><span>Events <b>${stats.events ?? 0}</b></span><span>Avg latency <b>${stats.avg_latency_ms ?? 0} ms</b></span>${demo.key === "fraud" ? `<span>Flagged <b class="bad">${stats.flagged ?? 0}</b></span>` : ""}</div><small>updated ${when(stats.updated_at)}</small>`
      : `<p class="empty">No events processed yet.</p>`;
    return `<article class="card"><span class="kind">stream</span><h3>${demo.title}</h3><p>${demo.detail}</p>${body}</article>`;
  }).join("");
}

async function loadCatalog(kind = "") {
  const items = await api(`/api/v1/catalog${kind ? `?kind=${encodeURIComponent(kind)}` : ""}`);
  document.querySelector("#catalog-grid").innerHTML = items.length ? items.map(item => `<article class="card"><span class="kind">${escapeHTML(item.kind)} · ${escapeHTML(item.version)}</span><h3>${escapeHTML(item.name)}</h3><div class="tags">${item.metadata.map(meta => `<span class="tag">${escapeHTML(meta)}</span>`).join("")}</div><footer><span></span>${status(item.status)}</footer></article>`).join("") : `<p class="empty">The catalog fills up as you register models, features, agents and tools.</p>`;
}

async function loadComponents() {
  const [items, readiness, connections] = await Promise.all([api("/api/v1/components"), api("/api/v1/onboarding/readiness"), api("/api/v1/connections")]);
  document.querySelector("#component-grid").innerHTML = items.map(item => `<article class="component"><div><span class="category">${escapeHTML(item.category)}</span><h3>${escapeHTML(item.name)}</h3></div>${status(item.status)}<p>${escapeHTML(item.description)}</p></article>`).join("");
  document.querySelector("#readiness-percent").textContent = `${readiness.percent}%`;
  document.querySelector("#readiness-list").innerHTML = readiness.items.map(item => `<li class="${item.status === "ready" ? "done" : ""}"><span>${item.status === "ready" ? "✓" : "○"}</span><div><b>${escapeHTML(item.label)}</b><small>${escapeHTML(item.description)}</small></div></li>`).join("");
  document.querySelector("#connection-grid").innerHTML = connections.items.length ? connections.items.map(item => `<article class="card connection-card"><span class="kind">${escapeHTML(item.type)}</span><h3>${escapeHTML(item.name)}</h3><p>${escapeHTML(item.endpoint)}</p><footer>${status(item.status)}<button data-connection-test="${escapeHTML(item.id)}">Test</button></footer>${item.message ? `<small>${escapeHTML(item.message)}</small>` : ""}</article>`).join("") : `<div class="empty-state"><b>No services connected</b><span>Add Kubernetes, MLflow, storage and Kafka to complete onboarding.</span></div>`;
}

Object.assign(viewLoaders, {
  overview: loadDashboard, projects: loadProjects, pipelines: loadRuns, models: loadModels,
  agents: loadAgents, features: loadFeatures, storage: loadStorage, realtime: loadRealtime,
  catalog: () => loadCatalog(document.querySelector("[data-kind].active")?.dataset.kind || ""),
  platform: loadComponents,
});

// ---- live updates over SSE --------------------------------------------------
let lastDigest = "";
function connectEvents() {
  const source = new EventSource("/api/v1/events");
  const indicator = document.querySelector("#live-indicator");
  source.onmessage = event => {
    indicator.textContent = "●  Local Cluster  live";
    if (event.data === lastDigest) return;
    lastDigest = event.data;
    loadDashboard().catch(() => {});
    if (viewLoaders[activeView] && activeView !== "overview") viewLoaders[activeView]().catch(() => {});
  };
  source.onerror = () => { indicator.textContent = "○  Local Cluster  reconnecting"; };
}

// ---- interactions -----------------------------------------------------------
document.querySelectorAll(".nav-item").forEach(button => button.addEventListener("click", () => showView(button.dataset.view)));
document.querySelectorAll("[data-view-target]").forEach(button => button.addEventListener("click", () => showView(button.dataset.viewTarget)));
document.querySelectorAll("[data-kind]").forEach(button => button.addEventListener("click", () => { document.querySelectorAll("[data-kind]").forEach(n => n.classList.remove("active")); button.classList.add("active"); loadCatalog(button.dataset.kind); }));
document.querySelector("#feature-search").addEventListener("input", event => renderFeatures(event.target.value));
document.querySelector("#storage-up").addEventListener("click", () => {
  if (storageState.prefix) {
    const parts = storageState.prefix.replace(/\/$/, "").split("/");
    parts.pop();
    storageState.prefix = parts.length ? parts.join("/") + "/" : "";
  } else storageState.bucket = "";
  loadStorage().catch(error => toast(error.message));
});

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

document.querySelector("#run-pipeline").addEventListener("click", async () => { await loadProjects(); document.querySelector("#submit-dialog").showModal(); });
document.querySelector("#submit-form").addEventListener("submit", async event => {
  event.preventDefault(); const error = document.querySelector("#submit-error"); error.textContent = "";
  try {
    await api("/api/v1/pipelines/submit", {method:"POST", body:JSON.stringify(Object.fromEntries(new FormData(event.target)))});
    document.querySelector("#submit-dialog").close(); toast("Run submitted to the engine."); await loadRuns();
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

document.querySelector("#metric-select").addEventListener("change", event => metricChart(cachedModels, event.target.value));

// Agent chat console
const chatState = {agentId: "", sessionId: ""};
document.querySelector("#chat-form").addEventListener("submit", async event => {
  event.preventDefault();
  const input = document.querySelector("#chat-input");
  const log = document.querySelector("#chat-log");
  const message = input.value.trim();
  if (!message) return;
  input.value = "";
  if (log.querySelector(".empty")) log.innerHTML = "";
  log.insertAdjacentHTML("beforeend", `<div class="chat-turn user"><span>You</span><p>${escapeHTML(message)}</p></div>`);
  log.insertAdjacentHTML("beforeend", `<div class="chat-turn agent pending"><span>Agent</span><p>…</p></div>`);
  log.scrollTop = log.scrollHeight;
  try {
    const reply = await api(`/api/v1/agents/${encodeURIComponent(chatState.agentId)}/invoke`, {method:"POST", body:JSON.stringify({message, session_id: chatState.sessionId, user_id: "console"})});
    chatState.sessionId = reply.session_id;
    log.lastElementChild.outerHTML = `<div class="chat-turn agent"><span>Agent · ${reply.input_tokens + reply.output_tokens} tokens · ${reply.duration_ms}ms</span><p>${escapeHTML(reply.reply)}</p></div>`;
  } catch (failure) {
    log.lastElementChild.outerHTML = `<div class="chat-turn agent failed"><span>Agent</span><p>${escapeHTML(failure.message)}</p></div>`;
  }
  log.scrollTop = log.scrollHeight;
});

// Model predict console
const predictState = {modelId: ""};
document.querySelector("#predict-form").addEventListener("submit", async event => {
  event.preventDefault();
  const output = document.querySelector("#predict-output");
  output.textContent = "…";
  try {
    const body = document.querySelector("#predict-input").value;
    JSON.parse(body);
    const result = await api(`/api/v1/models/${encodeURIComponent(predictState.modelId)}/predict`, {method:"POST", body});
    output.textContent = JSON.stringify(result, null, 2);
  } catch (failure) { output.textContent = `Error: ${failure.message}`; }
});

document.addEventListener("click", async event => {
  const bucket = event.target.closest("[data-bucket]");
  if (bucket) { storageState.bucket = bucket.dataset.bucket; storageState.prefix = ""; await loadStorage(); return; }
  const prefix = event.target.closest("[data-prefix]");
  if (prefix) { storageState.prefix = prefix.dataset.prefix; await loadStorage(); return; }
  const object = event.target.closest("[data-object]");
  if (object) {
    const preview = await api(`/api/v1/storage/object?bucket=${encodeURIComponent(storageState.bucket)}&key=${encodeURIComponent(object.dataset.object)}`);
    document.querySelector("#preview-detail").innerHTML = `<p class="eyebrow">OBJECT PREVIEW</p><h2>${escapeHTML(preview.key)}</h2><div class="detail-meta"><span class="tag">${escapeHTML(preview.content_type || "unknown type")}</span><span class="tag">${bytes(preview.size)}</span>${preview.truncated ? `<span class="tag">truncated</span>` : ""}</div><pre class="hint object-preview">${escapeHTML(preview.content)}</pre>`;
    document.querySelector("#preview-dialog").showModal();
    return;
  }
  const chat = event.target.closest("[data-agent-chat]");
  if (chat) { chatState.agentId = chat.dataset.agentChat; chatState.sessionId = ""; document.querySelector("#chat-agent-name").textContent = chat.dataset.agentName; document.querySelector("#chat-log").innerHTML = `<p class="empty">Ask something — the turn runs through the real agent runtime.</p>`; document.querySelector("#chat-dialog").showModal(); return; }
  const traffic = event.target.closest("[data-agent-traffic]");
  if (traffic) {
    const weight = prompt("Canary weight (0-100):", "10");
    if (weight === null) return;
    await api(`/api/v1/agents/${encodeURIComponent(traffic.dataset.agentTraffic)}/traffic`, {method:"PUT", body:JSON.stringify({canary_weight: Number(weight)})});
    toast(`Canary weight set to ${weight}%.`); await loadAgents(); return;
  }
  const modelTest = event.target.closest("[data-model-test]");
  if (modelTest) { predictState.modelId = modelTest.dataset.modelTest; const model = cachedModels.find(item => item.id === modelTest.dataset.modelTest); document.querySelector("#predict-model-name").textContent = model ? `${model.name} v${model.version}` : "Model"; document.querySelector("#predict-output").textContent = ""; document.querySelector("#predict-dialog").showModal(); return; }
  const fnInvoke = event.target.closest("[data-function-invoke]");
  if (fnInvoke) {
    const payload = prompt("JSON payload:", "{}");
    if (payload === null) return;
    try { const result = await api(`/api/v1/functions/${encodeURIComponent(fnInvoke.dataset.functionInvoke)}/invoke`, {method:"POST", body: payload}); toast(`Function replied: ${JSON.stringify(result).slice(0, 120)}`); }
    catch (failure) { toast(failure.message); }
    return;
  }
  const runRow = event.target.closest("[data-run-id]");
  if (runRow && !runRow.dataset.runAction) { await showRun(runRow.dataset.runId); return; }
  const runAction = event.target.closest("[data-run-action]");
  if (runAction) { await api(`/api/v1/pipelines/runs/${encodeURIComponent(runAction.dataset.runId)}/${runAction.dataset.runAction}`, {method:"POST", body:"{}"}); document.querySelector("#run-dialog").close(); toast(`Run ${runAction.dataset.runAction} requested.`); await Promise.all([loadRuns(),loadDashboard()]); return; }
  const modelAction = event.target.closest("[data-model-action]");
  if (modelAction) {
    const action = modelAction.dataset.modelAction; const id = encodeURIComponent(modelAction.dataset.modelId);
    try {
      if (action === "promote") await api(`/api/v1/models/${id}/promote`, {method:"POST", body:JSON.stringify({stage:"production"})});
      if (action === "deploy") await api(`/api/v1/models/${id}/deploy`, {method:"POST", body:JSON.stringify({canary_weight:0})});
      if (action === "rollback") await api(`/api/v1/models/${id}/rollback`, {method:"POST", body:"{}"});
      toast(`Model ${action} requested.`);
    } catch (failure) { toast(failure.message); }
    await loadModels(); return;
  }
  const connectionTest = event.target.closest("[data-connection-test]");
  if (connectionTest) { await api(`/api/v1/connections/${encodeURIComponent(connectionTest.dataset.connectionTest)}/test`, {method:"POST", body:"{}"}); toast("Connection check completed."); await loadComponents(); }
});

Promise.all([loadDashboard(), loadProjects(), loadRuns()]).catch(error => toast(error.message));
connectEvents();
