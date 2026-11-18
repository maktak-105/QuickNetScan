// QuickNetScan Frontend JavaScript

const i18nData = {
  ja: {
    subtitle: "高速LANスキャナー",
    lang_btn_text: "🌐 English",
    export: "エクスポート",
    target_network: "接続先ネットワーク:",
    target_cidr: "スキャンCIDR:",
    concurrency_label: "並行数",
    detecting: "検出中...",
    custom_option: "手動指定 / カスタム",
    enable_port_scan: "ポートスキャンを実行",
    start_scan: "スキャン開始",
    stop_scan: "停止",
    col_status: "状態",
    col_ip: "IP アドレス",
    col_mac: "MAC アドレス",
    col_ports: "開放ポート",
    col_rtt: "応答速度",
    empty_state: "スキャンを開始すると検出された機器がここに表示されます",
    no_hosts_found: "応答のある機器は検出されませんでした",
    status_ready: "待機中",
    status_scanning: "スキャン中...",
    status_stopped: "停止しました",
    status_complete: "スキャン完了",
    found_count: "検出:",
    found_unit: "台",
    progress_count: "進捗:",
    elapsed: "経過時間:",
    about_title: "バージョン情報",
    about_desc: "自宅LANや社内LANのIPレンジを、Goの得意な並行処理（Goroutine）で数百スレッド同時に一瞬でスキャンし、生きている機器のIP・MACアドレス・空いているポート・応答速度を即座に一覧表示する単一EXE。",
    ok: "OK",
    err_empty_cidr: "スキャン対象のCIDRまたはIP範囲を入力してください。",
    err_invalid_cidr: "無効なCIDRまたはIP範囲です: ",
    err_no_export_data: "エクスポートするスキャン結果がありません。",
    export_saved: "CSVファイルを保存しました:\n"
  },
  en: {
    subtitle: "High-speed LAN Scanner",
    lang_btn_text: "🌐 日本語",
    export: "Export",
    target_network: "Network Adapter:",
    target_cidr: "Target CIDR:",
    concurrency_label: "Concurrency",
    detecting: "Detecting...",
    custom_option: "Custom Range / Manual",
    enable_port_scan: "Enable Port Scan",
    start_scan: "Start Scan",
    stop_scan: "Stop",
    col_status: "Status",
    col_ip: "IP Address",
    col_mac: "MAC Address",
    col_ports: "Open Ports",
    col_rtt: "Latency",
    empty_state: "Discovered active hosts will be displayed here once scanning begins",
    no_hosts_found: "No active hosts were found in the specified range",
    status_ready: "Ready",
    status_scanning: "Scanning...",
    status_stopped: "Stopped",
    status_complete: "Scan Completed",
    found_count: "Found:",
    found_unit: "hosts",
    progress_count: "Progress:",
    elapsed: "Elapsed:",
    about_title: "About QuickNetScan",
    about_desc: "A single-binary Windows utility that instantly scans local IP ranges (home or office LAN) using highly concurrent goroutines across hundreds of targets, listing active devices, IP/MAC addresses, open ports, and latency in real time.",
    ok: "OK",
    err_empty_cidr: "Please enter a target CIDR or IP range.",
    err_invalid_cidr: "Invalid CIDR or IP range: ",
    err_no_export_data: "No scan results to export.",
    export_saved: "Saved CSV file:\n"
  }
};

let currentLang = "ja";
let eventSource = null;
let scanStartTime = null;
let timerInterval = null;
let foundHosts = [];
let foundHostMap = new Map();
let isScanningNow = false;
let isWasmReady = false;

document.addEventListener("DOMContentLoaded", () => {
  setupLanguage();
  setupUIEvents();
  setupNativeBridge();
  initWasm();
});

window.onWasmReady = () => {
  isWasmReady = true;
  window.isWasmReady = true;
  console.log("Go WASM GUI engine ready!");
};

async function initWasm() {
  if (typeof Go === "undefined") {
    console.warn("Go WASM runtime not found.");
    return;
  }
  try {
    const go = new Go();
    const wasmUrl = (location.protocol === "http:" || location.protocol === "https:")
      ? "app.wasm"
      : "https://quicknetscan.local/app.wasm";
    const resp = await fetch(wasmUrl);
    const bytes = await resp.arrayBuffer();
    const result = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(result.instance);
    console.log("Go WASM loaded via fetch + arrayBuffer.");
  } catch (err) {
    console.error("Failed to initialize Go WASM:", err);
  }
}

function setupNativeBridge() {
  if (window.chrome && window.chrome.webview) {
    window.chrome.webview.addEventListener("message", (event) => {
      handleNativeMessage(event.data);
    });
    // Request subnets immediately
    window.chrome.webview.postMessage({ cmd: "get_subnets" });
  } else {
    // Browser HTTP fallback
    detectLocalSubnet();
  }
}

function handleNativeMessage(data) {
  let msg = data;
  if (typeof msg === "string") {
    try { msg = JSON.parse(msg); } catch (_) {}
  }
  if (!msg) return;

  switch (msg.cmd) {
    case "subnets_result":
      populateSubnets(msg.subnets);
      break;

    case "scan_started":
      if (msg.total_targets) {
        document.getElementById("progressCount").textContent = `0 / ${msg.total_targets}`;
      }
      break;

    case "scan_error":
      alert(i18nData[currentLang].err_invalid_cidr + (msg.error || ""));
      completeScan(true);
      break;

    case "progress":
      handleProgressUpdate(msg.data);
      break;

    case "scan_complete":
      completeScan(false);
      break;

    case "export_result":
      if (msg.success && msg.path) {
        alert(i18nData[currentLang].export_saved + msg.path);
      }
      break;
  }
}

function setupLanguage() {
  const toggleBtn = document.getElementById("langToggleBtn");
  toggleBtn.addEventListener("click", () => {
    currentLang = currentLang === "ja" ? "en" : "ja";
    applyLanguage(currentLang);
  });
  applyLanguage(currentLang);
}

function applyLanguage(lang) {
  const texts = i18nData[lang];
  document.getElementById("currentLangLabel").textContent = texts.lang_btn_text;

  document.querySelectorAll("[data-i18n]").forEach(el => {
    const key = el.getAttribute("data-i18n");
    if (texts[key]) {
      el.textContent = texts[key];
    }
  });

  // Update status text according to current state
  const statusEl = document.getElementById("scanStatusText");
  if (statusEl) {
    if (isScanningNow) {
      statusEl.textContent = texts.status_scanning;
    }
  }
}

function setupUIEvents() {
  const concurrencyRange = document.getElementById("concurrencyRange");
  const concurrencyBadge = document.getElementById("concurrencyBadge");
  concurrencyRange.addEventListener("input", (e) => {
    concurrencyBadge.textContent = e.target.value;
  });

  const subnetSelect = document.getElementById("subnetSelect");
  const targetCidr = document.getElementById("targetCidr");
  subnetSelect.addEventListener("change", (e) => {
    if (e.target.value) {
      targetCidr.value = e.target.value;
    }
  });

  document.getElementById("startScanBtn").addEventListener("click", startScan);
  document.getElementById("stopScanBtn").addEventListener("click", stopScan);
  document.getElementById("exportBtn").addEventListener("click", exportCSV);

  // Enter key on CIDR input triggers scan
  targetCidr.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !isScanningNow) {
      startScan();
    }
  });

  // About modal
  const modal = document.getElementById("aboutModal");
  document.getElementById("aboutBtn").addEventListener("click", () => {
    modal.classList.remove("hidden");
  });
  document.getElementById("closeModalBtn").addEventListener("click", () => {
    modal.classList.add("hidden");
  });
  document.getElementById("aboutOkBtn").addEventListener("click", () => {
    modal.classList.add("hidden");
  });
  modal.addEventListener("click", (e) => {
    if (e.target === modal) modal.classList.add("hidden");
  });
}

async function detectLocalSubnet() {
  try {
    const res = await fetch("/api/local-subnets");
    if (res.ok) {
      const data = await res.json();
      populateSubnets(data.subnets || []);
    }
  } catch (err) {
    console.warn("Could not auto-detect subnet:", err);
  }
}

function populateSubnets(subnets) {
  const select = document.getElementById("subnetSelect");
  const targetCidr = document.getElementById("targetCidr");
  if (!select) return;
  select.innerHTML = "";

  if (subnets && subnets.length > 0) {
    subnets.forEach((sub, idx) => {
      const opt = document.createElement("option");
      opt.value = sub.cidr;
      opt.textContent = sub.label;
      if (idx === 0) {
        opt.selected = true;
        if (targetCidr) targetCidr.value = sub.cidr;
      }
      select.appendChild(opt);
    });

    const customOpt = document.createElement("option");
    customOpt.value = "";
    customOpt.textContent = i18nData[currentLang].custom_option;
    select.appendChild(customOpt);
  } else {
    select.innerHTML = `<option value="">${i18nData[currentLang].custom_option}</option>`;
  }
}

function ipToNumber(ip) {
  if (!ip) return 0;
  const parts = ip.split(".");
  if (parts.length !== 4) return 0;
  return (((parseInt(parts[0], 10) & 255) << 24) |
          ((parseInt(parts[1], 10) & 255) << 16) |
          ((parseInt(parts[2], 10) & 255) << 8) |
          (parseInt(parts[3], 10) & 255)) >>> 0;
}

async function startScan() {
  const targetCidr = document.getElementById("targetCidr").value.trim();
  const concurrency = parseInt(document.getElementById("concurrencyRange").value, 10) || 256;
  const scanPorts = document.getElementById("scanPortsCheckbox").checked;
  const ports = document.getElementById("portListInput").value.trim();

  if (!targetCidr) {
    alert(i18nData[currentLang].err_empty_cidr);
    return;
  }

  // Reset table & stats
  foundHosts = [];
  foundHostMap.clear();
  const tbody = document.getElementById("resultsBody");
  tbody.innerHTML = "";
  document.getElementById("foundCount").textContent = "0";
  document.getElementById("progressCount").textContent = "0 / 0";
  document.getElementById("elapsedTime").textContent = "0.0s";
  document.getElementById("progressBar").style.width = "0%";
  document.getElementById("scanStatusText").textContent = i18nData[currentLang].status_scanning;

  setScanningState(true);
  scanStartTime = Date.now();
  if (timerInterval) clearInterval(timerInterval);
  timerInterval = setInterval(updateTimer, 100);

  // Send to native WebView2 if available
  if (window.chrome && window.chrome.webview) {
    window.chrome.webview.postMessage({
      cmd: "start_scan",
      target_cidr: targetCidr,
      concurrency: concurrency,
      scan_ports: scanPorts,
      ports: ports
    });
    return;
  }

  // HTTP Fallback (SSE + POST)
  if (eventSource) {
    eventSource.close();
  }
  eventSource = new EventSource("/api/scan/events");
  eventSource.onmessage = (e) => {
    try {
      const update = JSON.parse(e.data);
      handleProgressUpdate(update);
    } catch (_) {}
  };

  try {
    const res = await fetch("/api/scan/start", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        target_cidr: targetCidr,
        concurrency: concurrency,
        scan_ports: scanPorts,
        ports: ports
      })
    });
    if (!res.ok) {
      const err = await res.text();
      alert(i18nData[currentLang].err_invalid_cidr + err);
      stopScan();
      return;
    }
    const data = await res.json();
    if (data.total_targets) {
      document.getElementById("progressCount").textContent = `0 / ${data.total_targets}`;
    }
  } catch (err) {
    alert("通信エラー: " + err);
    stopScan();
  }
}

async function stopScan() {
  if (window.chrome && window.chrome.webview) {
    window.chrome.webview.postMessage({ cmd: "stop_scan" });
  } else {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    try { await fetch("/api/scan/stop"); } catch (_) {}
  }
  completeScan(true);
}

function handleProgressUpdate(update) {
  if (!update || !isScanningNow) return;

  // Let Go WASM update the progress bar and counter
  if (window.wasmUpdateProgress) {
    try {
      window.wasmUpdateProgress(update.scanned_count, update.total_targets);
    } catch (_) {
      updateProgressDOM(update.scanned_count, update.total_targets);
    }
  } else {
    updateProgressDOM(update.scanned_count, update.total_targets);
  }

  // Let Go WASM or JS render the new host row in DOM
  if (update.latest_host && update.latest_host.is_alive) {
    const host = update.latest_host;
    if (!foundHostMap.has(host.ip)) {
      foundHostMap.set(host.ip, host);
      foundHosts.push(host);
    } else {
      foundHostMap.set(host.ip, host);
    }

    if (window.wasmAddHostRow) {
      try {
        window.wasmAddHostRow(host);
      } catch (_) {
        addHostRowDOM(host);
      }
    } else {
      addHostRowDOM(host);
    }
  }

  document.getElementById("foundCount").textContent = String(foundHostMap.size || update.found_count || 0);

  if (update.is_complete) {
    completeScan(false);
  }
}

function updateProgressDOM(scanned, total) {
  if (total > 0) {
    const percent = Math.min(100, Math.round((scanned / total) * 100));
    document.getElementById("progressBar").style.width = percent + "%";
    document.getElementById("progressCount").textContent = `${scanned} / ${total}`;
  }
}

function addHostRowDOM(host) {
  const tbody = document.getElementById("resultsBody");
  const ipNum = ipToNumber(host.ip);
  const tr = document.createElement("tr");
  tr.setAttribute("data-ip", host.ip);
  tr.setAttribute("data-ip-num", String(ipNum));

  const portsHtml = (host.open_ports && host.open_ports.length > 0)
    ? host.open_ports.map(p => `<span class="port-tag">${p}</span>`).join("")
    : `<span style="color: #666;">-</span>`;

  let rttStr = `${(host.rtt_ms || 0).toFixed(1)} ms`;
  if (window.wasmFormatRTT) {
    try {
      rttStr = window.wasmFormatRTT(host.rtt_ms || 0);
    } catch (_) {}
  }

  tr.innerHTML = `
    <td><span class="status-badge">ONLINE</span></td>
    <td><strong>${host.ip}</strong></td>
    <td><code>${host.mac || "-"}</code></td>
    <td>${portsHtml}</td>
    <td>${rttStr}</td>
  `;

  // Insert in ascending IPv4 order
  const rows = tbody.children;
  let inserted = false;
  for (let i = 0; i < rows.length; i++) {
    const row = rows[i];
    if (row.getAttribute("data-ip") === host.ip) {
      tbody.replaceChild(tr, row);
      inserted = true;
      break;
    }
    const rowIpNum = parseInt(row.getAttribute("data-ip-num") || "0", 10);
    if (rowIpNum > ipNum) {
      tbody.insertBefore(tr, row);
      inserted = true;
      break;
    }
  }
  if (!inserted) {
    tbody.appendChild(tr);
  }
}

function completeScan(stopped = false) {
  setScanningState(false);
  if (timerInterval) {
    clearInterval(timerInterval);
    timerInterval = null;
  }
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  updateTimer();

  document.getElementById("scanStatusText").textContent = stopped
    ? i18nData[currentLang].status_stopped
    : i18nData[currentLang].status_complete;

  const tbody = document.getElementById("resultsBody");
  if (tbody && tbody.children.length === 0) {
    const emptyTr = document.createElement("tr");
    emptyTr.className = "empty-row";
    emptyTr.innerHTML = `<td colspan="5" data-i18n="no_hosts_found">${i18nData[currentLang].no_hosts_found}</td>`;
    tbody.appendChild(emptyTr);
  }
}

function setScanningState(isScanning) {
  isScanningNow = isScanning;
  const startBtn = document.getElementById("startScanBtn");
  const stopBtn = document.getElementById("stopScanBtn");
  const subnetSelect = document.getElementById("subnetSelect");
  const targetCidr = document.getElementById("targetCidr");
  const concurrencyRange = document.getElementById("concurrencyRange");
  const scanPortsCheckbox = document.getElementById("scanPortsCheckbox");
  const portListInput = document.getElementById("portListInput");

  if (startBtn) startBtn.disabled = isScanning;
  if (stopBtn) stopBtn.disabled = !isScanning;
  if (subnetSelect) subnetSelect.disabled = isScanning;
  if (targetCidr) targetCidr.disabled = isScanning;
  if (concurrencyRange) concurrencyRange.disabled = isScanning;
  if (scanPortsCheckbox) scanPortsCheckbox.disabled = isScanning;
  if (portListInput) portListInput.disabled = isScanning;
}

function updateTimer() {
  if (!scanStartTime) return;
  const elapsedSec = ((Date.now() - scanStartTime) / 1000).toFixed(1);
  document.getElementById("elapsedTime").textContent = `${elapsedSec}s`;
}

function exportCSV() {
  if (foundHostMap.size === 0) {
    alert(i18nData[currentLang].err_no_export_data);
    return;
  }

  const sortedHosts = Array.from(foundHostMap.values()).sort(
    (a, b) => ipToNumber(a.ip) - ipToNumber(b.ip)
  );

  let csv = "IP,MAC,OpenPorts,RTT_ms\r\n";
  sortedHosts.forEach(h => {
    const ports = (h.open_ports || []).join(";");
    const rtt = typeof h.rtt_ms === "number" ? h.rtt_ms.toFixed(2) : "0.00";
    csv += `"${h.ip}","${h.mac || ""}","${ports}",${rtt}\r\n`;
  });

  if (window.chrome && window.chrome.webview) {
    window.chrome.webview.postMessage({
      cmd: "export_csv",
      csv: csv
    });
    return;
  }

  const blob = new Blob(["\uFEFF" + csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `quicknetscan_results_${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}
