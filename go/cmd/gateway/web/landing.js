const host = location.hostname;
document.querySelectorAll(".workbench-link").forEach(link => { link.href = `http://${host}:8888`; });
document.querySelectorAll(".ide-link").forEach(link => { link.href = `http://${host}:13337`; });

fetch("/api/v1/health", {headers:{"Accept":"application/json"}})
  .then(response => response.ok ? response.json() : Promise.reject())
  .then(() => {
    document.querySelector("#health-label").textContent = "Your control plane is ready";
    document.querySelector("#health-dot").classList.add("online");
  })
  .catch(() => {
    document.querySelector("#health-label").textContent = "Self-hosted platform";
  });
