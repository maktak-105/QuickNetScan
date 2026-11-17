// QuickNetScan Frontend JavaScript

const i18nData = {
  ja: {
    subtitle: "高速LANスキャナー (Go+Goroutines)",
    lang_label: "日本語",
    export: "エクスポート",
    target_range: "スキャン対象 (CIDR / 範囲):",
    auto_detect: "自サブネット検出",
    concurrency: "並行 Goroutine 数:",
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
    lang_label: "English",
    export: "Export",
    target_range: "Target Range (CIDR):",
    auto_detect: "Detect Subnet",
    concurrency: "Concurrent Goroutines:",
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

document.addEventListener("DOMContentLoaded", () => {
  initWasm();
  setupLanguage();
  setupUIEvents();
  detectLocalSubnet();
});

async function initWasm() {
  if (typeof Go === "undefined") {
    console.warn("Go WASM runtime (wasm_exec.js) not loaded.");
    return;
  }
  try {
    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(fetch("app.wasm"), go.importObject);
    go.run(result.instance);
    console.log("WASM loaded successfully.");
  } catch (err) {
    console.info("WASM binary not found or fallback to JS:", err.message);
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
  document.getElementById("currentLangLabel").textContent = texts.lang_label;

  document.querySelectorAll("[data-i18n]").forEach(el => {
    const key = el.getAttribute("data-i18n");
    if (texts[key]) {
      el.textContent = texts[key];
    }
  });
}

function setupUIEvents() {
  const concurrencyRange = document.getElementById("concurrencyRange");
  const concurrencyVal = document.getElementById("concurrencyVal");
  concurrencyRange.addEventListener("input", (e) => {
    concurrencyVal.textContent = e.target.value;
  });

  document.getElementById("detectSubnetBtn").addEventListener("click", detectLocalSubnet);
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
  try {
    const res = await fetch("/api/local-subnets");
    if (res.ok) {
      const data = await res.json();
      if (data.subnets && data.subnets.length > 0) {
        document.getElementById("targetCidr").value = data.subnets[0];
      }
    }
  } catch (err) {
    console.warn("Could not auto-detect subnet:", err);
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
  timerInterval = setInterval(updateTimer, 100);

  connectEventSource();

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
    }
  } catch (err) {
    alert("サーバー通信エラー: " + err);
    stopScan();
  }
}

function connectEventSource() {
  if (eventSource) {
    eventSource.close();
  }
  eventSource = new EventSource("/api/scan/events");

  eventSource.onmessage = (e) => {
    const update = JSON.parse(e.data);
    handleProgressUpdate(update);
  };

  eventSource.onerror = () => {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  };
}

function handleProgressUpdate(update) {
  if (update.total_targets > 0) {
    const percent = Math.min(100, Math.round((update.scanned_count / update.total_targets) * 100));
    document.getElementById("progressBar").style.width = percent + "%";
    document.getElementById("progressCount").textContent = `${update.scanned_count} / ${update.total_targets}`;
  }

  document.getElementById("foundCount").textContent = update.found_count;

  if (update.latest_host && update.latest_host.is_alive) {
    addHostRow(update.latest_host);
  }

  if (update.is_complete) {
    completeScan();
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
  if (eventSource) {
    eventSource.close();
    eventSource = null;
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
