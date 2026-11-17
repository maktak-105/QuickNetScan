// QuickNetScan Frontend JavaScript

const i18nData = {
  ja: {
    subtitle: "高速LANスキャナー (Go+Goroutines)",
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
    status_ready: "待機中",
    status_scanning: "スキャン中...",
    status_stopped: "停止しました",
    status_complete: "スキャン完了",
    found_count: "検出:",
    progress_count: "進捗:",
    elapsed: "経過時間:",
    about_title: "バージョン情報",
    about_desc: "自宅LANや社内LANのIPレンジを、Goの得意な並行処理（Goroutine）で数百スレッド同時に一瞬でスキャンし、生きている機器のIP・MACアドレス・空いているポート・応答速度を即座に一覧表示する単一EXE。",
    ok: "OK"
  },
  en: {
    subtitle: "High-speed LAN Scanner (Go+Goroutines)",
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
    status_ready: "Ready",
    status_scanning: "Scanning...",
    status_stopped: "Stopped",
    status_complete: "Scan Completed",
    found_count: "Found:",
    progress_count: "Progress:",
    elapsed: "Elapsed:",
    about_title: "About QuickNetScan",
    about_desc: "A single-binary Windows utility that instantly scans local IP ranges (home or office LAN) using highly concurrent goroutines across hundreds of targets, listing active devices, IP/MAC addresses, open ports, and latency in real time.",
    ok: "OK"
  }
};

let currentLang = "ja";
let eventSource = null;
let scanStartTime = null;
let timerInterval = null;
let foundHosts = [];

let isWasmReady = false;

document.addEventListener("DOMContentLoaded", () => {
  setupLanguage();
  setupUIEvents();
  initWasm();
  setupNativeBridge();
});

window.onWasmReady = () => {
  isWasmReady = true;
  console.log("Go WASM GUI engine ready!");
};

async function initWasm() {
  if (typeof Go === "undefined" || typeof GO_WASM_BASE64 === "undefined") {
    console.warn("Go WASM runtime or embedded binary not found.");
    return;
  }
  try {
    const binaryString = atob(GO_WASM_BASE64);
    const bytes = new Uint8Array(binaryString.length);
    for (let i = 0; i < binaryString.length; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }
    const go = new Go();
    const result = await WebAssembly.instantiate(bytes.buffer, go.importObject);
    go.run(result.instance);
    console.log("Go WASM loaded directly from embedded memory (No HTTP server required).");
  } catch (err) {
    console.error("Failed to initialize Go WASM from memory:", err);
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
    // Browser fallback
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

    case "progress":
      handleProgressUpdate(msg.data);
      break;

    case "scan_complete":
      completeScan(false);
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
  const select = document.getElementById("subnetSelect");
  const targetCidr = document.getElementById("targetCidr");

  try {
    const res = await fetch("/api/local-subnets");
    if (res.ok) {
      const data = await res.json();
      select.innerHTML = "";

      if (data.subnets && data.subnets.length > 0) {
        data.subnets.forEach((sub, idx) => {
          const opt = document.createElement("option");
          opt.value = sub.cidr;
          opt.textContent = sub.label;
          if (idx === 0) {
            opt.selected = true;
            targetCidr.value = sub.cidr;
          }
          select.appendChild(opt);
        });

        // Add custom option
        const customOpt = document.createElement("option");
        customOpt.value = "";
        customOpt.textContent = i18nData[currentLang].custom_option;
        select.appendChild(customOpt);
      } else {
        select.innerHTML = `<option value="">${i18nData[currentLang].custom_option}</option>`;
      }
    }
  } catch (err) {
    console.warn("Could not auto-detect subnet:", err);
  }
}

function populateSubnets(subnets) {
  const select = document.getElementById("subnetSelect");
  const targetCidr = document.getElementById("targetCidr");
  select.innerHTML = "";

  if (subnets && subnets.length > 0) {
    subnets.forEach((sub, idx) => {
      const opt = document.createElement("option");
      opt.value = sub.cidr;
      opt.textContent = sub.label;
      if (idx === 0) {
        opt.selected = true;
        targetCidr.value = sub.cidr;
      }
      select.appendChild(opt);
    });

    const customOpt = document.createElement("option");
    customOpt.value = "";
    customOpt.textContent = i18nData[currentLang].custom_option;
    select.appendChild(customOpt);
  }
}

async function startScan() {
  const targetCidr = document.getElementById("targetCidr").value.trim();
  const concurrency = parseInt(document.getElementById("concurrencyRange").value, 10);
  const scanPorts = document.getElementById("scanPortsCheckbox").checked;
  const ports = document.getElementById("portListInput").value.trim();

  if (!targetCidr) {
    alert("スキャン対象のCIDRを入力してください。");
    return;
  }

  // Reset table & stats
  foundHosts = [];
  const tbody = document.getElementById("resultsBody");
  tbody.innerHTML = "";
  document.getElementById("foundCount").textContent = "0";
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

  // HTTP Fallback (if any)
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
      alert("スキャン開始エラー: " + err);
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
    try { await fetch("/api/scan/stop"); } catch (_) {}
  }
  completeScan(true);
}

function handleProgressUpdate(update) {
  if (!update) return;

  // Let Go WASM update the progress bar and counter
  if (window.wasmUpdateProgress) {
    try {
      window.wasmUpdateProgress(update.scanned_count, update.total_targets);
    } catch (_) {}
  } else {
    if (update.total_targets > 0) {
      const percent = Math.min(100, Math.round((update.scanned_count / update.total_targets) * 100));
      document.getElementById("progressBar").style.width = percent + "%";
      document.getElementById("progressCount").textContent = `${update.scanned_count} / ${update.total_targets}`;
    }
  }

  document.getElementById("foundCount").textContent = update.found_count;

  // Let Go WASM render the new host row in DOM
  if (update.latest_host && update.latest_host.is_alive) {
    foundHosts.push(update.latest_host);
    if (window.wasmAddHostRow) {
      try {
        window.wasmAddHostRow(update.latest_host);
      } catch (err) {
        addHostRow(update.latest_host);
      }
    } else {
      addHostRow(update.latest_host);
    }
  }

  if (update.is_complete) {
    completeScan(false);
  }
}

function addHostRow(host) {
  foundHosts.push(host);
  const tbody = document.getElementById("resultsBody");
  const tr = document.createElement("tr");

  const portsHtml = (host.open_ports && host.open_ports.length > 0)
    ? host.open_ports.map(p => `<span class="port-tag">${p}</span>`).join("")
    : `<span style="color: #666;">-</span>`;

  // WASM formatter hook if available
  let rttStr = `${host.rtt_ms.toFixed(1)} ms`;
  if (window.wasmFormatRTT) {
    try {
      rttStr = window.wasmFormatRTT(host.rtt_ms);
    } catch (_) {}
  }

  tr.innerHTML = `
    <td><span class="status-badge">ONLINE</span></td>
    <td><strong>${host.ip}</strong></td>
    <td><code>${host.mac || "-"}</code></td>
    <td>${portsHtml}</td>
    <td>${rttStr}</td>
  `;
  tbody.appendChild(tr);
}

async function stopScan() {
  try {
    await fetch("/api/scan/stop");
  } catch (_) {}
  completeScan(true);
}

function completeScan(stopped = false) {
  setScanningState(false);
  if (timerInterval) {
    clearInterval(timerInterval);
    timerInterval = null;
  }
  document.getElementById("scanStatusText").textContent = stopped
    ? i18nData[currentLang].status_stopped
    : i18nData[currentLang].status_complete;
}

function setScanningState(isScanning) {
  document.getElementById("startScanBtn").disabled = isScanning;
  document.getElementById("stopScanBtn").disabled = !isScanning;
  document.getElementById("detectSubnetBtn").disabled = isScanning;
}

function updateTimer() {
  if (!scanStartTime) return;
  const elapsedSec = ((Date.now() - scanStartTime) / 1000).toFixed(1);
  document.getElementById("elapsedTime").textContent = `${elapsedSec}s`;
}

function exportCSV() {
  if (foundHosts.length === 0) {
    alert("エクスポートするスキャン結果がありません。");
    return;
  }

  let csv = "IP,MAC,OpenPorts,RTT_ms\r\n";
  foundHosts.forEach(h => {
    const ports = (h.open_ports || []).join(";");
    csv += `"${h.ip}","${h.mac || ""}","${ports}",${h.rtt_ms}\r\n`;
  });

  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `quicknetscan_results_${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}
