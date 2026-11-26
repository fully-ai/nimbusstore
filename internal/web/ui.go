package web

import "net/http"

// ServeDashboard writes the NimbusStore Object & Bucket Governance Console HTML.
func ServeDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(DashboardHTML))
}

const DashboardHTML = `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>NimbusStore — S3-Compatible Content-Addressed Object Storage & Governance Console</title>
  <style>
    :root[data-theme="dark"] {
      --bg-primary: #070d19;
      --bg-secondary: #0f172a;
      --bg-card: #111c33;
      --bg-elevated: #172542;
      --border-color: #1e3156;
      --text-primary: #f0f6fc;
      --text-secondary: #94a3b8;
      --text-muted: #64748b;
      --accent-sky: #0ea5e9;
      --accent-cobalt: #0284c7;
      --accent-cyan: #38bdf8;
      --accent-glow: rgba(14, 165, 233, 0.18);
      --success: #10b981;
      --warning: #f59e0b;
      --danger: #ef4444;
      --code-bg: #081020;
    }
    :root[data-theme="light"] {
      --bg-primary: #f1f6fb;
      --bg-secondary: #e2edf8;
      --bg-card: #ffffff;
      --bg-elevated: #f8fafc;
      --border-color: #cbd5e1;
      --text-primary: #0f172a;
      --text-secondary: #334155;
      --text-muted: #64748b;
      --accent-sky: #0284c7;
      --accent-cobalt: #0369a1;
      --accent-cyan: #0ea5e9;
      --accent-glow: rgba(2, 132, 199, 0.12);
      --success: #059669;
      --warning: #d97706;
      --danger: #dc2626;
      --code-bg: #f1f5f9;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Inter, sans-serif;
      background: var(--bg-primary);
      color: var(--text-primary);
      display: flex;
      min-height: 100vh;
      line-height: 1.5;
    }
    .sidebar {
      width: 270px;
      background: var(--bg-secondary);
      border-right: 1px solid var(--border-color);
      display: flex;
      flex-direction: column;
      padding: 1.25rem;
      flex-shrink: 0;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      padding-bottom: 1.25rem;
      border-bottom: 1px solid var(--border-color);
      margin-bottom: 1.25rem;
    }
    .brand-icon {
      width: 40px;
      height: 40px;
      border-radius: 10px;
      background: linear-gradient(135deg, var(--accent-sky), var(--accent-cobalt));
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: 800;
      color: #fff;
      font-size: 1.15rem;
      box-shadow: 0 4px 14px var(--accent-glow);
    }
    .brand-title { font-size: 1.15rem; font-weight: 700; letter-spacing: -0.02em; }
    .brand-sub { font-size: 0.75rem; color: var(--accent-cyan); font-weight: 600; }
    .nav-list { list-style: none; display: flex; flex-direction: column; gap: 0.35rem; flex: 1; }
    .nav-btn {
      width: 100%;
      text-align: left;
      background: transparent;
      border: 1px solid transparent;
      color: var(--text-secondary);
      padding: 0.65rem 0.85rem;
      border-radius: 8px;
      font-size: 0.88rem;
      font-weight: 600;
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 0.65rem;
      transition: all 0.15s ease;
    }
    .nav-btn:hover { background: var(--bg-elevated); color: var(--text-primary); }
    .nav-btn.active {
      background: var(--accent-glow);
      color: var(--accent-cyan);
      border-color: var(--accent-sky);
    }
    .sidebar-footer {
      border-top: 1px solid var(--border-color);
      padding-top: 1rem;
      display: flex;
      flex-direction: column;
      gap: 0.6rem;
    }
    .main {
      flex: 1;
      display: flex;
      flex-direction: column;
      overflow-x: hidden;
    }
    .topbar {
      background: var(--bg-secondary);
      border-bottom: 1px solid var(--border-color);
      padding: 0.9rem 1.75rem;
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 1rem;
      flex-wrap: wrap;
    }
    .topbar-status {
      display: flex;
      align-items: center;
      gap: 0.85rem;
      flex-wrap: wrap;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      gap: 0.4rem;
      font-size: 0.76rem;
      font-weight: 600;
      padding: 0.25rem 0.65rem;
      border-radius: 999px;
      background: var(--bg-elevated);
      border: 1px solid var(--border-color);
      color: var(--text-secondary);
    }
    .pill.ok { color: var(--success); border-color: rgba(16, 185, 129, 0.35); }
    .pill.sky { color: var(--accent-cyan); border-color: rgba(56, 189, 248, 0.35); }
    .pill.warn { color: var(--warning); border-color: rgba(245, 158, 11, 0.35); }
    .pill.danger { color: var(--danger); border-color: rgba(239, 68, 68, 0.35); }
    .content { padding: 1.75rem; max-width: 1450px; width: 100%; margin: 0 auto; }
    .tab-pane { display: none; }
    .tab-pane.active { display: block; }
    .grid-4 {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
      gap: 1.1rem;
      margin-bottom: 1.5rem;
    }
    .grid-2 {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(430px, 1fr));
      gap: 1.25rem;
      margin-bottom: 1.5rem;
    }
    .card {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: 12px;
      padding: 1.25rem;
      box-shadow: 0 4px 16px rgba(0, 0, 0, 0.14);
    }
    .card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 1rem;
      gap: 0.75rem;
      flex-wrap: wrap;
    }
    .card-title { font-size: 1rem; font-weight: 700; color: var(--text-primary); }
    .stat-label { font-size: 0.78rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); font-weight: 600; }
    .stat-value { font-size: 1.75rem; font-weight: 800; margin: 0.35rem 0; color: var(--text-primary); }
    .stat-meta { font-size: 0.8rem; color: var(--text-secondary); }
    .btn {
      background: linear-gradient(135deg, var(--accent-sky), var(--accent-cobalt));
      color: #fff;
      border: none;
      padding: 0.55rem 1rem;
      border-radius: 8px;
      font-weight: 600;
      font-size: 0.84rem;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 0.45rem;
      transition: opacity 0.15s;
    }
    .btn:hover { opacity: 0.92; }
    .btn-secondary {
      background: var(--bg-elevated);
      color: var(--text-primary);
      border: 1px solid var(--border-color);
    }
    .btn-danger {
      background: rgba(239, 68, 68, 0.16);
      color: var(--danger);
      border: 1px solid rgba(239, 68, 68, 0.35);
    }
    .btn-sm { padding: 0.32rem 0.65rem; font-size: 0.76rem; }
    .form-group { margin-bottom: 0.9rem; }
    .form-label { display: block; font-size: 0.8rem; font-weight: 600; color: var(--text-secondary); margin-bottom: 0.35rem; }
    .input, .select, .textarea {
      width: 100%;
      background: var(--code-bg);
      border: 1px solid var(--border-color);
      color: var(--text-primary);
      padding: 0.55rem 0.75rem;
      border-radius: 8px;
      font-size: 0.86rem;
    }
    .textarea { min-height: 95px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
    .checkbox-row {
      display: flex;
      gap: 1.25rem;
      align-items: center;
      flex-wrap: wrap;
      margin: 0.65rem 0;
      font-size: 0.85rem;
      color: var(--text-secondary);
    }
    table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
    th, td {
      text-align: left;
      padding: 0.7rem 0.75rem;
      border-bottom: 1px solid var(--border-color);
    }
    th {
      font-size: 0.75rem;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      color: var(--text-muted);
      font-weight: 700;
    }
    code, .mono {
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 0.8rem;
      background: var(--code-bg);
      padding: 0.15rem 0.4rem;
      border-radius: 5px;
      border: 1px solid var(--border-color);
    }
    .progress-bar {
      width: 100%;
      height: 8px;
      background: var(--code-bg);
      border-radius: 999px;
      overflow: hidden;
      margin-top: 0.35rem;
      border: 1px solid var(--border-color);
    }
    .progress-fill {
      height: 100%;
      background: linear-gradient(90deg, var(--accent-sky), var(--accent-cyan));
      transition: width 0.25s ease;
    }
    .toast {
      position: fixed;
      bottom: 1.5rem;
      right: 1.5rem;
      background: var(--bg-elevated);
      border: 1px solid var(--accent-sky);
      color: var(--text-primary);
      padding: 0.8rem 1.15rem;
      border-radius: 10px;
      box-shadow: 0 10px 25px rgba(0,0,0,0.35);
      font-size: 0.86rem;
      font-weight: 600;
      z-index: 9999;
      display: none;
    }
  </style>
</head>
<body>
  <aside class="sidebar">
    <div class="brand">
      <div class="brand-icon">NS</div>
      <div>
        <div class="brand-title">NimbusStore</div>
        <div class="brand-sub">CAS S3 & Governance Cloud</div>
      </div>
    </div>
    <ul class="nav-list">
      <li><button class="nav-btn active" onclick="switchTab('overview', this)"><span>📊</span> Overview & Deduplication</button></li>
      <li><button class="nav-btn" onclick="switchTab('buckets', this)"><span>🪣</span> Buckets & Quotas</button></li>
      <li><button class="nav-btn" onclick="switchTab('explorer', this)"><span>🗂️</span> Object Explorer & Versions</button></li>
      <li><button class="nav-btn" onclick="switchTab('uploader', this)"><span>⬆️</span> Object Uploader</button></li>
      <li><button class="nav-btn" onclick="switchTab('presign', this)"><span>🔐</span> Presigned URL Studio</button></li>
      <li><button class="nav-btn" onclick="switchTab('scrub', this)"><span>🛡️</span> Bit-Rot Scrubber & Backup</button></li>
      <li><button class="nav-btn" onclick="switchTab('audit', this)"><span>📜</span> Governance Audit Logs</button></li>
    </ul>
    <div class="sidebar-footer">
      <div class="form-group" style="margin-bottom:0.35rem;">
        <label class="form-label">API Token / Secret Key</label>
        <input id="apiTokenInput" class="input" type="password" placeholder="Optional Bearer token" onchange="saveAuthToken()" />
      </div>
      <button class="btn btn-secondary" onclick="toggleTheme()" style="width:100%;justify-content:center;">🌓 Toggle Dark / Light Theme</button>
    </div>
  </aside>

  <div class="main">
    <header class="topbar">
      <div>
        <strong style="font-size:1.05rem;">NimbusStore Object & Bucket Governance Console</strong>
        <span style="color:var(--text-secondary);font-size:0.84rem;margin-left:0.65rem;">SHA-256 Content-Addressed Storage • S3 API • HMAC Presigning</span>
      </div>
      <div class="topbar-status">
        <span id="healthBadge" class="pill ok">● CAS Engine Online</span>
        <span id="dedupBadge" class="pill sky">Dedup Savings: 0 B</span>
        <button class="btn btn-sm btn-secondary" onclick="refreshAll()">↻ Refresh Telemetry</button>
      </div>
    </header>

    <main class="content">
      <!-- TAB 1: OVERVIEW -->
      <section id="tab-overview" class="tab-pane active">
        <div class="grid-4">
          <div class="card">
            <div class="stat-label">Managed Buckets</div>
            <div id="statBuckets" class="stat-value">0</div>
            <div id="statActiveKeys" class="stat-meta">0 active object keys</div>
          </div>
          <div class="card">
            <div class="stat-label">Object Versions</div>
            <div id="statVersions" class="stat-value">0</div>
            <div id="statLogicalBytes" class="stat-meta">0 B logical footprint</div>
          </div>
          <div class="card">
            <div class="stat-label">Physical CAS Blobs</div>
            <div id="statUniqueBlobs" class="stat-value">0</div>
            <div id="statPhysicalBytes" class="stat-meta">0 B on-disk CAS footprint</div>
          </div>
          <div class="card">
            <div class="stat-label">Deduplication Efficiency</div>
            <div id="statDedupHits" class="stat-value">0 hits</div>
            <div id="statDedupSaved" class="stat-meta">0 B saved via SHA-256 CAS</div>
          </div>
        </div>

        <div class="grid-2">
          <div class="card">
            <div class="card-header">
              <div class="card-title">Bucket Governance & Quota Utilization</div>
              <button class="btn btn-sm" onclick="switchTab('buckets', document.querySelectorAll('.nav-btn')[1])">+ New Bucket</button>
            </div>
            <div id="overviewBucketsList">Loading buckets...</div>
          </div>

          <div class="card">
            <div class="card-header">
              <div class="card-title">CAS Integrity & Quick Operations</div>
              <button class="btn btn-sm" onclick="triggerScrub()">🛡️ Run Bit-Rot Scrub Now</button>
            </div>
            <p style="font-size:0.86rem;color:var(--text-secondary);margin-bottom:1rem;">
              Every object version uploaded to NimbusStore is content-addressed by its SHA-256 cryptographic digest. Identical payloads across different keys, versions, or buckets are stored once on disk.
            </p>
            <div id="overviewScrubBox" style="padding:0.9rem;background:var(--code-bg);border:1px solid var(--border-color);border-radius:8px;font-size:0.84rem;">
              No bit-rot scrub executed in this session yet. Click <strong>Run Bit-Rot Scrub Now</strong> to verify every CAS blob on disk.
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 2: BUCKETS -->
      <section id="tab-buckets" class="tab-pane">
        <div class="grid-2">
          <div class="card">
            <div class="card-header">
              <div class="card-title">Create or Update Bucket Policy</div>
            </div>
            <form onsubmit="submitBucketForm(event)">
              <div class="form-group">
                <label class="form-label">Bucket Name (3-63 chars, lowercase alphanumeric / hyphen)</label>
                <input id="bucketName" class="input" type="text" placeholder="e.g. ml-artifacts-prod" required />
              </div>
              <div class="grid-2" style="margin-bottom:0;">
                <div class="form-group">
                  <label class="form-label">Storage Quota (MB)</label>
                  <input id="bucketQuota" class="input" type="number" min="1" max="1048576" value="50" required />
                </div>
                <div class="form-group">
                  <label class="form-label">Lifecycle Retention (Days, 0 = Indefinite)</label>
                  <input id="bucketRetention" class="input" type="number" min="0" max="36500" value="90" required />
                </div>
              </div>
              <div class="checkbox-row">
                <label><input id="bucketVersioning" type="checkbox" checked /> Enable Immutable Object Versioning</label>
                <label><input id="bucketPublicRead" type="checkbox" /> Allow Anonymous Public Read</label>
              </div>
              <div style="display:flex;gap:0.75rem;margin-top:0.9rem;">
                <button type="submit" class="btn">Save Bucket Governance Policy</button>
              </div>
            </form>
          </div>

          <div class="card">
            <div class="card-header">
              <div class="card-title">Configured Buckets</div>
            </div>
            <div style="overflow-x:auto;">
              <table>
                <thead>
                  <tr>
                    <th>Bucket</th>
                    <th>Policies</th>
                    <th>Usage / Quota</th>
                    <th>Objects (Versions)</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody id="bucketsTableBody"></tbody>
              </table>
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 3: OBJECT EXPLORER -->
      <section id="tab-explorer" class="tab-pane">
        <div class="card">
          <div class="card-header">
            <div class="card-title">Object Explorer & Immutable Version History</div>
            <div style="display:flex;gap:0.65rem;align-items:center;flex-wrap:wrap;">
              <select id="explorerBucketSelect" class="select" style="width:210px;" onchange="loadObjects()"></select>
              <input id="explorerPrefix" class="input" style="width:200px;" placeholder="Filter key prefix..." oninput="loadObjects()" />
              <label style="font-size:0.82rem;color:var(--text-secondary);display:flex;align-items:center;gap:0.35rem;">
                <input id="explorerIncludeVersions" type="checkbox" checked onchange="loadObjects()" /> Show All Versions & Delete Markers
              </label>
              <button class="btn btn-sm btn-secondary" onclick="loadObjects()">↻ Reload</button>
            </div>
          </div>
          <div style="overflow-x:auto;">
            <table>
              <thead>
                <tr>
                  <th>Object Key</th>
                  <th>Version ID</th>
                  <th>Status</th>
                  <th>SHA-256 Content Digest</th>
                  <th>Size</th>
                  <th>Tags</th>
                  <th>Created (UTC)</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody id="objectsTableBody"></tbody>
            </table>
          </div>
        </div>
      </section>

      <!-- TAB 4: UPLOADER -->
      <section id="tab-uploader" class="tab-pane">
        <div class="grid-2">
          <div class="card">
            <div class="card-header">
              <div class="card-title">Upload Object to Bucket (CAS SHA-256 Deduplicated)</div>
            </div>
            <form onsubmit="submitUploadForm(event)">
              <div class="form-group">
                <label class="form-label">Target Bucket</label>
                <select id="uploadBucketSelect" class="select" required></select>
              </div>
              <div class="form-group">
                <label class="form-label">Object Key (Path)</label>
                <input id="uploadKey" class="input" type="text" placeholder="e.g. configs/cluster-v1.json" required />
              </div>
              <div class="grid-2" style="margin-bottom:0;">
                <div class="form-group">
                  <label class="form-label">Content-Type</label>
                  <input id="uploadContentType" class="input" type="text" value="text/plain; charset=utf-8" />
                </div>
                <div class="form-group">
                  <label class="form-label">Metadata Tags (k=v, comma-separated)</label>
                  <input id="uploadTags" class="input" type="text" placeholder="env=prod,team=storage" />
                </div>
              </div>
              <div class="form-group">
                <label class="form-label">Choose File (optional — overrides text box below if selected)</label>
                <input id="uploadFileInput" class="input" type="file" onchange="onFilePicked()" />
              </div>
              <div class="form-group">
                <label class="form-label">Or Enter Raw Object Payload (Text / JSON / Config)</label>
                <textarea id="uploadTextContent" class="textarea" placeholder="Paste content to store in CAS engine...">{"service":"nimbusstore","storage":"cas-sha256","deduplication":true}</textarea>
              </div>
              <button type="submit" class="btn">⬆️ Put Object Version</button>
            </form>
          </div>

          <div class="card">
            <div class="card-header">
              <div class="card-title">Last Upload & CAS Deduplication Receipt</div>
            </div>
            <div id="uploadResultBox" style="padding:1rem;background:var(--code-bg);border:1px solid var(--border-color);border-radius:8px;font-size:0.85rem;">
              Upload an object to view its SHA-256 content digest, assigned Version ID, and whether the CAS engine deduplicated the payload against an existing physical blob.
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 5: PRESIGN STUDIO -->
      <section id="tab-presign" class="tab-pane">
        <div class="grid-2">
          <div class="card">
            <div class="card-header">
              <div class="card-title">HMAC-SHA256 Presigned URL Generator</div>
            </div>
            <form onsubmit="submitPresignForm(event)">
              <div class="grid-2" style="margin-bottom:0;">
                <div class="form-group">
                  <label class="form-label">HTTP Method Binding</label>
                  <select id="presignMethod" class="select">
                    <option value="GET">GET (Download Object)</option>
                    <option value="PUT">PUT (Direct S3 Upload)</option>
                  </select>
                </div>
                <div class="form-group">
                  <label class="form-label">Expiration TTL (Seconds)</label>
                  <input id="presignTTL" class="input" type="number" min="5" max="604800" value="900" required />
                </div>
              </div>
              <div class="form-group">
                <label class="form-label">Bucket</label>
                <select id="presignBucketSelect" class="select" required></select>
              </div>
              <div class="form-group">
                <label class="form-label">Object Key</label>
                <input id="presignKey" class="input" type="text" placeholder="e.g. docs/architecture.md" required />
              </div>
              <div class="form-group">
                <label class="form-label">Specific Version ID (optional)</label>
                <input id="presignVersionId" class="input" type="text" placeholder="Leave blank for latest version" />
              </div>
              <button type="submit" class="btn">🔐 Generate Presigned URL</button>
            </form>
          </div>

          <div class="card">
            <div class="card-header">
              <div class="card-title">Generated Presigned Token & Verification Link</div>
            </div>
            <div id="presignResultBox" style="padding:1rem;background:var(--code-bg);border:1px solid var(--border-color);border-radius:8px;font-size:0.85rem;">
              Generate a presigned URL to inspect the canonical HMAC-SHA256 signature, expiration timestamp, and one-click browser verification link.
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 6: SCRUBBER & BACKUP -->
      <section id="tab-scrub" class="tab-pane">
        <div class="grid-2">
          <div class="card">
            <div class="card-header">
              <div class="card-title">SHA-256 Bit-Rot Integrity Scrubber</div>
              <button class="btn btn-sm" onclick="triggerScrub()">🛡️ Execute Full CAS Scrub</button>
            </div>
            <p style="font-size:0.85rem;color:var(--text-secondary);margin-bottom:1rem;">
              Scans every physical <code>.blob</code> file in the CAS shard tree, recomputes its SHA-256 digest from raw bytes on disk, and alerts immediately if silent disk corruption or bit-rot has altered any byte.
            </p>
            <div id="scrubReportBox" style="padding:1rem;background:var(--code-bg);border:1px solid var(--border-color);border-radius:8px;font-size:0.85rem;">
              Click <strong>Execute Full CAS Scrub</strong> to run verification.
            </div>
          </div>

          <div class="card">
            <div class="card-header">
              <div class="card-title">Transactional Catalog Snapshot Backup & Restore</div>
              <button class="btn btn-sm btn-secondary" onclick="exportBackup()">⬇️ Export Snapshot JSON</button>
            </div>
            <div class="form-group">
              <label class="form-label">Catalog Snapshot Payload (JSON)</label>
              <textarea id="backupJsonArea" class="textarea" style="min-height:175px;" placeholder="Click 'Export Snapshot JSON' to load current catalog backup or paste a snapshot here to restore..."></textarea>
            </div>
            <button class="btn btn-sm" onclick="restoreBackup()">♻️ Restore Catalog Snapshot</button>
          </div>
        </div>
      </section>

      <!-- TAB 7: AUDIT LOGS -->
      <section id="tab-audit" class="tab-pane">
        <div class="card">
          <div class="card-header">
            <div class="card-title">Governance & Data-Plane Access Audit Trail</div>
            <button class="btn btn-sm btn-secondary" onclick="loadAudits()">↻ Refresh Logs</button>
          </div>
          <div style="overflow-x:auto;">
            <table>
              <thead>
                <tr>
                  <th>Timestamp (UTC)</th>
                  <th>Action</th>
                  <th>Bucket</th>
                  <th>Key / Version</th>
                  <th>Actor</th>
                  <th>Status</th>
                  <th>Details</th>
                </tr>
              </thead>
              <tbody id="auditTableBody"></tbody>
            </table>
          </div>
        </div>
      </section>
    </main>
  </div>

  <div id="toast" class="toast"></div>

  <script>
    let currentBuckets = [];

    function showToast(msg, isErr) {
      const el = document.getElementById('toast');
      el.textContent = msg;
      el.style.borderColor = isErr ? 'var(--danger)' : 'var(--accent-sky)';
      el.style.display = 'block';
      setTimeout(() => { el.style.display = 'none'; }, 3600);
    }

    function toggleTheme() {
      const root = document.documentElement;
      const next = root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
      root.setAttribute('data-theme', next);
      localStorage.setItem('nimbus_theme', next);
    }

    function saveAuthToken() {
      const val = document.getElementById('apiTokenInput').value.trim();
      localStorage.setItem('nimbus_token', val);
      showToast('Saved API authentication token');
      refreshAll();
    }

    function authHeaders(extra = {}) {
      const h = Object.assign({}, extra);
      const t = localStorage.getItem('nimbus_token') || '';
      if (t) h['Authorization'] = 'Bearer ' + t;
      return h;
    }

    async function apiFetch(path, opts = {}) {
      opts.headers = authHeaders(opts.headers || {});
      const res = await fetch(path, opts);
      const text = await res.text();
      let data = {};
      try { data = text ? JSON.parse(text) : {}; } catch (_) { data = { raw: text }; }
      if (!res.ok) {
        throw new Error(data.error || ('HTTP ' + res.status));
      }
      return data;
    }

    function formatBytes(bytes) {
      const n = Number(bytes || 0);
      if (n < 1024) return n + ' B';
      if (n < 1024 * 1024) return (n / 1024).toFixed(2) + ' KB';
      return (n / (1024 * 1024)).toFixed(2) + ' MB';
    }

    function escapeHtml(s) {
      return String(s ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
    }

    function switchTab(name, btn) {
      document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));
      document.getElementById('tab-' + name).classList.add('active');
      document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
      if (btn) btn.classList.add('active');
      if (name === 'explorer') loadObjects();
      if (name === 'audit') loadAudits();
    }

    async function loadMetricsAndBuckets() {
      const [metrics, bucketsResp] = await Promise.all([
        apiFetch('/metrics'),
        apiFetch('/api/v1/buckets')
      ]);

      currentBuckets = bucketsResp.buckets || [];
      document.getElementById('statBuckets').textContent = metrics.catalog.total_buckets;
      document.getElementById('statActiveKeys').textContent = metrics.catalog.total_active_keys + ' active object keys';
      document.getElementById('statVersions').textContent = metrics.catalog.total_versions;
      document.getElementById('statLogicalBytes').textContent = formatBytes(metrics.catalog.total_logical_bytes) + ' logical footprint';
      document.getElementById('statUniqueBlobs').textContent = metrics.cas.unique_blobs;
      document.getElementById('statPhysicalBytes').textContent = formatBytes(metrics.cas.physical_bytes) + ' on-disk CAS footprint';
      document.getElementById('statDedupHits').textContent = metrics.cas.dedup_hits + ' hits';
      document.getElementById('statDedupSaved').textContent = formatBytes(metrics.cas.dedup_bytes_saved) + ' saved via SHA-256 CAS';
      document.getElementById('dedupBadge').textContent = 'Dedup Saved: ' + formatBytes(metrics.cas.dedup_bytes_saved);

      if (metrics.cas.last_scrub) {
        renderScrubSummary(metrics.cas.last_scrub);
      }

      renderBucketSelects();
      renderBucketsViews();
    }

    function renderBucketSelects() {
      const ids = ['explorerBucketSelect', 'uploadBucketSelect', 'presignBucketSelect'];
      ids.forEach(id => {
        const sel = document.getElementById(id);
        const prev = sel.value;
        sel.innerHTML = '';
        currentBuckets.forEach(b => {
          const opt = document.createElement('option');
          opt.value = b.name;
          opt.textContent = b.name + (b.versioning_enabled ? ' (versioned)' : '');
          sel.appendChild(opt);
        });
        if (prev && currentBuckets.some(b => b.name === prev)) {
          sel.value = prev;
        }
      });
    }

    function renderBucketsViews() {
      const overviewEl = document.getElementById('overviewBucketsList');
      const tbody = document.getElementById('bucketsTableBody');
      if (currentBuckets.length === 0) {
        overviewEl.innerHTML = '<p style="color:var(--text-muted);font-size:0.85rem;">No buckets exist yet. Create one in Buckets & Quotas.</p>';
        tbody.innerHTML = '<tr><td colspan="5" style="color:var(--text-muted);">No buckets configured.</td></tr>';
        return;
      }

      let ovHtml = '';
      let rowsHtml = '';
      currentBuckets.forEach(b => {
        const quotaBytes = b.quota_mb * 1024 * 1024;
        const pct = quotaBytes > 0 ? Math.min(100, ((b.logical_bytes / quotaBytes) * 100)).toFixed(1) : 0;
        ovHtml += '<div style="margin-bottom:0.9rem;">' +
          '<div style="display:flex;justify-content:space-between;font-size:0.84rem;font-weight:600;">' +
            '<span>🪣 ' + escapeHtml(b.name) + '</span>' +
            '<span>' + formatBytes(b.logical_bytes) + ' / ' + b.quota_mb + ' MB (' + pct + '%)</span>' +
          '</div>' +
          '<div class="progress-bar"><div class="progress-fill" style="width:' + pct + '%;"></div></div>' +
        '</div>';

        rowsHtml += '<tr>' +
          '<td><strong>' + escapeHtml(b.name) + '</strong></td>' +
          '<td>' +
            (b.versioning_enabled ? '<span class="pill sky">Versioning</span> ' : '<span class="pill">Single-Version</span> ') +
            (b.public_read ? '<span class="pill ok">Public Read</span> ' : '<span class="pill">Private</span> ') +
            '<span class="pill">' + b.retention_days + 'd retention</span>' +
          '</td>' +
          '<td>' + formatBytes(b.logical_bytes) + ' / ' + b.quota_mb + ' MB</td>' +
          '<td>' + b.object_count + ' keys (' + b.version_count + ' versions)</td>' +
          '<td>' +
            '<button class="btn btn-sm btn-secondary" onclick="inspectBucket(\'' + escapeHtml(b.name) + '\')">Explore</button> ' +
            '<button class="btn btn-sm btn-danger" onclick="deleteBucket(\'' + escapeHtml(b.name) + '\')">Delete</button>' +
          '</td>' +
        '</tr>';
      });

      overviewEl.innerHTML = ovHtml;
      tbody.innerHTML = rowsHtml;
    }

    function inspectBucket(name) {
      document.getElementById('explorerBucketSelect').value = name;
      switchTab('explorer', document.querySelectorAll('.nav-btn')[2]);
    }

    async function submitBucketForm(e) {
      e.preventDefault();
      try {
        const payload = {
          name: document.getElementById('bucketName').value.trim(),
          quota_mb: Number(document.getElementById('bucketQuota').value),
          retention_days: Number(document.getElementById('bucketRetention').value),
          versioning_enabled: document.getElementById('bucketVersioning').checked,
          public_read: document.getElementById('bucketPublicRead').checked
        };
        await apiFetch('/api/v1/buckets', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        showToast('Bucket policy saved for ' + payload.name);
        document.getElementById('bucketName').value = '';
        await refreshAll();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function deleteBucket(name) {
      try {
        await apiFetch('/api/v1/buckets/' + encodeURIComponent(name), { method: 'DELETE' });
        showToast('Deleted bucket ' + name);
        await refreshAll();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function loadObjects() {
      const bucket = document.getElementById('explorerBucketSelect').value;
      const tbody = document.getElementById('objectsTableBody');
      if (!bucket) {
        tbody.innerHTML = '<tr><td colspan="8" style="color:var(--text-muted);">Select a bucket first.</td></tr>';
        return;
      }
      const prefix = document.getElementById('explorerPrefix').value.trim();
      const includeVersions = document.getElementById('explorerIncludeVersions').checked;
      try {
        const q = new URLSearchParams({
          prefix: prefix,
          versions: includeVersions ? 'true' : 'false'
        });
        const data = await apiFetch('/api/v1/buckets/' + encodeURIComponent(bucket) + '/objects?' + q.toString());
        const objs = data.objects || [];
        if (objs.length === 0) {
          tbody.innerHTML = '<tr><td colspan="8" style="color:var(--text-muted);">No object versions match in bucket <strong>' + escapeHtml(bucket) + '</strong>.</td></tr>';
          return;
        }
        tbody.innerHTML = objs.map(o => {
          const tagsStr = o.tags ? Object.entries(o.tags).map(([k,v]) => k + '=' + v).join(', ') : '';
          const dlUrl = '/s3/' + encodeURIComponent(o.bucket) + '/' + o.key.split('/').map(encodeURIComponent).join('/') + '?versionId=' + encodeURIComponent(o.version_id);
          return '<tr>' +
            '<td><strong>' + escapeHtml(o.key) + '</strong></td>' +
            '<td><code>' + escapeHtml(o.version_id) + '</code></td>' +
            '<td>' +
              (o.is_delete_marker ? '<span class="pill danger">Delete Marker</span>' :
                (o.is_latest ? '<span class="pill ok">Latest</span>' : '<span class="pill">Historical</span>')) +
              (o.deduplicated ? ' <span class="pill sky">CAS Dedup</span>' : '') +
            '</td>' +
            '<td><code>' + (o.sha256 ? escapeHtml(o.sha256.substring(0, 16)) + '…' : '—') + '</code></td>' +
            '<td>' + (o.is_delete_marker ? '0 B' : formatBytes(o.size)) + '</td>' +
            '<td>' + (tagsStr ? '<code>' + escapeHtml(tagsStr) + '</code>' : '—') + '</td>' +
            '<td>' + escapeHtml(new Date(o.created_at).toISOString().replace('T', ' ').substring(0, 19)) + '</td>' +
            '<td>' +
              (!o.is_delete_marker ? '<a class="btn btn-sm btn-secondary" href="' + dlUrl + '" target="_blank">Download</a> ' : '') +
              (!o.is_delete_marker ? '<button class="btn btn-sm btn-secondary" onclick="quickPresign(\'' + escapeHtml(o.bucket) + '\',\'' + escapeHtml(o.key) + '\',\'' + escapeHtml(o.version_id) + '\')">Presign</button> ' : '') +
              '<button class="btn btn-sm btn-danger" onclick="deleteObjectVersion(\'' + escapeHtml(o.bucket) + '\',\'' + escapeHtml(o.key) + '\',\'' + escapeHtml(o.version_id) + '\')">Delete</button>' +
            '</td>' +
          '</tr>';
        }).join('');
      } catch (err) {
        showToast(err.message, true);
      }
    }

    function onFilePicked() {
      const f = document.getElementById('uploadFileInput').files[0];
      if (!f) return;
      if (!document.getElementById('uploadKey').value.trim()) {
        document.getElementById('uploadKey').value = 'uploads/' + f.name;
      }
      if (f.type) {
        document.getElementById('uploadContentType').value = f.type;
      }
    }

    async function submitUploadForm(e) {
      e.preventDefault();
      try {
        const bucket = document.getElementById('uploadBucketSelect').value;
        const key = document.getElementById('uploadKey').value.trim();
        const contentType = document.getElementById('uploadContentType').value.trim() || 'application/octet-stream';
        const tags = document.getElementById('uploadTags').value.trim();
        const fileInput = document.getElementById('uploadFileInput');
        let bodyPayload;
        if (fileInput.files && fileInput.files.length > 0) {
          bodyPayload = fileInput.files[0];
        } else {
          bodyPayload = document.getElementById('uploadTextContent').value;
        }

        const q = new URLSearchParams({ key: key, tags: tags });
        const res = await apiFetch('/api/v1/buckets/' + encodeURIComponent(bucket) + '/objects?' + q.toString(), {
          method: 'POST',
          headers: { 'Content-Type': contentType },
          body: bodyPayload
        });

        const v = res.object;
        document.getElementById('uploadResultBox').innerHTML =
          '<div style="display:flex;flex-direction:column;gap:0.45rem;">' +
            '<div><span class="pill ok">STORED</span> ' +
              (v.deduplicated ? '<span class="pill sky">CAS DEDUPLICATED (0 Extra Disk Bytes!)</span>' : '<span class="pill">New CAS Physical Blob Written</span>') +
            '</div>' +
            '<div><strong>Bucket / Key:</strong> <code>' + escapeHtml(v.bucket + '/' + v.key) + '</code></div>' +
            '<div><strong>Assigned Version ID:</strong> <code>' + escapeHtml(v.version_id) + '</code></div>' +
            '<div><strong>SHA-256 Digest:</strong> <code>' + escapeHtml(v.sha256) + '</code></div>' +
            '<div><strong>Payload Size:</strong> ' + formatBytes(v.size) + ' (' + escapeHtml(v.content_type) + ')</div>' +
          '</div>';
        fileInput.value = '';
        showToast('Uploaded ' + v.bucket + '/' + v.key + ' (' + (v.deduplicated ? 'CAS Dedup Hit' : 'New Blob') + ')');
        await refreshAll();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function deleteObjectVersion(bucket, key, versionId) {
      try {
        const q = new URLSearchParams({ key: key });
        if (versionId) q.set('versionId', versionId);
        await apiFetch('/api/v1/buckets/' + encodeURIComponent(bucket) + '/objects?' + q.toString(), {
          method: 'DELETE'
        });
        showToast('Deleted ' + key + (versionId ? ' (' + versionId + ')' : ''));
        await refreshAll();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function quickPresign(bucket, key, versionId) {
      document.getElementById('presignBucketSelect').value = bucket;
      document.getElementById('presignKey').value = key;
      document.getElementById('presignVersionId').value = versionId || '';
      document.getElementById('presignMethod').value = 'GET';
      switchTab('presign', document.querySelectorAll('.nav-btn')[4]);
      await generatePresignUrl();
    }

    async function submitPresignForm(e) {
      e.preventDefault();
      await generatePresignUrl();
    }

    async function generatePresignUrl() {
      try {
        const payload = {
          method: document.getElementById('presignMethod').value,
          bucket: document.getElementById('presignBucketSelect').value,
          key: document.getElementById('presignKey').value.trim(),
          version_id: document.getElementById('presignVersionId').value.trim(),
          ttl_seconds: Number(document.getElementById('presignTTL').value),
          base_url: window.location.origin
        };
        const res = await apiFetch('/api/v1/presign', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        document.getElementById('presignResultBox').innerHTML =
          '<div style="display:flex;flex-direction:column;gap:0.55rem;">' +
            '<div><span class="pill sky">HMAC-SHA256 Signed</span> <span class="pill">Method: ' + escapeHtml(res.method) + '</span></div>' +
            '<div><strong>Expires At (UTC):</strong> <code>' + escapeHtml(res.expires_at) + '</code></div>' +
            '<div><strong>Signature:</strong> <code>' + escapeHtml(res.signature) + '</code></div>' +
            '<div><strong>Presigned URL:</strong><br/><input class="input" style="margin-top:0.35rem;" readonly value="' + escapeHtml(res.url) + '" onclick="this.select()" /></div>' +
            (res.method === 'GET' ? '<div><a class="btn btn-sm" href="' + escapeHtml(res.path) + '" target="_blank">🔗 Open Presigned URL in New Tab</a></div>' : '') +
          '</div>';
        showToast('Generated expiring HMAC-SHA256 presigned URL');
      } catch (err) {
        showToast(err.message, true);
      }
    }

    function renderScrubSummary(rep) {
      const badge = rep.healthy
        ? '<span class="pill ok">✓ HEALTHY (0 Bit-Rot Corruptions)</span>'
        : '<span class="pill danger">⚠ BIT-ROT DETECTED (' + rep.corrupted_count + ' Corrupted Blobs)</span>';
      const html =
        '<div style="display:flex;flex-direction:column;gap:0.45rem;">' +
          '<div>' + badge + '</div>' +
          '<div><strong>Verified CAS Blobs:</strong> ' + rep.verified_blobs + ' / ' + rep.total_blobs + ' (' + formatBytes(rep.total_bytes_verified) + ' verified in ' + rep.duration_ms + ' ms)</div>' +
          '<div><strong>Scrubbed At:</strong> <code>' + escapeHtml(rep.scrubbed_at) + '</code></div>' +
          (rep.corrupted_count > 0 ? '<div style="color:var(--danger);"><strong>Corrupted Digests:</strong> <code>' + escapeHtml(JSON.stringify(rep.corrupted_blobs)) + '</code></div>' : '') +
        '</div>';
      document.getElementById('overviewScrubBox').innerHTML = html;
      document.getElementById('scrubReportBox').innerHTML = html;
    }

    async function triggerScrub() {
      try {
        const rep = await apiFetch('/api/v1/scrub', { method: 'POST' });
        renderScrubSummary(rep);
        showToast(rep.healthy ? 'CAS Scrub Passed: ' + rep.verified_blobs + ' blobs verified!' : 'CAS Scrub detected bit-rot!', !rep.healthy);
        await loadAudits();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function exportBackup() {
      try {
        const snap = await apiFetch('/api/v1/admin/backup');
        document.getElementById('backupJsonArea').value = JSON.stringify(snap, null, 2);
        showToast('Exported catalog metadata backup snapshot');
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function restoreBackup() {
      try {
        const raw = document.getElementById('backupJsonArea').value.trim();
        if (!raw) throw new Error('Paste or export a JSON snapshot first');
        const parsed = JSON.parse(raw);
        await apiFetch('/api/v1/admin/restore', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(parsed)
        });
        showToast('Restored catalog snapshot successfully');
        await refreshAll();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function loadAudits() {
      try {
        const data = await apiFetch('/api/v1/audits');
        const audits = data.audits || [];
        const tbody = document.getElementById('auditTableBody');
        if (audits.length === 0) {
          tbody.innerHTML = '<tr><td colspan="7" style="color:var(--text-muted);">No audit events recorded yet.</td></tr>';
          return;
        }
        tbody.innerHTML = audits.map(a => {
          const pillClass = a.status === 'SUCCESS' ? 'ok' : 'danger';
          return '<tr>' +
            '<td><code>' + escapeHtml(new Date(a.timestamp).toISOString().replace('T', ' ').substring(0, 19)) + '</code></td>' +
            '<td><strong>' + escapeHtml(a.action) + '</strong></td>' +
            '<td>' + escapeHtml(a.bucket || '—') + '</td>' +
            '<td><code>' + escapeHtml((a.key || '—') + (a.version_id ? ' @ ' + a.version_id : '')) + '</code></td>' +
            '<td>' + escapeHtml(a.actor) + '</td>' +
            '<td><span class="pill ' + pillClass + '">' + escapeHtml(a.status) + '</span></td>' +
            '<td>' + escapeHtml(a.details || '') + '</td>' +
          '</tr>';
        }).join('');
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function refreshAll() {
      try {
        await loadMetricsAndBuckets();
        await loadObjects();
        await loadAudits();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    window.addEventListener('DOMContentLoaded', () => {
      const savedTheme = localStorage.getItem('nimbus_theme');
      if (savedTheme) document.documentElement.setAttribute('data-theme', savedTheme);
      const savedToken = localStorage.getItem('nimbus_token');
      if (savedToken) document.getElementById('apiTokenInput').value = savedToken;
      refreshAll();
    });
  </script>
</body>
</html>`
