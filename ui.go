package main

// ResourcePageHTML provides the self-contained HTML/CSS/JS dashboard
// served at /v0/resource/plugins/cpa-live-voice/status.
const ResourcePageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>CPA Live Voice Scheduler</title>
  <style>
    :root {
      --bg: #f8fafc;
      --surface: #ffffff;
      --surface-subtle: #f1f5f9;
      --border: #e2e8f0;
      --text: #0f172a;
      --text-muted: #64748b;
      --primary: #2563eb;
      --primary-hover: #1d4ed8;
      --success: #16a34a;
      --success-bg: #dcfce7;
      --warning: #d97706;
      --warning-bg: #fef3c7;
      --danger: #dc2626;
      --danger-bg: #fee2e2;
      --focus-ring: 0 0 0 2px #93c5fd;
      --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      --radius: 8px;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #0b0f19;
        --surface: #111827;
        --surface-subtle: #1f2937;
        --border: #374151;
        --text: #f9fafb;
        --text-muted: #9ca3af;
        --primary: #3b82f6;
        --primary-hover: #60a5fa;
        --success: #22c55e;
        --success-bg: #052e16;
        --warning: #f59e0b;
        --warning-bg: #451a03;
        --danger: #ef4444;
        --danger-bg: #450a0a;
        --focus-ring: 0 0 0 2px #1d4ed8;
      }
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background: var(--bg);
      color: var(--text);
      line-height: 1.5;
      padding: 1.5rem;
    }
    .container { max-width: 1100px; margin: 0 auto; display: flex; flex-direction: column; gap: 1.25rem; }
    header {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      align-items: center;
      gap: 1rem;
      padding-bottom: 1rem;
      border-bottom: 1px solid var(--border);
    }
    .header-title h1 { font-size: 1.35rem; font-weight: 700; display: flex; align-items: center; gap: 0.5rem; }
    .header-title p { font-size: 0.85rem; color: var(--text-muted); }
    .badge {
      display: inline-block;
      padding: 0.2rem 0.5rem;
      border-radius: 9999px;
      font-size: 0.75rem;
      font-weight: 600;
      text-transform: uppercase;
    }
    .badge-primary { background: var(--surface-subtle); color: var(--primary); border: 1px solid var(--border); }
    .badge-success { background: var(--success-bg); color: var(--success); }
    .badge-warning { background: var(--warning-bg); color: var(--warning); }
    .badge-danger { background: var(--danger-bg); color: var(--danger); }

    /* Key Gate */
    .key-gate {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 0.75rem 1rem;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 0.75rem;
    }
    .key-gate label { font-size: 0.85rem; font-weight: 600; white-space: nowrap; }
    .key-gate input[type="password"] {
      flex: 1;
      min-width: 200px;
      padding: 0.45rem 0.75rem;
      border: 1px solid var(--border);
      border-radius: 6px;
      background: var(--surface-subtle);
      color: var(--text);
      font-size: 0.9rem;
    }
    .key-gate input:focus-visible, button:focus-visible, textarea:focus-visible { outline: none; box-shadow: var(--focus-ring); }
    button {
      background: var(--primary);
      color: #ffffff;
      border: none;
      border-radius: 6px;
      padding: 0.45rem 0.9rem;
      font-size: 0.85rem;
      font-weight: 600;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
    }
    button:hover { background: var(--primary-hover); }
    button.btn-secondary { background: var(--surface-subtle); color: var(--text); border: 1px solid var(--border); }
    button.btn-secondary:hover { background: var(--border); }
    button.btn-danger { background: var(--danger); }

    /* Warning Banner */
    .alert-banner {
      background: var(--warning-bg);
      border: 1px solid var(--warning);
      border-radius: var(--radius);
      padding: 0.85rem 1rem;
      font-size: 0.85rem;
      color: var(--text);
      display: flex;
      gap: 0.75rem;
      align-items: flex-start;
    }
    .alert-icon { font-size: 1.25rem; line-height: 1; }

    /* Four Status Cards */
    .grid-cards {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
      gap: 1rem;
    }
    .card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 1rem;
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
    }
    .card-title { font-size: 0.8rem; text-transform: uppercase; font-weight: 600; color: var(--text-muted); }
    .card-metric { font-size: 1.5rem; font-weight: 700; }
    .card-subtext { font-size: 0.8rem; color: var(--text-muted); }

    /* Section styling */
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 1.25rem;
      display: flex;
      flex-direction: column;
      gap: 1rem;
    }
    .panel-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      flex-wrap: wrap;
      gap: 0.5rem;
    }
    .panel-header h2 { font-size: 1.1rem; font-weight: 600; }

    /* Tables */
    .table-responsive { overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; font-size: 0.85rem; text-align: left; }
    th { padding: 0.6rem 0.75rem; background: var(--surface-subtle); color: var(--text-muted); font-weight: 600; border-bottom: 1px solid var(--border); }
    td { padding: 0.6rem 0.75rem; border-bottom: 1px solid var(--border); }
    tr:last-child td { border-bottom: none; }
    .mono { font-family: var(--font-mono); }

    /* Configuration code */
    pre code {
      display: block;
      padding: 0.85rem;
      background: var(--surface-subtle);
      border: 1px solid var(--border);
      border-radius: 6px;
      font-family: var(--font-mono);
      font-size: 0.8rem;
      overflow-x: auto;
      white-space: pre;
    }

    /* Modal / Validate Box */
    .validate-box {
      display: flex;
      flex-direction: column;
      gap: 0.75rem;
      padding: 1rem;
      background: var(--surface-subtle);
      border-radius: 6px;
      border: 1px solid var(--border);
    }
    textarea {
      width: 100%;
      height: 100px;
      padding: 0.5rem;
      font-family: var(--font-mono);
      font-size: 0.8rem;
      background: var(--surface);
      color: var(--text);
      border: 1px solid var(--border);
      border-radius: 6px;
      resize: vertical;
    }
    .validate-result {
      padding: 0.75rem;
      border-radius: 6px;
      font-size: 0.85rem;
      display: none;
    }
    .footer { font-size: 0.75rem; color: var(--text-muted); text-align: center; margin-top: 1rem; }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div class="header-title">
        <h1>Live Voice Scheduler <span class="badge badge-primary">v0.1.1</span></h1>
        <p>Codex Live Voice (gpt-live-1-codex) OAuth candidate routing & delegation controller</p>
      </div>
      <div>
        <button id="btnRefresh" title="Refresh status and decisions">↻ Refresh Data</button>
      </div>
    </header>

    <!-- Key Gate -->
    <div class="key-gate" role="region" aria-label="Management Authentication Gate">
      <label for="mgmtKeyInput">Management Key:</label>
      <input type="password" id="mgmtKeyInput" placeholder="Enter CPA Management Key..." autocomplete="off">
      <button id="btnSaveKey">Set Key</button>
      <button id="btnClearKey" class="btn-secondary">Clear Key</button>
      <span id="keyStatusBadge" class="badge badge-warning">No Key In Session</span>
    </div>

    <!-- Scheduler Ownership Warning Banner -->
    <div class="alert-banner" role="alert">
      <div class="alert-icon">⚠️</div>
      <div>
        <strong>Scheduler Ownership Architecture:</strong>
        <code>cpa-live-voice</code> is designed as the sole active scheduler plugin and must be assigned priority above any other scheduler plugins. Non-Live requests are delegated internally to CPA's built-in scheduler (preserving PAT token candidacy). Because delegation is handled directly, lower-priority plugin schedulers will not execute. Note: Deployments with CLIProxyAPIHome enabled bypass local plugin schedulers and are unsupported in v0.1.
      </div>
    </div>

    <!-- Four Status Cards -->
    <div class="grid-cards">
      <div class="card">
        <div class="card-title">Active Strategy &amp; Fail-Safe</div>
        <div class="card-metric" id="cardStrategy">fill-first</div>
        <div class="card-subtext" id="cardFailClosed">Fail-Closed: Enabled</div>
      </div>
      <div class="card">
        <div class="card-title">Live Pool Health</div>
        <div class="card-metric" id="cardPoolSize">0 IDs</div>
        <div class="card-subtext" id="cardPoolHealth">Status: Loading...</div>
      </div>
      <div class="card">
        <div class="card-title">Traffic Breakdown</div>
        <div class="card-metric" id="cardTotalDecisions">0</div>
        <div class="card-subtext" id="cardTrafficRatio">Live: 0 | Normal: 0</div>
      </div>
      <div class="card">
        <div class="card-title">Live Outcomes</div>
        <div class="card-metric" id="cardSelectedCount">0</div>
        <div class="card-subtext" id="cardOutcomeDetail">Delegated: 0 | Rejected: 0</div>
      </div>
    </div>

    <!-- Pool Inventory Panel -->
    <div class="panel">
      <div class="panel-header">
        <h2>Configured Live Auth Pool</h2>
        <span class="card-subtext">Exact OAuth identifiers qualified for gpt-live-1-codex routing</span>
      </div>
      <div class="table-responsive">
        <table id="poolTable">
          <thead>
            <tr>
              <th style="width: 60px;">#</th>
              <th>Auth Identifier</th>
              <th style="width: 140px;">Candidate Target</th>
            </tr>
          </thead>
          <tbody id="poolTableBody">
            <tr><td colspan="3" style="text-align: center; color: var(--text-muted);">No configured live auth IDs found.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Candidate Inventory Validation -->
    <div class="panel">
      <div class="panel-header">
        <h2>Host Inventory Check</h2>
        <button id="btnValidate" class="btn-secondary">Refresh Check</button>
      </div>
      <div class="validate-box">
        <p class="card-subtext">Compares the configured live pool against CPA auth files automatically. No paste required.</p>
        <div id="validateOutput" class="validate-result" role="region" aria-live="polite"></div>
      </div>
    </div>

    <!-- Recent Decisions Log -->
    <div class="panel">
      <div class="panel-header">
        <h2>Recent Decision History (Max 32)</h2>
        <span class="card-subtext">Race-safe bounded log. Zero tokens, credentials, or SDP payload metadata stored.</span>
      </div>
      <div class="table-responsive">
        <table>
          <thead>
            <tr>
              <th style="width: 60px;">ID</th>
              <th style="width: 170px;">Timestamp</th>
              <th style="width: 80px;">Type</th>
              <th style="width: 90px;">Outcome</th>
              <th style="width: 80px;">Candidates</th>
              <th style="width: 80px;">Matching</th>
              <th>Sanitized Reason</th>
            </tr>
          </thead>
          <tbody id="decisionsTableBody">
            <tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No routing decisions recorded yet.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Configuration Guidance -->
    <div class="panel">
      <div class="panel-header">
        <h2>Configuration Reference</h2>
        <button id="btnCopyConfig" class="btn-secondary">📋 Copy Example YAML</button>
      </div>
      <p style="font-size: 0.85rem; color: var(--text-muted);">
        Place configuration under <code>plugins.configs.cpa-live-voice</code> in your CPA <code>config.yaml</code>.
        Browser configuration file editing is intentionally prohibited.
      </p>
      <pre><code id="configExampleCode">plugins:
  enabled: true
  configs:
    cpa-live-voice:
      enabled: true
      priority: 100
      live_auth_ids:
        - "codex-account-primary"
        - "codex-account-secondary"
      strategy: "fill-first" # fill-first or round-robin
      fail_closed: true     # reject with live_oauth_unavailable when no match</code></pre>
    </div>

    <div class="footer">
      cpa-live-voice v0.1.1 &bull; Native Go Dynamic Plugin &bull; <a href="https://github.com/karlorz/cpa-live-voice" target="_blank" rel="noopener noreferrer" style="color: var(--primary);">GitHub Repository</a>
    </div>
  </div>

  <script>
    (function() {
      var STORAGE_KEY = "cpa_live_voice_mgmt_key";

      function getSavedKey() {
        try {
          return sessionStorage.getItem(STORAGE_KEY) || "";
        } catch(e) {
          return "";
        }
      }

      function setSavedKey(key) {
        try {
          if (key) {
            sessionStorage.setItem(STORAGE_KEY, key);
          } else {
            sessionStorage.removeItem(STORAGE_KEY);
          }
        } catch(e) {}
      }

      function updateKeyBadge() {
        var key = getSavedKey();
        var badge = document.getElementById("keyStatusBadge");
        var input = document.getElementById("mgmtKeyInput");
        if (key) {
          badge.textContent = "Key Active in Session";
          badge.className = "badge badge-success";
          input.value = "••••••••••••";
        } else {
          badge.textContent = "No Key In Session";
          badge.className = "badge badge-warning";
          input.value = "";
        }
      }

      function getAuthHeader() {
        var key = getSavedKey();
        return key ? { "Authorization": "Bearer " + key } : {};
      }

      function safeText(str) {
        if (!str) return "";
        var div = document.createElement("div");
        div.textContent = str;
        return div.innerHTML;
      }

      function fetchStatus() {
        var headers = getAuthHeader();
        fetch("/v0/management/plugins/cpa-live-voice/status", {
          method: "GET",
          headers: headers
        })
        .then(function(res) {
          if (res.status === 401 || res.status === 403) {
            throw new Error("Management Key invalid or missing (HTTP " + res.status + ")");
          }
          if (!res.ok) {
            throw new Error("Failed to load status (HTTP " + res.status + ")");
          }
          return res.json();
        })
        .then(function(data) {
          renderStatus(data);
          runValidation();
        })
        .catch(function(err) {
          var poolBody = document.getElementById("poolTableBody");
          poolBody.innerHTML = '<tr><td colspan="3" style="text-align: center; color: var(--danger);">' + safeText(err.message) + '</td></tr>';
          document.getElementById("cardPoolHealth").textContent = "Status: Locked";
        });
      }

      function renderStatus(data) {
        var cfg = data.config || {};
        document.getElementById("cardStrategy").textContent = cfg.strategy || "fill-first";
        document.getElementById("cardFailClosed").textContent = "Fail-Closed: " + (cfg.fail_closed ? "Enabled" : "Disabled");
        
        var pool = cfg.live_auth_ids || [];
        document.getElementById("cardPoolSize").textContent = pool.length + " IDs";
        document.getElementById("cardPoolHealth").textContent = pool.length > 0 ? "Status: Operational" : "Status: Pool Empty";

        document.getElementById("cardTotalDecisions").textContent = data.total_decisions || 0;
        document.getElementById("cardTrafficRatio").textContent = "Live: " + (data.live_decisions || 0) + " | Normal: " + (data.normal_decisions || 0);

        document.getElementById("cardSelectedCount").textContent = data.selected_count || 0;
        document.getElementById("cardOutcomeDetail").textContent = "Delegated: " + (data.delegated_count || 0) + " | Rejected: " + (data.rejected_count || 0);

        // Render Pool Table
        var poolBody = document.getElementById("poolTableBody");
        if (pool.length === 0) {
          poolBody.innerHTML = '<tr><td colspan="3" style="text-align: center; color: var(--text-muted);">No configured live auth IDs.</td></tr>';
        } else {
          var rows = "";
          for (var i = 0; i < pool.length; i++) {
            rows += '<tr>' +
              '<td>' + (i + 1) + '</td>' +
              '<td class="mono"><strong>' + safeText(pool[i]) + '</strong></td>' +
              '<td><span class="badge badge-success">Target</span></td>' +
              '</tr>';
          }
          poolBody.innerHTML = rows;
        }

        // Render Decisions Table
        var decBody = document.getElementById("decisionsTableBody");
        var decisions = data.recent_decisions || [];
        if (decisions.length === 0) {
          decBody.innerHTML = '<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No routing decisions recorded yet.</td></tr>';
        } else {
          var decRows = "";
          for (var j = 0; j < decisions.length; j++) {
            var d = decisions[j];
            var outcomeBadge = "badge-primary";
            if (d.outcome === "selected") outcomeBadge = "badge-success";
            else if (d.outcome === "rejected") outcomeBadge = "badge-danger";
            else if (d.outcome === "delegated") outcomeBadge = "badge-warning";

            var timeStr = d.timestamp ? d.timestamp.replace("T", " ").replace("Z", "").slice(0, 19) : "-";
            decRows += '<tr>' +
              '<td>' + d.id + '</td>' +
              '<td class="mono" style="font-size: 0.75rem;">' + safeText(timeStr) + '</td>' +
              '<td><span class="badge ' + (d.classification === "live" ? "badge-primary" : "badge-secondary") + '">' + safeText(d.classification) + '</span></td>' +
              '<td><span class="badge ' + outcomeBadge + '">' + safeText(d.outcome) + '</span></td>' +
              '<td>' + d.candidate_count + '</td>' +
              '<td>' + d.matching_count + '</td>' +
              '<td>' + safeText(d.reason) + '</td>' +
              '</tr>';
          }
          decBody.innerHTML = decRows;
        }
      }

      function hostCandidateIDs(payload) {
        var files = [];
        if (!payload) {
          return [];
        }
        if (Array.isArray(payload)) {
          files = payload;
        } else if (Array.isArray(payload.files)) {
          files = payload.files;
        } else if (payload.data && Array.isArray(payload.data.files)) {
          files = payload.data.files;
        }
        var ids = [];
        var seen = {};
        for (var i = 0; i < files.length; i++) {
          var file = files[i] || {};
          var parts = [file.id, file.name, file.file_name, file.filename];
          for (var j = 0; j < parts.length; j++) {
            var id = String(parts[j] || "").trim();
            if (!id || seen[id]) {
              continue;
            }
            seen[id] = true;
            ids.push(id);
          }
        }
        return ids;
      }

      function fetchHostInventory() {
        var headers = getAuthHeader();
        var urls = ["/v8/management/credentials", "/v0/management/auth-files"];
        function next(index) {
          if (index >= urls.length) {
            return Promise.reject(new Error("Could not load CPA auth-file inventory"));
          }
          return fetch(urls[index], { method: "GET", headers: headers }).then(function(res) {
            if (res.status === 401 || res.status === 403) {
              throw new Error("Management Key invalid or missing (HTTP " + res.status + ")");
            }
            if (res.status === 404) {
              return next(index + 1);
            }
            if (!res.ok) {
              throw new Error("Failed to load host inventory (HTTP " + res.status + ")");
            }
            return res.json();
          });
        }
        return next(0);
      }

      function applyPoolMatchBadges(matchingIDs, missingIDs) {
        var matching = {};
        var missing = {};
        var i;
        for (i = 0; i < (matchingIDs || []).length; i++) {
          matching[matchingIDs[i]] = true;
        }
        for (i = 0; i < (missingIDs || []).length; i++) {
          missing[missingIDs[i]] = true;
        }
        var rows = document.querySelectorAll("#poolTableBody tr");
        for (i = 0; i < rows.length; i++) {
          var idCell = rows[i].querySelector("td.mono");
          var badgeCell = rows[i].querySelector("td:last-child");
          if (!idCell || !badgeCell) {
            continue;
          }
          var id = idCell.textContent.trim();
          if (matching[id]) {
            badgeCell.innerHTML = '<span class="badge badge-success">Present</span>';
          } else if (missing[id]) {
            badgeCell.innerHTML = '<span class="badge badge-danger">Missing</span>';
          }
        }
      }

      function runValidation() {
        var headers = getAuthHeader();
        var out = document.getElementById("validateOutput");
        out.style.display = "block";
        out.style.background = "var(--surface-subtle)";
        out.style.color = "var(--text)";
        out.textContent = "Checking host inventory...";

        fetchHostInventory()
        .then(function(inventory) {
          var ids = hostCandidateIDs(inventory);
          var candidates = [];
          for (var i = 0; i < ids.length; i++) {
            candidates.push({ id: ids[i] });
          }
          var validateHeaders = getAuthHeader();
          validateHeaders["Content-Type"] = "application/json";
          return fetch("/v0/management/plugins/cpa-live-voice/validate", {
            method: "POST",
            headers: validateHeaders,
            body: JSON.stringify({ candidates: candidates })
          }).then(function(res) {
            if (!res.ok) throw new Error("Validation failed (HTTP " + res.status + ")");
            return res.json();
          }).then(function(res) {
            res.host_candidate_count = ids.length;
            return res;
          });
        })
        .then(function(res) {
          var bg = "var(--success-bg)";
          var col = "var(--success)";
          if (res.pool_health === "critical") {
            bg = "var(--danger-bg)";
            col = "var(--danger)";
          } else if (res.pool_health === "degraded") {
            bg = "var(--warning-bg)";
            col = "var(--warning)";
          }
          out.style.background = bg;
          out.style.color = col;
          out.innerHTML = '<strong>Health: ' + safeText(res.pool_health).toUpperCase() + '</strong> &mdash; ' + safeText(res.message) +
            '<br><span class="mono" style="font-size: 0.8rem;">Host files: ' + (res.host_candidate_count || res.candidate_count || 0) + ' | Configured: ' + res.configured_count + ' | Matching: ' + res.matching_count + ' | Missing: ' + res.missing_count + '</span>';
          applyPoolMatchBadges(res.matching_ids, res.missing_ids);
        })
        .catch(function(err) {
          out.style.background = "var(--danger-bg)";
          out.style.color = "var(--danger)";
          out.textContent = err.message;
        });
      }

      // Event Listeners
      document.getElementById("btnSaveKey").addEventListener("click", function() {
        var val = document.getElementById("mgmtKeyInput").value.trim();
        if (val && val !== "••••••••••••") {
          setSavedKey(val);
          updateKeyBadge();
          fetchStatus();
        }
      });

      document.getElementById("btnClearKey").addEventListener("click", function() {
        setSavedKey("");
        updateKeyBadge();
      });

      document.getElementById("btnRefresh").addEventListener("click", function() {
        fetchStatus();
      });

      document.getElementById("btnValidate").addEventListener("click", function() {
        runValidation();
      });

      document.getElementById("btnCopyConfig").addEventListener("click", function() {
        var code = document.getElementById("configExampleCode").textContent;
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(code).then(function() {
            var btn = document.getElementById("btnCopyConfig");
            var original = btn.textContent;
            btn.textContent = "✓ Copied!";
            setTimeout(function() { btn.textContent = original; }, 2000);
          });
        } else {
          var ta = document.createElement("textarea");
          ta.value = code;
          document.body.appendChild(ta);
          ta.select();
          document.execCommand("copy");
          document.body.removeChild(ta);
          alert("Example YAML copied to clipboard!");
        }
      });

      // Init
      updateKeyBadge();
      fetchStatus();
    })();
  </script>
</body>
</html>`
