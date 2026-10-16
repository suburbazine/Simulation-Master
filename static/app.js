// Simulation Master — Client Telemetry & Firmware Studio (app.js)

let inventory = null;
let lastSimResult = null;
let lastMapCompare = null;
let currentGridMode = "delta";
let hoverTimeSec = null;

let quickImportState = { target: "auto", assignSelectId: null };

document.addEventListener("DOMContentLoaded", async () => {
  setupTabs();
  setupControls();
  setupImportModal();
  await loadInventory();
});

function setupTabs() {
  const btns = document.querySelectorAll(".tab-btn");
  btns.forEach((btn) => {
    btn.addEventListener("click", () => {
      btns.forEach((b) => b.classList.remove("active"));
      document.querySelectorAll(".tab-content").forEach((tc) => tc.classList.remove("active"));
      btn.classList.add("active");
      const target = document.getElementById(btn.dataset.tab);
      if (target) target.classList.add("active");
      renderAllCharts();
      if (btn.dataset.tab === "tab-db-maps" && !lastMapCompare) {
        loadDbMapList();
        compareSpecificMap("ecu", "0x80855518");
      }
    });
  });

  document.querySelectorAll(".subtab-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".subtab-btn").forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      currentGridMode = btn.dataset.gridmode;
      renderMapComparisonGrid();
    });
  });
}

function setupControls() {
  document.getElementById("btnRunSim").addEventListener("click", () => runSimulation());
  document.getElementById("btnSwapAB").addEventListener("click", () => {
    swapRunConfigs();
    runSimulation();
  });
  document.getElementById("btnExportCan").addEventListener("click", () => exportCanDump());
  document.getElementById("btnExportCsv").addEventListener("click", () => exportTelemetryCsv());

  document.getElementById("presetSelect").addEventListener("change", (e) => {
    const idx = parseInt(e.target.value, 10);
    if (inventory && inventory.presets && inventory.presets[idx]) {
      applyPreset(inventory.presets[idx]);
      runSimulation();
    }
  });

  document.getElementById("surfaceSelect").addEventListener("change", (e) => {
    const surfId = e.target.value;
    const surf = (inventory?.surfaces || []).find((s) => s.id === surfId);
    if (surf) {
      document.getElementById("rngMuPeak").value = surf.mu_peak;
      document.getElementById("lblMuPeak").textContent = surf.mu_peak.toFixed(2);
      document.getElementById("rngKappaPeak").value = surf.kappa_peak;
      document.getElementById("lblKappaPeak").textContent = Math.round(surf.kappa_peak * 100) + "%";
      document.getElementById("muBadge").innerHTML = `&mu; = ${surf.mu_peak.toFixed(2)}`;
      runSimulation();
    }
  });

  document.getElementById("rngMuPeak").addEventListener("input", (e) => {
    const v = parseFloat(e.target.value);
    document.getElementById("lblMuPeak").textContent = v.toFixed(2);
    document.getElementById("muBadge").innerHTML = `&mu; = ${v.toFixed(2)}`;
  });
  document.getElementById("rngMuPeak").addEventListener("change", () => runSimulation());

  document.getElementById("rngKappaPeak").addEventListener("input", (e) => {
    const v = parseFloat(e.target.value);
    document.getElementById("lblKappaPeak").textContent = Math.round(v * 100) + "%";
  });
  document.getElementById("rngKappaPeak").addEventListener("change", () => runSimulation());

  document.getElementById("chkEnableRunB").addEventListener("change", () => runSimulation());

  // Map browser controls
  document.getElementById("btnSearchMaps").addEventListener("click", () => loadDbMapList());
  document.getElementById("dbModuleSelect").addEventListener("change", () => loadDbMapList());
  document.getElementById("dbSearchInput").addEventListener("keydown", (e) => {
    if (e.key === "Enter") loadDbMapList();
  });

  document.querySelectorAll(".chip-btn").forEach((chip) => {
    chip.addEventListener("click", () => {
      const mod = chip.dataset.mod;
      const addr = chip.dataset.addr;
      document.getElementById("dbModuleSelect").value = mod;
      loadDbMapList();
      compareSpecificMap(mod, addr);
    });
  });

  window.addEventListener("resize", () => renderAllCharts());
}

function renderInventoryUI(inv, preserveSelection) {
  const prevA_ecu = document.getElementById("runA_ecuBin").value;
  const prevB_ecu = document.getElementById("runB_ecuBin").value;
  const prevA_tcu = document.getElementById("runA_tcuBin").value;
  const prevB_tcu = document.getElementById("runB_tcuBin").value;
  const prevA_awd = document.getElementById("runA_awdBin").value;
  const prevB_awd = document.getElementById("runB_awdBin").value;

  // Populate DB status pills (clickable to open Import Modal)
  const dbBar = document.getElementById("dbStatusBar");
  dbBar.innerHTML = (inv.databases || [])
    .map(
      (db) =>
        `<div class="db-pill ${db.available ? "ok" : ""}" style="cursor:pointer" onclick="openImportModal('${db.module}')" title="${escapeHtml(db.path || "Click to import SQLite .db file")}">
          <strong>${db.module.toUpperCase()} DB:</strong> ${
          db.available ? `${db.map_count} maps | ${db.func_count} funcs` : "IMPORT .DB"
        }
        </div>`
    )
    .join("");

  // Populate Surface selector
  const surfSel = document.getElementById("surfaceSelect");
  const prevSurf = surfSel.value;
  surfSel.innerHTML = (inv.surfaces || [])
    .map((s) => `<option value="${s.id}">${s.name} (&mu;=${s.mu_peak.toFixed(2)})</option>`)
    .join("");
  if (preserveSelection && prevSurf) surfSel.value = prevSurf;

  // Populate Binary selectors for Run A and Run B
  populateBinSelect("runA_ecuBin", inv.ecu_bins, preserveSelection ? prevA_ecu : null);
  populateBinSelect("runB_ecuBin", inv.ecu_bins, preserveSelection ? prevB_ecu : null);
  populateBinSelect("runA_tcuBin", inv.tcu_bins, preserveSelection ? prevA_tcu : null);
  populateBinSelect("runB_tcuBin", inv.tcu_bins, preserveSelection ? prevB_tcu : null);
  populateBinSelect("runA_awdBin", inv.awd_bins, preserveSelection ? prevA_awd : null);
  populateBinSelect("runB_awdBin", inv.awd_bins, preserveSelection ? prevB_awd : null);

  // Populate Scenario Presets
  const presetSel = document.getElementById("presetSelect");
  const prevPreset = presetSel.value;
  presetSel.innerHTML = (inv.presets || [])
    .map((p, idx) => `<option value="${idx}">${p.name}</option>`)
    .join("");
  if (preserveSelection && prevPreset) presetSel.value = prevPreset;
}

async function loadInventory() {
  try {
    const res = await fetch("/api/inventory");
    inventory = await res.json();
    renderInventoryUI(inventory, false);

    if (inventory.presets && inventory.presets.length > 0) {
      applyPreset(inventory.presets[0]);
    }
    await runSimulation();
  } catch (err) {
    console.error("Failed to load inventory:", err);
  }
}

function populateBinSelect(selectId, bins, preferredValue) {
  const el = document.getElementById(selectId);
  if (!bins || bins.length === 0) {
    el.innerHTML = `<option value="">[Synthetic Baseline] Built-in Default Profile (Click + Import .BIN)</option>`;
    return;
  }
  el.innerHTML = bins
    .map((b) => `<option value="${escapeHtml(b.path)}" title="${escapeHtml(b.description)}">[${b.tag}] ${escapeHtml(b.name)}</option>`)
    .join("");
  if (preferredValue && bins.some((b) => b.path === preferredValue)) {
    el.value = preferredValue;
  }
}

function applyPreset(preset) {
  document.getElementById("scenarioBannerTitle").textContent = `${preset.name} — [${preset.category}]`;
  document.getElementById("scenarioBannerDesc").textContent = `${preset.description} ${preset.highlight}`;

  const env = preset.environment;
  document.getElementById("surfaceSelect").value = env.surface_id || "dry_warm";
  const surf = (inventory.surfaces || []).find((s) => s.id === env.surface_id) || inventory.surfaces[1];
  if (surf) {
    document.getElementById("rngMuPeak").value = surf.mu_peak;
    document.getElementById("lblMuPeak").textContent = surf.mu_peak.toFixed(2);
    document.getElementById("rngKappaPeak").value = surf.kappa_peak;
    document.getElementById("lblKappaPeak").textContent = Math.round(surf.kappa_peak * 100) + "%";
    document.getElementById("muBadge").innerHTML = `&mu; = ${surf.mu_peak.toFixed(2)}`;
  }
  document.getElementById("inpMassKg").value = env.vehicle_mass_kg || 1915;
  document.getElementById("inpAmbientC").value = env.ambient_temp_c || 20;
  document.getElementById("inpGradePct").value = env.road_grade_pct || 0;
  document.getElementById("inpFrontTireMm").value = Math.round((env.front_tire_radius || 0.334) * 1000);
  document.getElementById("inpRearTireMm").value = Math.round((env.rear_tire_radius || 0.334) * 1000);
  document.getElementById("inpSimDurSec").value = env.sim_duration_sec || 14.5;

  applyRunConfigToForm("runA", preset.run_a);
  if (preset.run_b) {
    document.getElementById("chkEnableRunB").checked = true;
    applyRunConfigToForm("runB", preset.run_b);
  }
}

function applyRunConfigToForm(prefix, cfg) {
  if (!cfg) return;
  document.getElementById(`${prefix}_label`).value = cfg.label || "";
  if (cfg.ecu_bin_path) document.getElementById(`${prefix}_ecuBin`).value = cfg.ecu_bin_path;
  if (cfg.tcu_bin_path) document.getElementById(`${prefix}_tcuBin`).value = cfg.tcu_bin_path;
  if (cfg.awd_bin_path) document.getElementById(`${prefix}_awdBin`).value = cfg.awd_bin_path;
  document.getElementById(`${prefix}_driveMode`).value = cfg.drive_mode || "sport";
  document.getElementById(`${prefix}_escMode`).value = cfg.esc_mode || "sport";
  if (document.getElementById(`${prefix}_fuelType`)) {
    document.getElementById(`${prefix}_fuelType`).value = cfg.fuel_type || "93_aki";
  }
  document.getElementById(`${prefix}_rollout`).value = String(cfg.rollout_inch || 0);
  document.getElementById(`${prefix}_preStage`).value = cfg.pre_stage_time_sec || 1.4;
  document.getElementById(`${prefix}_brakePsi`).value = cfg.brake_hold_psi || 1200;
  document.getElementById(`${prefix}_releaseMs`).value = cfg.brake_release_ms || 140;

  const ov = cfg.overrides || {};
  document.getElementById(`${prefix}_ovTorquePct`).value = ov.torque_scale_pct || 100;
  if (document.getElementById(`${prefix}_ovBoostAdder`)) {
    document.getElementById(`${prefix}_ovBoostAdder`).value = ov.boost_adder_psi ?? "";
  }
  if (document.getElementById(`${prefix}_ovSparkAdder`)) {
    document.getElementById(`${prefix}_ovSparkAdder`).value = ov.spark_advance_adder_deg ?? "";
  }
  if (document.getElementById(`${prefix}_ovCamOverlap`)) {
    document.getElementById(`${prefix}_ovCamOverlap`).value = ov.cam_overlap_adder_deg ?? "";
  }
  document.getElementById(`${prefix}_ovSparkFloor`).value = ov.shift_spark_floor_deg ?? "";
  document.getElementById(`${prefix}_ovStallRPM`).value = ov.launch_stall_rpm ?? "";
  document.getElementById(`${prefix}_ovTcuCap`).value = ov.torque_cap_nm ?? "";
  document.getElementById(`${prefix}_ovAwdSlew`).value = ov.awd_slew_rate_nm_s ?? "";
  document.getElementById(`${prefix}_ovBrakeGate`).value =
    ov.bypass_brake_gate === undefined || ov.bypass_brake_gate === null ? "" : String(ov.bypass_brake_gate);
  document.getElementById(`${prefix}_ovTcsInhibit`).value =
    ov.tcs_inhibit === undefined || ov.tcs_inhibit === null ? "" : String(ov.tcs_inhibit);
  document.getElementById(`${prefix}_ovTchaFix`).value =
    ov.tcha_surge_fix === undefined || ov.tcha_surge_fix === null ? "" : String(ov.tcha_surge_fix);
}

function readRunConfigFromForm(prefix) {
  const numOrNull = (id) => {
    const el = document.getElementById(id);
    if (!el) return null;
    const raw = el.value.trim();
    if (raw === "") return null;
    const v = parseFloat(raw);
    return isNaN(v) ? null : v;
  };
  const intOrNull = (id) => {
    const el = document.getElementById(id);
    if (!el) return null;
    const raw = el.value.trim();
    if (raw === "") return null;
    const v = parseInt(raw, 10);
    return isNaN(v) ? null : v;
  };
  const boolOrNull = (id) => {
    const el = document.getElementById(id);
    if (!el) return null;
    const raw = el.value;
    if (raw === "true") return true;
    if (raw === "false") return false;
    return null;
  };

  return {
    label: document.getElementById(`${prefix}_label`).value || prefix.toUpperCase(),
    ecu_bin_path: document.getElementById(`${prefix}_ecuBin`).value,
    tcu_bin_path: document.getElementById(`${prefix}_tcuBin`).value,
    awd_bin_path: document.getElementById(`${prefix}_awdBin`).value,
    drive_mode: document.getElementById(`${prefix}_driveMode`).value,
    esc_mode: document.getElementById(`${prefix}_escMode`).value,
    fuel_type: document.getElementById(`${prefix}_fuelType`)
      ? document.getElementById(`${prefix}_fuelType`).value
      : "93_aki",
    rollout_inch: parseFloat(document.getElementById(`${prefix}_rollout`).value) || 0,
    pre_stage_time_sec: parseFloat(document.getElementById(`${prefix}_preStage`).value) || 1.4,
    brake_hold_psi: parseFloat(document.getElementById(`${prefix}_brakePsi`).value) || 1200,
    brake_release_ms: parseFloat(document.getElementById(`${prefix}_releaseMs`).value) || 140,
    pedal_pct: 100.0,
    overrides: {
      torque_scale_pct: parseFloat(document.getElementById(`${prefix}_ovTorquePct`).value) || 100,
      boost_adder_psi: numOrNull(`${prefix}_ovBoostAdder`),
      spark_advance_adder_deg: numOrNull(`${prefix}_ovSparkAdder`),
      cam_overlap_adder_deg: numOrNull(`${prefix}_ovCamOverlap`),
      shift_spark_floor_deg: numOrNull(`${prefix}_ovSparkFloor`),
      launch_stall_rpm: numOrNull(`${prefix}_ovStallRPM`),
      torque_cap_nm: numOrNull(`${prefix}_ovTcuCap`),
      awd_slew_rate_nm_s: numOrNull(`${prefix}_ovAwdSlew`),
      bypass_brake_gate: boolOrNull(`${prefix}_ovBrakeGate`),
      tcs_inhibit: intOrNull(`${prefix}_ovTcsInhibit`),
      tcha_surge_fix: boolOrNull(`${prefix}_ovTchaFix`),
      shift_cut_scale_pct: 100,
      front_split_scale_pct: 100,
    },
  };
}

function buildSimRequestPayload() {
  const env = {
    surface_id: document.getElementById("surfaceSelect").value,
    custom_mu_peak: parseFloat(document.getElementById("rngMuPeak").value) || 1.08,
    custom_kappa_peak: parseFloat(document.getElementById("rngKappaPeak").value) || 0.11,
    vehicle_mass_kg: parseFloat(document.getElementById("inpMassKg").value) || 1915,
    ambient_temp_c: parseFloat(document.getElementById("inpAmbientC").value) || 20,
    coolant_temp_c: 85.0,
    road_grade_pct: parseFloat(document.getElementById("inpGradePct").value) || 0,
    front_tire_radius: (parseFloat(document.getElementById("inpFrontTireMm").value) || 334) / 1000.0,
    rear_tire_radius: (parseFloat(document.getElementById("inpRearTireMm").value) || 334) / 1000.0,
    sim_duration_sec: parseFloat(document.getElementById("inpSimDurSec").value) || 14.5,
  };

  const req = {
    environment: env,
    run_a: readRunConfigFromForm("runA"),
  };
  if (document.getElementById("chkEnableRunB").checked) {
    req.run_b = readRunConfigFromForm("runB");
  }
  return req;
}

function swapRunConfigs() {
  const a = readRunConfigFromForm("runA");
  const b = readRunConfigFromForm("runB");
  applyRunConfigToForm("runA", b);
  applyRunConfigToForm("runB", a);
}

async function runSimulation() {
  const btn = document.getElementById("btnRunSim");
  btn.textContent = "SOLVING 1MS...";
  try {
    const payload = buildSimRequestPayload();
    const res = await fetch("/api/simulate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const data = await res.json();
    if (!res.ok) {
      alert("Simulation Error: " + (data.error || res.statusText));
      return;
    }
    lastSimResult = data;
    renderMuSlipCurve(data.mu_slip_curve, data.surface);
    renderKpiStrip(data);
    renderCalDiffBanner(data);
    renderShiftAndCalTables(data);
    renderAllCharts();
  } catch (err) {
    console.error("Simulation failed:", err);
  } finally {
    btn.innerHTML = "&#9654; RUN A/B EMULATION";
  }
}

// RENDER MU-SLIP TIRE FRICTION CURVE
function renderMuSlipCurve(curve, surf) {
  const canvas = document.getElementById("muCurveCanvas");
  if (!canvas || !curve) return;
  const ctx = canvas.getContext("2d");
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  const padL = 32, padR = 10, padT = 12, padB = 18;
  const pw = w - padL - padR;
  const ph = h - padT - padB;
  const maxMu = 1.55;

  // Grid lines
  ctx.strokeStyle = "#1e293b";
  ctx.lineWidth = 1;
  [0.5, 1.0, 1.5].forEach((m) => {
    const y = padT + ph * (1 - m / maxMu);
    ctx.beginPath();
    ctx.moveTo(padL, y);
    ctx.lineTo(w - padR, y);
    ctx.stroke();
    ctx.fillStyle = "#64748b";
    ctx.font = "9px Consolas, monospace";
    ctx.fillText(m.toFixed(1), 4, y + 3);
  });

  // Draw curve
  ctx.strokeStyle = "#00e5ff";
  ctx.lineWidth = 2;
  ctx.beginPath();
  curve.forEach((pt, idx) => {
    const x = padL + (pt.slip_pct / 60.0) * pw;
    const y = padT + ph * (1 - pt.mu / maxMu);
    if (idx === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  });
  ctx.stroke();

  // Peak marker
  if (surf) {
    const px = padL + ((surf.kappa_peak * 100) / 60.0) * pw;
    const py = padT + ph * (1 - surf.mu_peak / maxMu);
    ctx.fillStyle = "#ff9100";
    ctx.beginPath();
    ctx.arc(px, py, 3.5, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = "#e2e8f0";
    ctx.font = "9.5px Consolas, monospace";
    ctx.fillText(`Peak ${surf.mu_peak.toFixed(2)} @ ${Math.round(surf.kappa_peak * 100)}% slip`, px + 6, Math.max(11, py - 2));
  }

  ctx.fillStyle = "#64748b";
  ctx.font = "9px Consolas, monospace";
  ctx.fillText("0%", padL, h - 4);
  ctx.fillText("20%", padL + pw * 0.333, h - 4);
  ctx.fillText("40%", padL + pw * 0.666, h - 4);
  ctx.fillText("60% Slip", w - 52, h - 4);
}

// RENDER DRAG STRIP & PERFORMANCE KPI CARDS
function renderKpiStrip(data) {
  const strip = document.getElementById("kpiStrip");
  const a = data.run_a.summary;
  const b = data.run_b ? data.run_b.summary : null;

  document.getElementById("legRunALabel").textContent = data.run_a.label;
  document.getElementById("legRunBLabel").textContent = data.run_b ? data.run_b.label : "Run B (Disabled)";

  const fmtSec = (v) => (v > 0 ? v.toFixed(2) + "s" : "—");
  const fmtDeltaTime = (va, vb) => {
    if (!b || va <= 0 || vb <= 0) return `<span class="kpi-delta delta-neutral">Single Run</span>`;
    const d = vb - va;
    const cls = d < -0.005 ? "delta-good" : d > 0.005 ? "delta-bad" : "delta-neutral";
    const sign = d > 0 ? "+" : "";
    return `<span class="kpi-delta ${cls}">&Delta; ${sign}${d.toFixed(2)}s ${d < 0 ? "(FASTER)" : ""}</span>`;
  };
  const fmtDeltaHigherBetter = (va, vb, unit) => {
    if (!b) return `<span class="kpi-delta delta-neutral">Single Run</span>`;
    const d = vb - va;
    const cls = d > 0.5 ? "delta-good" : d < -0.5 ? "delta-bad" : "delta-neutral";
    const sign = d > 0 ? "+" : "";
    return `<span class="kpi-delta ${cls}">&Delta; ${sign}${d.toFixed(1)} ${unit}</span>`;
  };

  const cards = [
    {
      title: "60-Foot Launch",
      valA: fmtSec(a.sixty_foot_sec),
      valB: b ? fmtSec(b.sixty_foot_sec) : "—",
      delta: fmtDeltaTime(a.sixty_foot_sec, b?.sixty_foot_sec),
    },
    {
      title: "0 – 60 MPH",
      valA: fmtSec(a.zero_to_60_mph_sec),
      valB: b ? fmtSec(b.zero_to_60_mph_sec) : "—",
      delta: fmtDeltaTime(a.zero_to_60_mph_sec, b?.zero_to_60_mph_sec),
    },
    {
      title: "0 – 100 KM/H",
      valA: fmtSec(a.zero_to_100_kmh_sec),
      valB: b ? fmtSec(b.zero_to_100_kmh_sec) : "—",
      delta: fmtDeltaTime(a.zero_to_100_kmh_sec, b?.zero_to_100_kmh_sec),
    },
    {
      title: "1/8 Mile ET @ MPH",
      valA: a.eighth_mile_sec > 0 ? `${a.eighth_mile_sec.toFixed(2)}s @ ${a.eighth_mile_mph.toFixed(1)}` : "—",
      valB: b && b.eighth_mile_sec > 0 ? `${b.eighth_mile_sec.toFixed(2)}s @ ${b.eighth_mile_mph.toFixed(1)}` : "—",
      delta: fmtDeltaTime(a.eighth_mile_sec, b?.eighth_mile_sec),
    },
    {
      title: "1/4 Mile ET @ MPH",
      valA: a.quarter_mile_sec > 0 ? `${a.quarter_mile_sec.toFixed(2)}s @ ${a.quarter_mile_mph.toFixed(1)}` : "—",
      valB: b && b.quarter_mile_sec > 0 ? `${b.quarter_mile_sec.toFixed(2)}s @ ${b.quarter_mile_mph.toFixed(1)}` : "—",
      delta: fmtDeltaTime(a.quarter_mile_sec, b?.quarter_mile_sec),
    },
    {
      title: "100 – 200 KM/H",
      valA: fmtSec(a.hundred_to_200_kmh_sec),
      valB: b ? fmtSec(b.hundred_to_200_kmh_sec) : "—",
      delta: fmtDeltaTime(a.hundred_to_200_kmh_sec, b?.hundred_to_200_kmh_sec),
    },
    {
      title: "AWD @ +50ms Launch",
      valA: `${a.launch_awd_torque_nm.toFixed(0)} Nm`,
      valB: b ? `${b.launch_awd_torque_nm.toFixed(0)} Nm` : "—",
      delta: fmtDeltaHigherBetter(a.launch_awd_torque_nm, b?.launch_awd_torque_nm, "Nm"),
    },
    {
      title: "Peak Torque / WHP",
      valA: `${a.peak_engine_torque_nm.toFixed(0)}Nm / ${a.peak_wheel_power_hp.toFixed(0)}hp`,
      valB: b ? `${b.peak_engine_torque_nm.toFixed(0)}Nm / ${b.peak_wheel_power_hp.toFixed(0)}hp` : "—",
      delta: fmtDeltaHigherBetter(a.peak_wheel_power_hp, b?.peak_wheel_power_hp, "whp"),
    },
  ];

  strip.innerHTML = cards
    .map(
      (c) => `
      <div class="kpi-card">
        <div class="kpi-title">${c.title}</div>
        <div class="kpi-row-ab">
          <span class="kpi-val-a" title="Run A">${c.valA}</span>
          <span class="kpi-val-b" title="Run B">${c.valB}</span>
        </div>
        ${c.delta}
      </div>`
    )
    .join("");
}

function renderCalDiffBanner(data) {
  const container = document.getElementById("calDiffChips");
  const diffs = data.comparison?.key_calibration_diffs || [];
  if (diffs.length === 0) {
    container.innerHTML = `<span class="diff-chip">Identical binary offsets or single-run mode</span>`;
    return;
  }
  container.innerHTML = diffs.map((d) => `<span class="diff-chip">${escapeHtml(d)}</span>`).join("");
}

// MULTI-CHANNEL SYNCHRONIZED OSCILLOSCOPE ENGINE
function renderAllCharts() {
  if (!lastSimResult) return;
  const telA = lastSimResult.run_a.telemetry || [];
  const telB = lastSimResult.run_b ? lastSimResult.run_b.telemetry || [] : [];

  // Tab 1: Overview 3-pane
  drawScopeChart("canvasOverview1", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "MPH",
    leftMax: 160,
    rightUnit: "Accel G",
    rightMin: -0.2,
    rightMax: 1.4,
    seriesLeft: { key: "v_mph", label: "Speed (mph)", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "accel_g", label: "Longitudinal G", colorA: "#00e676", colorB: "#ff5252" },
  });

  drawScopeChart("canvasOverview2", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "RPM",
    leftMax: 7000,
    rightUnit: "Torque Nm",
    rightMin: 0,
    rightMax: 850,
    seriesLeft: { key: "rpm", label: "Engine RPM", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "eng_nm", label: "Actual Crank Nm", colorA: "#b388ff", colorB: "#ffd740" },
  });

  drawScopeChart("canvasOverview3", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "AWD Nm",
    leftMax: 1100,
    rightUnit: "Slip %",
    rightMin: 0,
    rightMax: 45,
    seriesLeft: { key: "awd_actual_nm", label: "AWD Coupling Nm", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "slip_r_pct", label: "Rear Wheel Slip %", colorA: "#ff5252", colorB: "#00e676" },
  });

  // Tab 2: Launch & Brake-Gate Microscope (t = -0.5s to +2.0s)
  drawScopeChart("canvasLaunch1", telA, telB, {
    tMin: -0.5,
    tMax: 2.0,
    leftUnit: "AWD Nm",
    leftMax: 1100,
    rightUnit: "Brake PSI",
    rightMin: 0,
    rightMax: 1400,
    refLineRight: { val: 64.5, label: "0xC07498 AWD Brake Gate (64.5 psi / 4.45 bar)", color: "#ff5252" },
    seriesLeft: { key: "awd_actual_nm", label: "AWD Coupling Nm", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "brake_psi", label: "Brake Pressure (psi)", colorA: "#94a3b8", colorB: "#cbd5e1" },
  });

  drawScopeChart("canvasLaunch2", telA, telB, {
    tMin: -0.5,
    tMax: 2.0,
    leftUnit: "Slip %",
    leftMax: 45,
    rightUnit: "Accel G",
    rightMin: 0,
    rightMax: 1.4,
    seriesLeft: { key: "slip_r_pct", label: "Rear Slip %", colorA: "#ff5252", colorB: "#ffd740" },
    seriesRight: { key: "accel_g", label: "Launch Accel G", colorA: "#00e5ff", colorB: "#ff9100" },
  });

  // Tab 3: ECU Torque, Boost & Shift Spark Cut
  drawScopeChart("canvasEcu1", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "Torque Nm",
    leftMax: 850,
    rightUnit: "Boost PSI",
    rightMin: 0,
    rightMax: 28,
    seriesLeft: { key: "eng_nm", label: "Actual Engine Torque (Nm)", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "boost_psi", label: "Boost Pressure (psi)", colorA: "#00e676", colorB: "#b388ff" },
  });

  drawScopeChart("canvasEcu2", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "Spark Deg",
    leftMin: -20,
    leftMax: 30,
    rightUnit: "Eta IGA",
    rightMin: 0,
    rightMax: 1.1,
    seriesLeft: { key: "iga_deg", label: "Spark Advance (deg BTDC)", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "eta_iga", label: "Spark Efficiency (eta_iga)", colorA: "#00e676", colorB: "#ff5252" },
  });

  drawScopeChart("canvasEcu3", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "Overlap °CA",
    leftMin: -5,
    leftMax: 60,
    rightUnit: "Knock °KR",
    rightMin: 0,
    rightMax: 10,
    seriesLeft: { key: "cam_overlap_deg", label: "Dual-CVVT Valve Overlap (°CA)", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "kr_deg", label: "Octane Knock Retard (°KR)", colorA: "#ff5252", colorB: "#ffd740" },
  });

  // Tab 4: AWD Split & Weight Transfer
  drawScopeChart("canvasAwd1", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "Rear Fz (N)",
    leftMin: 6000,
    leftMax: 14000,
    rightUnit: "Front Fz (N)",
    rightMin: 6000,
    rightMax: 14000,
    seriesLeft: { key: "fz_rear_n", label: "Rear Normal Load Fz,r (N)", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "fz_front_n", label: "Front Normal Load Fz,f (N)", colorA: "#64748b", colorB: "#cbd5e1" },
  });

  drawScopeChart("canvasAwd2", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "Split %",
    leftMin: 0,
    leftMax: 55,
    rightUnit: "Rear Fx (N)",
    rightMin: 0,
    rightMax: 14000,
    seriesLeft: { key: "awd_split_pct", label: "AWD Front Split %", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "fx_rear_n", label: "Rear Tractive Force Fx,r (N)", colorA: "#00e676", colorB: "#b388ff" },
  });
}

function findSampleAtTime(arr, t) {
  if (!arr || arr.length === 0 || t === null || t === undefined) return null;
  let best = arr[0];
  let bestD = Math.abs(arr[0].t - t);
  for (let i = 1; i < arr.length; i++) {
    const d = Math.abs(arr[i].t - t);
    if (d < bestD) {
      bestD = d;
      best = arr[i];
    }
  }
  return best;
}

function formatLegendVal(val, maxRange) {
  if (val === undefined || val === null || isNaN(val)) return "";
  if (maxRange <= 2.5) return val.toFixed(2);
  if (maxRange <= 60) return val.toFixed(1);
  return val.toFixed(0);
}

function renderPerChartLegend(canvasId, telA, telB, cfg, tMin, tMax) {
  const legEl = document.getElementById(`legend_${canvasId}`);
  if (!legEl) return;

  const hasHover = hoverTimeSec !== null && hoverTimeSec >= tMin && hoverTimeSec <= tMax;
  const sa = hasHover ? findSampleAtTime(telA, hoverTimeSec) : null;
  const sb = hasHover && telB.length > 0 ? findSampleAtTime(telB, hoverTimeSec) : null;

  const lMax = cfg.leftMax || 100;
  const rMax = cfg.rightMax || 100;

  const items = [];

  // Left Axis - Run A (Solid)
  const valLA = sa ? `: <strong>${formatLegendVal(sa[cfg.seriesLeft.key], lMax)}</strong>` : "";
  items.push(
    `<span class="chart-legend-item" title="Left Y-Axis (${cfg.leftUnit}) — Run A (Solid)">
      <i class="chart-legend-line" style="border-top: 2.5px solid ${cfg.seriesLeft.colorA};"></i>
      <span>[A] ${escapeHtml(cfg.seriesLeft.label)}${valLA}</span>
    </span>`
  );

  // Left Axis - Run B (Dashed)
  if (telB.length > 0) {
    const valLB = sb ? `: <strong>${formatLegendVal(sb[cfg.seriesLeft.key], lMax)}</strong>` : "";
    items.push(
      `<span class="chart-legend-item" title="Left Y-Axis (${cfg.leftUnit}) — Run B (Dashed)">
        <i class="chart-legend-line" style="border-top: 2.5px dashed ${cfg.seriesLeft.colorB};"></i>
        <span>[B] ${escapeHtml(cfg.seriesLeft.label)}${valLB}</span>
      </span>`
    );
  }

  // Right Axis - Run A (Solid)
  const valRA = sa ? `: <strong>${formatLegendVal(sa[cfg.seriesRight.key], rMax)}</strong>` : "";
  items.push(
    `<span class="chart-legend-item" title="Right Y-Axis (${cfg.rightUnit}) — Run A (Solid)">
      <i class="chart-legend-line" style="border-top: 2.5px solid ${cfg.seriesRight.colorA};"></i>
      <span>[A] ${escapeHtml(cfg.seriesRight.label)}${valRA}</span>
    </span>`
  );

  // Right Axis - Run B (Dashed)
  if (telB.length > 0) {
    const valRB = sb ? `: <strong>${formatLegendVal(sb[cfg.seriesRight.key], rMax)}</strong>` : "";
    items.push(
      `<span class="chart-legend-item" title="Right Y-Axis (${cfg.rightUnit}) — Run B (Dashed)">
        <i class="chart-legend-line" style="border-top: 2.5px dashed ${cfg.seriesRight.colorB};"></i>
        <span>[B] ${escapeHtml(cfg.seriesRight.label)}${valRB}</span>
      </span>`
    );
  }

  // Optional Reference Threshold Line
  if (cfg.refLineRight) {
    items.push(
      `<span class="chart-legend-item" title="Reference Gate">
        <i class="chart-legend-line" style="border-top: 2px dotted ${cfg.refLineRight.color};"></i>
        <span>${escapeHtml(cfg.refLineRight.label)}</span>
      </span>`
    );
  }

  legEl.innerHTML = items.join("");
}

function drawScopeChart(canvasId, telA, telB, cfg) {
  const canvas = document.getElementById(canvasId);
  if (!canvas || telA.length === 0) return;

  // Bind hover listener once
  if (!canvas.dataset.boundHover) {
    canvas.dataset.boundHover = "1";
    canvas.addEventListener("mousemove", (e) => {
      const rect = canvas.getBoundingClientRect();
      const relX = (e.clientX - rect.left) * (canvas.width / rect.width);
      const padL = 54, padR = 56;
      const pw = canvas.width - padL - padR;
      const tMin = parseFloat(canvas.dataset.tmin || "-0.5");
      const tMax = parseFloat(canvas.dataset.tmax || "14.5");
      if (relX >= padL && relX <= canvas.width - padR) {
        hoverTimeSec = tMin + ((relX - padL) / pw) * (tMax - tMin);
        updateCursorReadout(hoverTimeSec);
        renderAllCharts();
      }
    });
    canvas.addEventListener("mouseleave", () => {
      hoverTimeSec = null;
      renderAllCharts();
    });
  }

  const ctx = canvas.getContext("2d");
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  const padL = 54, padR = 56, padT = 24, padB = 22;
  const pw = w - padL - padR;
  const ph = h - padT - padB;

  const tMin = cfg.tMin !== undefined && cfg.tMin !== null ? cfg.tMin : telA[0].t;
  const tMax = cfg.tMax !== undefined && cfg.tMax !== null ? cfg.tMax : telA[telA.length - 1].t;
  canvas.dataset.tmin = String(tMin);
  canvas.dataset.tmax = String(tMax);

  // Update the DOM header legend for this chart
  renderPerChartLegend(canvasId, telA, telB, cfg, tMin, tMax);

  const lMin = cfg.leftMin || 0;
  const lMax = cfg.leftMax || 100;
  const rMin = cfg.rightMin || 0;
  const rMax = cfg.rightMax || 100;

  const xOfT = (t) => padL + ((t - tMin) / (tMax - tMin)) * pw;
  const yOfL = (v) => padT + ph * (1 - (v - lMin) / (lMax - lMin));
  const yOfR = (v) => padT + ph * (1 - (v - rMin) / (rMax - rMin));

  // Top-left & Top-right Y-Axis unit titles + embedded canvas legend strip
  ctx.font = "bold 9.5px Consolas, monospace";
  ctx.fillStyle = cfg.seriesLeft.colorA;
  ctx.textAlign = "left";
  ctx.fillText(`◀ ${cfg.leftUnit}`, 4, 13);

  ctx.fillStyle = cfg.seriesRight.colorA;
  ctx.textAlign = "right";
  ctx.fillText(`${cfg.rightUnit} ▶`, w - 4, 13);

  // Draw embedded in-canvas legend items along top margin (y = 10..13)
  const canvasLegEntries = [
    { label: `[A] ${cfg.seriesLeft.label}`, color: cfg.seriesLeft.colorA, dashed: false },
  ];
  if (telB.length > 0) {
    canvasLegEntries.push({ label: `[B] ${cfg.seriesLeft.label}`, color: cfg.seriesLeft.colorB, dashed: true });
  }
  canvasLegEntries.push({ label: `[A] ${cfg.seriesRight.label}`, color: cfg.seriesRight.colorA, dashed: false });
  if (telB.length > 0) {
    canvasLegEntries.push({ label: `[B] ${cfg.seriesRight.label}`, color: cfg.seriesRight.colorB, dashed: true });
  }

  let legX = padL + 8;
  ctx.font = "10px Consolas, monospace";
  ctx.textAlign = "left";
  canvasLegEntries.forEach((entry) => {
    ctx.save();
    ctx.strokeStyle = entry.color;
    ctx.lineWidth = 2.2;
    if (entry.dashed) ctx.setLineDash([4, 3]);
    ctx.beginPath();
    ctx.moveTo(legX, 10);
    ctx.lineTo(legX + 16, 10);
    ctx.stroke();
    ctx.restore();

    ctx.fillStyle = "#cbd5e1";
    ctx.fillText(entry.label, legX + 20, 13);
    legX += 20 + ctx.measureText(entry.label).width + 18;
  });

  // Horizontal grid lines & Y axis labels
  ctx.strokeStyle = "#162032";
  ctx.lineWidth = 1;
  for (let i = 0; i <= 4; i++) {
    const frac = i / 4;
    const y = padT + ph * frac;
    ctx.beginPath();
    ctx.moveTo(padL, y);
    ctx.lineTo(w - padR, y);
    ctx.stroke();

    const lVal = lMax - frac * (lMax - lMin);
    const rVal = rMax - frac * (rMax - rMin);
    ctx.fillStyle = cfg.seriesLeft.colorA;
    ctx.font = "10px Consolas, monospace";
    ctx.textAlign = "right";
    ctx.fillText(lVal.toFixed(lMax <= 5 ? 1 : 0), padL - 5, y + 3);

    ctx.fillStyle = cfg.seriesRight.colorA;
    ctx.textAlign = "left";
    ctx.fillText(rVal.toFixed(rMax <= 5 ? 2 : 0), w - padR + 5, y + 3);
  }

  // Vertical time grid lines & t=0 launch line
  const stepT = tMax - tMin <= 3.5 ? 0.25 : 2.0;
  for (let t = Math.ceil(tMin / stepT) * stepT; t <= tMax; t += stepT) {
    const x = xOfT(t);
    ctx.strokeStyle = Math.abs(t) < 0.001 ? "#38bdf8" : "#162032";
    ctx.lineWidth = Math.abs(t) < 0.001 ? 1.5 : 1;
    ctx.beginPath();
    ctx.moveTo(x, padT);
    ctx.lineTo(x, h - padB);
    ctx.stroke();

    ctx.fillStyle = Math.abs(t) < 0.001 ? "#38bdf8" : "#64748b";
    ctx.textAlign = "center";
    ctx.font = "10px Consolas, monospace";
    ctx.fillText((t >= 0 ? "+" : "") + t.toFixed(tMax - tMin <= 3.5 ? 2 : 1) + "s", x, h - 6);
  }

  // Optional reference line (e.g. 64.5 psi brake gate)
  if (cfg.refLineRight) {
    const ry = yOfR(cfg.refLineRight.val);
    ctx.save();
    ctx.strokeStyle = cfg.refLineRight.color;
    ctx.setLineDash([4, 4]);
    ctx.lineWidth = 1.2;
    ctx.beginPath();
    ctx.moveTo(padL, ry);
    ctx.lineTo(w - padR, ry);
    ctx.stroke();
    ctx.fillStyle = cfg.refLineRight.color;
    ctx.textAlign = "left";
    ctx.font = "10px Consolas, monospace";
    ctx.fillText(cfg.refLineRight.label, padL + 8, ry - 4);
    ctx.restore();
  }

  // Helper to plot a trace
  const plotTrace = (samples, key, yFn, color, dashed) => {
    if (!samples || samples.length === 0) return;
    ctx.save();
    ctx.strokeStyle = color;
    ctx.lineWidth = dashed ? 1.8 : 2.0;
    if (dashed) ctx.setLineDash([5, 3]);
    ctx.beginPath();
    let started = false;
    for (let i = 0; i < samples.length; i++) {
      const s = samples[i];
      if (s.t < tMin || s.t > tMax) continue;
      const x = xOfT(s.t);
      const y = Math.max(padT, Math.min(h - padB, yFn(s[key])));
      if (!started) {
        ctx.moveTo(x, y);
        started = true;
      } else {
        ctx.lineTo(x, y);
      }
    }
    ctx.stroke();
    ctx.restore();
  };

  // Plot Left Series (Run A solid, Run B dashed)
  plotTrace(telA, cfg.seriesLeft.key, yOfL, cfg.seriesLeft.colorA, false);
  if (telB.length > 0) {
    plotTrace(telB, cfg.seriesLeft.key, yOfL, cfg.seriesLeft.colorB, true);
  }

  // Plot Right Series (Run A solid, Run B dashed)
  plotTrace(telA, cfg.seriesRight.key, yOfR, cfg.seriesRight.colorA, false);
  if (telB.length > 0) {
    plotTrace(telB, cfg.seriesRight.key, yOfR, cfg.seriesRight.colorB, true);
  }

  // Draw hover crosshair if active
  if (hoverTimeSec !== null && hoverTimeSec >= tMin && hoverTimeSec <= tMax) {
    const cx = xOfT(hoverTimeSec);
    ctx.save();
    ctx.strokeStyle = "rgba(255, 255, 255, 0.55)";
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(cx, padT);
    ctx.lineTo(cx, h - padB);
    ctx.stroke();
    ctx.restore();
  }
}

function updateCursorReadout(t) {
  if (!lastSimResult) return;
  const findNearest = (arr) => {
    if (!arr || arr.length === 0) return null;
    let best = arr[0];
    let bestD = Math.abs(arr[0].t - t);
    for (let i = 1; i < arr.length; i++) {
      const d = Math.abs(arr[i].t - t);
      if (d < bestD) {
        bestD = d;
        best = arr[i];
      }
    }
    return best;
  };

  const sa = findNearest(lastSimResult.run_a.telemetry);
  const sb = lastSimResult.run_b ? findNearest(lastSimResult.run_b.telemetry) : null;
  if (!sa) return;

  const el = document.getElementById("cursorReadout");
  let txt = `t=${sa.t >= 0 ? "+" : ""}${sa.t.toFixed(2)}s | [A] ${sa.v_mph.toFixed(1)} mph (G${sa.gear}, ${sa.rpm.toFixed(0)} RPM, ${sa.eng_nm.toFixed(0)} Nm, ${sa.boost_psi.toFixed(1)} psi, Spark ${sa.iga_deg.toFixed(1)}° / KR -${(sa.kr_deg || 0).toFixed(1)}°, CamOvl ${(sa.cam_overlap_deg || 0).toFixed(0)}°CA, AWD ${sa.awd_actual_nm.toFixed(0)} Nm)`;
  if (sb) {
    txt += `  vs  [B] ${sb.v_mph.toFixed(1)} mph (G${sb.gear}, ${sb.rpm.toFixed(0)} RPM, ${sb.eng_nm.toFixed(0)} Nm, ${sb.boost_psi.toFixed(1)} psi, Spark ${sb.iga_deg.toFixed(1)}° / KR -${(sb.kr_deg || 0).toFixed(1)}°, CamOvl ${(sb.cam_overlap_deg || 0).toFixed(0)}°CA, AWD ${sb.awd_actual_nm.toFixed(0)} Nm)`;
  }
  el.textContent = txt;
}

// RENDER SHIFT TABLES & DECODED CALIBRATION SUMMARY TABLE
function renderShiftAndCalTables(data) {
  const fillShiftTable = (tblId, shifts) => {
    const tbody = document.querySelector(`#${tblId} tbody`);
    if (!shifts || shifts.length === 0) {
      tbody.innerHTML = `<tr><td colspan="9">No upshifts occurred during simulation window</td></tr>`;
      return;
    }
    tbody.innerHTML = shifts
      .map(
        (s) => `
      <tr>
        <td><strong>${s.from_gear} &rarr; ${s.to_gear}</strong></td>
        <td>+${s.start_time_sec.toFixed(2)}s</td>
        <td>${s.trigger_speed_kmh.toFixed(0)} km/h (${s.trigger_speed_mph.toFixed(0)} mph)</td>
        <td>${s.entry_rpm.toFixed(0)} &rarr; ${s.drop_to_rpm.toFixed(0)}</td>
        <td>${s.tcu_torque_req_nm.toFixed(0)} Nm</td>
        <td>${s.min_spark_deg.toFixed(1)}&deg;</td>
        <td>${
          s.spark_floor_clamped
            ? `<span class="clamped-pill">CLAMPED BY 0x207AD9</span>`
            : `<span class="clean-pill">Unclamped Cut</span>`
        }</td>
        <td>${s.actual_min_torque_nm.toFixed(0)} Nm</td>
        <td>${s.duration_ms.toFixed(0)} ms</td>
      </tr>`
      )
      .join("");
  };

  fillShiftTable("tblShiftsA", data.run_a.shifts);
  fillShiftTable("tblShiftsB", data.run_b ? data.run_b.shifts : []);

  const ca = data.run_a.calibration;
  const cb = data.run_b ? data.run_b.calibration : null;
  const rows = [
    ["ECU", "Driver Demand Sport Peak", "0x80814E80 (+0x214E80)", `${ca.ecu_driver_demand_peak_nm} Nm`, cb ? `${cb.ecu_driver_demand_peak_nm} Nm` : "—", "Driver pedal-to-torque request ceiling"],
    ["ECU", "Forward Torque Model Peak (TQI_BAS)", "0x80855518 (+0x255518)", `${ca.ecu_tqi_bas_peak_nm} Nm`, cb ? `${cb.ecu_tqi_bas_peak_nm} Nm` : "—", "Airload-to-indicated torque conversion (inflated in Loud -> trips TCHA)"],
    ["ECU", "Cam-State Torque Ceiling (TQI_MAX)", "0x80853D14 (+0x253D14)", `${ca.ecu_tqi_max_peak_nm} Nm`, cb ? `${cb.ecu_tqi_max_peak_nm} Nm` : "—", "Maximum indicated torque envelope across 8 VVT states"],
    ["ECU", "Max Boost Target (PUT_SP)", "0x8081EC00 (+0x21EC00)", `${ca.ecu_boost_target_hpa} hPa`, cb ? `${cb.ecu_boost_target_hpa} hPa` : "—", "Manifold absolute pressure target (~24.7 psi gauge @ sea level)"],
    ["ECU", "Shift Spark Floor Scalar", "0x80807AD9 (+0x207AD9)", `${ca.ecu_shift_spark_floor_deg}° BTDC`, cb ? `${cb.ecu_shift_spark_floor_deg}° BTDC` : "—", "Clamps minimum spark advance during TCU shift torque cuts"],
    ["ECU", "Warm WOT Minimum Spark (IGA_MIN)", "0x80839AF4 (+0x239AF4)", `${ca.ecu_iga_min_warm_wot_deg}° BTDC`, cb ? `${cb.ecu_iga_min_warm_wot_deg}° BTDC` : "—", "Combustion stability spark floor at 81°C coolant"],
    ["ECU", "TCS Torque Intervention Inhibit", "0x80800B4B (+0x200B4B)", `${ca.ecu_tcs_inhibit} (${ca.ecu_tcs_inhibit ? "Inhibited" : "Active"})`, cb ? `${cb.ecu_tcs_inhibit} (${cb.ecu_tcs_inhibit ? "Inhibited" : "Active"})` : "—", "When 1 (v103), bypasses ESC CAN 0x153 wheelspin torque cut"],
    ["ECU", "TCHA Turbo Overspeed Torque Cut", "0x80803EC6 (+0x203EC6)", `${ca.ecu_tcha_limit_active ? `ACTIVE (${ca.ecu_tcha_debounce_cyc} cyc -> ${ca.ecu_tcha_min_torque_nm} Nm)` : "DISABLED (65535)"}`, cb ? `${cb.ecu_tcha_limit_active ? `ACTIVE (${cb.ecu_tcha_debounce_cyc} cyc -> ${cb.ecu_tcha_min_torque_nm} Nm)` : "DISABLED (65535)"}` : "—", "Causes WOT 200 Nm drop & 17 Nm/cycle surge in Sleepy Loud bin"],
    ["ECU", "RALv1 Antilag Code Cave", "0x808DC000 (+0x3DC000)", `${ca.ecu_ral_patch_detected ? "DETECTED" : "Not Present"}`, cb ? `${cb.ecu_ral_patch_detected ? "DETECTED" : "Not Present"}` : "—", "Builds 100% boost target on the brake-stand launch line"],
    ["TCU", "Launch Control Brake-Stall Limit", "SW 0x238B68", `${ca.tcu_launch_stall_rpm} RPM`, cb ? `${cb.tcu_launch_stall_rpm} RPM` : "—", "Maximum engine stall RPM while holding brake + WOT"],
    ["TCU", "Primary Torque Protection Cap", "SW 0x1FD198 / 0x2096E4", `${ca.tcu_primary_torque_cap_nm} Nm (Global ${ca.tcu_global_torque_cap_nm} Nm)`, cb ? `${cb.tcu_primary_torque_cap_nm} Nm (Global ${cb.tcu_global_torque_cap_nm} Nm)` : "—", "Continuous CAN 0x162 torque limit ceiling"],
    ["TCU", "Sport WOT Upshift Speeds (1->2..7->8)", "SW 0x205FD8 (Slot 23)", `[${(ca.tcu_sport_upshift_kmh || []).join(", ")}] km/h`, cb ? `[${(cb.tcu_sport_upshift_kmh || []).join(", ")}] km/h` : "—", "Full-throttle upshift schedule in Sport mode"],
    ["TCU", "Shift Torque Cut & Timer", "SW 0x233D3C / 0x20FC52", `Cut=${ca.tcu_shift_cut_raw} | Timer=${ca.tcu_shift_timer_ms}ms | VCM=${ca.tcu_vcm_solenoid_floor_ma}mA`, cb ? `Cut=${cb.tcu_shift_cut_raw} | Timer=${cb.tcu_shift_timer_ms}ms | VCM=${cb.tcu_vcm_solenoid_floor_ma}mA` : "—", "Controls inertia-phase torque reduction depth & shift speed"],
    ["AWD", "Brake-Pressure Inhibit Gate", "0xC07498 / 0xC06080", `${ca.awd_brake_gate_bypassed ? "BYPASSED (Pre-Staged)" : `ACTIVE (> ${ca.awd_brake_gate_bar} bar / 64.5 psi -> 0 Nm)`}`, cb ? `${cb.awd_brake_gate_bypassed ? "BYPASSED (Pre-Staged)" : `ACTIVE (> ${cb.awd_brake_gate_bar} bar / 64.5 psi -> 0 Nm)`}` : "—", "Stock forces 0 Nm AWD coupling while holding brake > 64.5 psi"],
    ["AWD", "Coupling Positive Slew Rate", "0xC07650", `${ca.awd_slew_rate_nm_20ms} Nm/20ms (${ca.awd_slew_rate_nm_sec} Nm/s)`, cb ? `${cb.awd_slew_rate_nm_20ms} Nm/20ms (${cb.awd_slew_rate_nm_sec} Nm/s)` : "—", "Maximum rate of front transfer clutch engagement after release"],
    ["AWD", "Sport WOT Feed-Forward Curve", "0xC04BA0 on 0xC06692", `[${(ca.awd_feed_forward_wot_nm || []).join(", ")}] Nm`, cb ? `[${(cb.awd_feed_forward_wot_nm || []).join(", ")}] Nm` : "—", "Open-loop coupling torque vs. vehicle speed at 100% pedal"],
    ["AWD", "Hardcoded Tire Rolling Radius", "0xC04214 (R_TIRE)", `${ca.awd_tire_radius_mm} mm`, cb ? `${cb.awd_tire_radius_mm} mm` : "—", "Used by AWD wheel-speed slip comparator (staggered tires trigger 0xC061FA)"],
  ];

  const calTbody = document.querySelector("#tblCalComparison tbody");
  calTbody.innerHTML = rows
    .map(
      (r) => `
    <tr>
      <td><strong>${r[0]}</strong></td>
      <td>${escapeHtml(r[1])}</td>
      <td><code>${escapeHtml(r[2])}</code></td>
      <td style="color:var(--col-a)">${escapeHtml(r[3])}</td>
      <td style="color:var(--col-b)">${escapeHtml(r[4])}</td>
      <td style="color:var(--text-muted)">${escapeHtml(r[5])}</td>
    </tr>`
    )
    .join("");
}

// BINGO SQLITE DB MAP BROWSER & BINARY HEX DIFF
async function loadDbMapList() {
  const mod = document.getElementById("dbModuleSelect").value;
  const search = document.getElementById("dbSearchInput").value.trim();
  try {
    const res = await fetch(`/api/maps?module=${encodeURIComponent(mod)}&search=${encodeURIComponent(search)}&limit=250`);
    const maps = await res.json();
    const tbody = document.querySelector("#tblDbMaps tbody");
    if (!maps || maps.length === 0) {
      tbody.innerHTML = `<tr><td colspan="5">No matching calibration maps found</td></tr>`;
      return;
    }
    tbody.innerHTML = maps
      .map(
        (m) => `
      <tr style="cursor:pointer" onclick="compareSpecificMap('${m.module}', '${m.address_hex}')">
        <td><code>${escapeHtml(m.address_hex)}</code></td>
        <td>${escapeHtml(m.name)}</td>
        <td>${m.rows}&times;${m.cols}</td>
        <td>${escapeHtml(m.element_type)}</td>
        <td>${escapeHtml(m.category)}</td>
      </tr>`
      )
      .join("");
  } catch (err) {
    console.error("Failed to load DB maps:", err);
  }
}

async function compareSpecificMap(mod, addrHex) {
  let binA = "";
  let binB = "";
  if (mod === "ecu") {
    binA = document.getElementById("runA_ecuBin").value;
    binB = document.getElementById("runB_ecuBin").value;
  } else if (mod === "tcu") {
    binA = document.getElementById("runA_tcuBin").value;
    binB = document.getElementById("runB_tcuBin").value;
  } else if (mod === "awd") {
    binA = document.getElementById("runA_awdBin").value;
    binB = document.getElementById("runB_awdBin").value;
  }

  try {
    const url = `/api/maps/compare?module=${encodeURIComponent(mod)}&addr=${encodeURIComponent(addrHex)}&bin_a=${encodeURIComponent(binA)}&bin_b=${encodeURIComponent(binB)}`;
    const res = await fetch(url);
    const data = await res.json();
    if (!res.ok) {
      document.getElementById("mapDiffHeader").textContent = "Error: " + (data.error || res.statusText);
      return;
    }
    lastMapCompare = data;
    renderMapComparisonGrid();
  } catch (err) {
    console.error("Map compare failed:", err);
  }
}

function renderMapComparisonGrid() {
  if (!lastMapCompare) return;
  const m = lastMapCompare.map;
  const gA = lastMapCompare.grid_a;
  const gB = lastMapCompare.grid_b;

  document.getElementById("mapDiffHeader").innerHTML = `
    <strong>[${m.module.toUpperCase()}] ${escapeHtml(m.name)}</strong> @ <code>${escapeHtml(m.address_hex)}</code>
    (${m.rows}&times;${m.cols} ${escapeHtml(m.element_type)}, scale=${m.scale_factor}, units=${escapeHtml(m.units || "raw")})
    &mdash; <strong>Changed Cells: ${lastMapCompare.changed_cells}</strong> | Max |&Delta;| = ${lastMapCompare.max_abs_delta}
    <div style="color:var(--text-muted);font-size:10.5px;margin-top:2px;">${escapeHtml(m.description || "")}</div>
  `;

  const container = document.getElementById("mapGridContainer");
  let grid = null;
  if (currentGridMode === "a" && gA) grid = gA.values;
  else if (currentGridMode === "b" && gB) grid = gB.values;
  else if (lastMapCompare.delta_values) grid = lastMapCompare.delta_values;
  else if (gA) grid = gA.values;

  if (!grid || grid.length === 0) {
    container.innerHTML = `<div style="padding:12px;color:var(--text-muted)">No decoded grid data available.</div>`;
    return;
  }

  const rows = grid.length;
  const cols = grid[0].length;
  const maxAbs = Math.max(1e-6, lastMapCompare.max_abs_delta || 1);

  let html = `<table class="heatmap-table"><thead><tr><th>R\\C</th>`;
  for (let c = 0; c < cols; c++) html += `<th>C${c}</th>`;
  html += `</tr></thead><tbody>`;

  for (let r = 0; r < rows; r++) {
    html += `<tr><th>R${r}</th>`;
    for (let c = 0; c < cols; c++) {
      const v = grid[r][c];
      let bg = "transparent";
      let fg = "#cbd5e1";
      if (currentGridMode === "delta") {
        if (Math.abs(v) > 1e-5) {
          const alpha = Math.min(0.65, 0.15 + 0.5 * (Math.abs(v) / maxAbs));
          bg = v > 0 ? `rgba(0, 230, 118, ${alpha.toFixed(2)})` : `rgba(255, 82, 82, ${alpha.toFixed(2)})`;
          fg = "#ffffff";
        } else {
          fg = "#475569";
        }
      }
      const disp = Number.isInteger(v) ? String(v) : v.toFixed(2);
      html += `<td style="background:${bg};color:${fg}">${currentGridMode === "delta" && v > 0 ? "+" : ""}${disp}</td>`;
    }
    html += `</tr>`;
  }
  html += `</tbody></table>`;
  container.innerHTML = html;
}

// EXPORT C-CAN LOG & TELEMETRY CSV
async function exportCanDump() {
  const payload = buildSimRequestPayload();
  const res = await fetch("/api/export/candump", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) return;
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "simulation_candump.log";
  a.click();
  URL.revokeObjectURL(url);
}

function exportTelemetryCsv() {
  if (!lastSimResult || !lastSimResult.run_a.telemetry) return;
  const telA = lastSimResult.run_a.telemetry;
  const telB = lastSimResult.run_b ? lastSimResult.run_b.telemetry : [];

  const headers = [
    "time_s",
    "A_speed_mph", "A_speed_kmh", "A_accel_g", "A_gear", "A_rpm", "A_eng_nm", "A_spark_deg", "A_boost_psi", "A_awd_nm", "A_slip_r_pct",
    "B_speed_mph", "B_speed_kmh", "B_accel_g", "B_gear", "B_rpm", "B_eng_nm", "B_spark_deg", "B_boost_psi", "B_awd_nm", "B_slip_r_pct",
  ];
  const lines = [headers.join(",")];
  for (let i = 0; i < telA.length; i++) {
    const a = telA[i];
    const b = telB[i] || {};
    lines.push(
      [
        a.t,
        a.v_mph, a.v_kmh, a.accel_g, a.gear, a.rpm, a.eng_nm, a.iga_deg, a.boost_psi, a.awd_actual_nm, a.slip_r_pct,
        b.v_mph ?? "", b.v_kmh ?? "", b.accel_g ?? "", b.gear ?? "", b.rpm ?? "", b.eng_nm ?? "", b.iga_deg ?? "", b.boost_psi ?? "", b.awd_actual_nm ?? "", b.slip_r_pct ?? "",
      ].join(",")
    );
  }
  const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "simulation_master_ab_telemetry.csv";
  link.click();
  URL.revokeObjectURL(url);
}

function escapeHtml(str) {
  return String(str || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

// BROWSER FILE PICKER & LOCAL STORAGE FOLDER TREE MANAGER
function setupImportModal() {
  const modal = document.getElementById("importModal");
  const btnOpen = document.getElementById("btnOpenImportModal");
  const btnClose = document.getElementById("btnCloseImportModal");
  const btnBrowse = document.getElementById("btnTriggerFilePicker");
  const fileInput = document.getElementById("filePickerInput");
  const quickInput = document.getElementById("quickFileInput");
  const dropzone = document.getElementById("importDropzone");

  if (btnOpen) {
    btnOpen.addEventListener("click", () => openImportModal("auto"));
  }
  if (btnClose) {
    btnClose.addEventListener("click", () => closeImportModal());
  }
  if (modal) {
    modal.addEventListener("click", (e) => {
      if (e.target === modal) closeImportModal();
    });
  }
  if (btnBrowse && fileInput) {
    btnBrowse.addEventListener("click", () => fileInput.click());
    fileInput.addEventListener("change", async (e) => {
      if (e.target.files && e.target.files.length > 0) {
        const targetMod = document.getElementById("importTargetSelect").value || "auto";
        await uploadFilesToStorage(e.target.files, targetMod, null);
        fileInput.value = "";
      }
    });
  }
  if (quickInput) {
    quickInput.addEventListener("change", async (e) => {
      if (e.target.files && e.target.files.length > 0) {
        await uploadFilesToStorage(e.target.files, quickImportState.target, quickImportState.assignSelectId);
        quickInput.value = "";
      }
    });
  }

  document.querySelectorAll(".btn-quick-import").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      quickImportState.target = btn.dataset.target || "auto";
      quickImportState.assignSelectId = btn.dataset.assign || null;
      if (quickInput) quickInput.click();
    });
  });

  if (dropzone) {
    dropzone.addEventListener("dragover", (e) => {
      e.preventDefault();
      dropzone.classList.add("dragover");
    });
    dropzone.addEventListener("dragleave", () => {
      dropzone.classList.remove("dragover");
    });
    dropzone.addEventListener("drop", async (e) => {
      e.preventDefault();
      dropzone.classList.remove("dragover");
      if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
        const targetMod = document.getElementById("importTargetSelect").value || "auto";
        await uploadFilesToStorage(e.dataTransfer.files, targetMod, null);
      }
    });
  }
}

async function openImportModal(defaultTarget) {
  const modal = document.getElementById("importModal");
  if (!modal) return;
  if (defaultTarget) {
    const sel = document.getElementById("importTargetSelect");
    if (sel) sel.value = defaultTarget;
  }
  modal.classList.add("open");
  modal.setAttribute("aria-hidden", "false");
  await loadStorageTree();
}

function closeImportModal() {
  const modal = document.getElementById("importModal");
  if (!modal) return;
  modal.classList.remove("open");
  modal.setAttribute("aria-hidden", "true");
}

function triggerFolderImport(module) {
  quickImportState.target = module || "auto";
  quickImportState.assignSelectId = null;
  const quickInput = document.getElementById("quickFileInput");
  if (quickInput) quickInput.click();
}

async function uploadFilesToStorage(fileList, targetMod, assignSelectId) {
  const banner = document.getElementById("importStatusBanner");
  if (banner) {
    banner.style.display = "block";
    banner.className = "import-status-banner";
    banner.textContent = `Uploading ${fileList.length} file(s) into local storage/ folder tree...`;
  }

  const formData = new FormData();
  formData.append("target", targetMod || "auto");
  for (let i = 0; i < fileList.length; i++) {
    formData.append("files", fileList[i]);
  }

  try {
    const res = await fetch("/api/import", {
      method: "POST",
      body: formData,
    });
    const data = await res.json();
    if (!res.ok) {
      if (banner) {
        banner.className = "import-status-banner err";
        banner.textContent = "Import Error: " + (data.error || res.statusText);
      } else {
        alert("Import Error: " + (data.error || res.statusText));
      }
      return;
    }

    if (data.inventory) {
      inventory = data.inventory;
      renderInventoryUI(inventory, true);
    }
    if (data.storage) {
      renderStorageTree(data.storage);
    }

    if (data.imported && data.imported.length > 0) {
      const summary = data.imported
        .map((f) => `${f.filename} -> ${f.rel_path} (${formatBytes(f.size_bytes)})`)
        .join(" | ");
      if (banner) {
        banner.className = "import-status-banner";
        banner.textContent = `Imported ${data.imported.length} file(s): ${summary}`;
      }

      // If triggered from a specific Run A / Run B binary selector, select the newly imported .bin and re-run
      if (assignSelectId) {
        const firstBin = data.imported.find((f) => f.category === "bin");
        const selEl = document.getElementById(assignSelectId);
        if (firstBin && selEl) {
          selEl.value = firstBin.saved_path;
        }
      }
      await runSimulation();
    }
  } catch (err) {
    console.error("Upload failed:", err);
    if (banner) {
      banner.className = "import-status-banner err";
      banner.textContent = "Upload failed: " + err.message;
    }
  }
}

async function loadStorageTree() {
  try {
    const res = await fetch("/api/storage");
    const data = await res.json();
    renderStorageTree(data);
  } catch (err) {
    console.error("Failed loading storage tree:", err);
  }
}

function renderStorageTree(storage) {
  if (!storage) return;
  const rootEl = document.getElementById("storageRootPath");
  if (rootEl) rootEl.textContent = storage.root_dir || "./storage";

  const badgeEl = document.getElementById("storageSummaryBadge");
  if (badgeEl) {
    badgeEl.textContent = `${storage.total_files || 0} files (${formatBytes(storage.total_bytes || 0)})`;
  }

  const grid = document.getElementById("storageTreeGrid");
  if (!grid) return;

  grid.innerHTML = (storage.directories || [])
    .map((dir) => {
      const filesHtml =
        dir.files && dir.files.length > 0
          ? dir.files
              .map(
                (f) => `
              <div class="storage-file-row" title="${escapeHtml(f.full_path)}">
                <span class="storage-file-name">${escapeHtml(f.name)}</span>
                <span class="storage-file-size">${formatBytes(f.size_bytes)}</span>
              </div>`
              )
              .join("")
          : `<div class="storage-empty">Empty folder — click + Import to add .${dir.category}</div>`;

      return `
        <div class="storage-dir-card">
          <div class="storage-dir-top">
            <span class="storage-dir-path">&#128193; ${escapeHtml(dir.rel_path)}/</span>
            <button type="button" class="btn-quick-import" onclick="triggerFolderImport('${dir.module}')">+ Import .${dir.category.toUpperCase()}</button>
          </div>
          <div class="storage-dir-desc">${escapeHtml(dir.description)}</div>
          <div class="storage-file-list">${filesHtml}</div>
        </div>
      `;
    })
    .join("");
}

function formatBytes(bytes) {
  if (!bytes || bytes <= 0) return "0 B";
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(2) + " MB";
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + " KB";
  return bytes + " B";
}

