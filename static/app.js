// Simulation Master — Client Telemetry & Firmware Studio (app.js)

let inventory = null;
let lastSimResult = null;
let lastMapCompare = null;
let currentGridMode = "delta";
let hoverTimeSec = null;

let quickImportState = { target: "auto", assignSelectId: null };

document.addEventListener("DOMContentLoaded", async () => {
  setupPageNavigation();
  setupTabs();
  setupControls();
  setupChassis3DStudio();
  setupImportModal();
  await loadInventory();
});

function switchPage(pageId) {
  document.querySelectorAll(".page-view").forEach((pv) => pv.classList.remove("active"));
  document.querySelectorAll(".page-nav-btn").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.page === pageId);
  });
  const target = document.getElementById(pageId);
  if (target) target.classList.add("active");
  if (pageId === "pageSimulation") {
    updateActiveConfigSummaryBar();
    renderAllCharts();
    renderChassis3DScene();
  } else if (lastSimResult) {
    renderMuSlipCurve(lastSimResult.mu_slip_curve, lastSimResult.surface);
  }
}

function setupPageNavigation() {
  document.querySelectorAll(".page-nav-btn").forEach((btn) => {
    btn.addEventListener("click", () => switchPage(btn.dataset.page));
  });
  const btnJump = document.getElementById("btnJumpToConfig");
  if (btnJump) {
    btnJump.addEventListener("click", () => switchPage("pageConfiguration"));
  }
  const btnApplyTop = document.getElementById("btnApplyAndReturnTop");
  if (btnApplyTop) {
    btnApplyTop.addEventListener("click", async () => {
      await runSimulation();
      switchPage("pageSimulation");
    });
  }
}

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
      if (btn.dataset.tab === "tab-chassis-3d") {
        renderChassis3DScene();
      }
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

  const inpAirflow = document.getElementById("inpAirflowRestrPct");
  if (inpAirflow) {
    inpAirflow.addEventListener("input", (e) => {
      const v = parseFloat(e.target.value) || 0;
      const lbl = document.getElementById("lblAirflowRestrPct");
      if (lbl) {
        lbl.textContent = v === 0 ? "0% (Unrestricted)" : `${v.toFixed(0)}% Restricted`;
      }
    });
    inpAirflow.addEventListener("change", () => runSimulation());
  }

  document.getElementById("chkEnableRunB").addEventListener("change", () => runSimulation());

  // Bind Turbo Wheel Preset <-> Custom Dimension (mm) synchronization for Run A & Run B
  const turboPresetMap = {
    stock_mgt1549: { compIn: 37.2, compEx: 49.0, turbIn: 40.0, turbEx: 35.5 },
    stage1_41_52: { compIn: 41.0, compEx: 52.0, turbIn: 42.5, turbEx: 38.0 },
    stage2_44_56: { compIn: 44.0, compEx: 56.0, turbIn: 45.0, turbEx: 40.5 },
    stage3_47_60: { compIn: 47.0, compEx: 60.0, turbIn: 49.0, turbEx: 44.0 },
  };
  ["runA", "runB"].forEach((prefix) => {
    const presetSel = document.getElementById(`${prefix}_turboPreset`);
    if (presetSel) {
      presetSel.addEventListener("change", (e) => {
        const dims = turboPresetMap[e.target.value];
        if (dims) {
          document.getElementById(`${prefix}_compInMm`).value = dims.compIn.toFixed(1);
          document.getElementById(`${prefix}_compExMm`).value = dims.compEx.toFixed(1);
          document.getElementById(`${prefix}_turbInMm`).value = dims.turbIn.toFixed(1);
          document.getElementById(`${prefix}_turbExMm`).value = dims.turbEx.toFixed(1);
        }
      });
    }
    ["compInMm", "compExMm", "turbInMm", "turbExMm"].forEach((field) => {
      const inp = document.getElementById(`${prefix}_${field}`);
      if (inp && presetSel) {
        inp.addEventListener("input", () => {
          presetSel.value = "custom";
        });
      }
    });
  });

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

  window.addEventListener("resize", () => {
    renderAllCharts();
    renderChassis3DScene();
  });
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
  if (document.getElementById("inpAirflowRestrPct")) {
    const af = env.airflow_restriction_pct || 0;
    document.getElementById("inpAirflowRestrPct").value = af;
    const lbl = document.getElementById("lblAirflowRestrPct");
    if (lbl) lbl.textContent = af === 0 ? "0% (Unrestricted)" : `${af}% Restricted`;
  }

  applyRunConfigToForm("runA", preset.run_a);
  if (preset.run_b) {
    document.getElementById("chkEnableRunB").checked = true;
    applyRunConfigToForm("runB", preset.run_b);
  }
  updateActiveConfigSummaryBar();
}

function updateActiveConfigSummaryBar() {
  const surfSel = document.getElementById("surfaceSelect");
  const surfText = surfSel && surfSel.selectedOptions[0] ? surfSel.selectedOptions[0].text : "Surface";
  const af = parseFloat(document.getElementById("inpAirflowRestrPct")?.value) || 0;
  const envPill = document.getElementById("summaryEnvPill");
  if (envPill) {
    envPill.innerHTML = `${escapeHtml(surfText)} &bull; ${document.getElementById("inpMassKg")?.value || 1915}kg &bull; Airflow Restr: <strong>${af.toFixed(0)}%</strong>`;
  }

  const fmtRunPill = (prefix, slotName, dotClass) => {
    const lbl = document.getElementById(`${prefix}_label`)?.value || slotName;
    const fuel = (document.getElementById(`${prefix}_fuelType`)?.value || "93_aki").toUpperCase();
    const cIn = document.getElementById(`${prefix}_compInMm`)?.value || "37.2";
    const cEx = document.getElementById(`${prefix}_compExMm`)?.value || "49.0";
    const exhSel = document.getElementById(`${prefix}_exhaustType`);
    const exhText = exhSel && exhSel.selectedOptions[0] ? exhSel.selectedOptions[0].text : "Exhaust";
    return `<i class="dot ${dotClass}"></i> <strong>${escapeHtml(lbl)}</strong>: ${fuel} | Turbo ${cIn}/${cEx}mm | ${escapeHtml(exhText)}`;
  };

  const pillA = document.getElementById("summaryRunAPill");
  if (pillA) pillA.innerHTML = fmtRunPill("runA", "Run A", "dot-a");

  const pillB = document.getElementById("summaryRunBPill");
  if (pillB) {
    if (document.getElementById("chkEnableRunB")?.checked) {
      pillB.innerHTML = fmtRunPill("runB", "Run B", "dot-b");
    } else {
      pillB.innerHTML = `<i class="dot dot-b"></i> <strong>Run B:</strong> Disabled`;
    }
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
  if (document.getElementById(`${prefix}_exhaustType`)) {
    document.getElementById(`${prefix}_exhaustType`).value = cfg.exhaust_type || "sec_cat_delete";
  }
  if (document.getElementById(`${prefix}_turboPreset`)) {
    document.getElementById(`${prefix}_turboPreset`).value = cfg.turbo_preset || "stock_mgt1549";
  }
  if (document.getElementById(`${prefix}_compInMm`)) {
    document.getElementById(`${prefix}_compInMm`).value = (cfg.comp_inducer_mm || 37.2).toFixed(1);
    document.getElementById(`${prefix}_compExMm`).value = (cfg.comp_exducer_mm || 49.0).toFixed(1);
    document.getElementById(`${prefix}_turbInMm`).value = (cfg.turb_inducer_mm || 40.0).toFixed(1);
    document.getElementById(`${prefix}_turbExMm`).value = (cfg.turb_exducer_mm || 35.5).toFixed(1);
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
    exhaust_type: document.getElementById(`${prefix}_exhaustType`)
      ? document.getElementById(`${prefix}_exhaustType`).value
      : "sec_cat_delete",
    turbo_preset: document.getElementById(`${prefix}_turboPreset`)
      ? document.getElementById(`${prefix}_turboPreset`).value
      : "stock_mgt1549",
    comp_inducer_mm: parseFloat(document.getElementById(`${prefix}_compInMm`)?.value) || 37.2,
    comp_exducer_mm: parseFloat(document.getElementById(`${prefix}_compExMm`)?.value) || 49.0,
    turb_inducer_mm: parseFloat(document.getElementById(`${prefix}_turbInMm`)?.value) || 40.0,
    turb_exducer_mm: parseFloat(document.getElementById(`${prefix}_turbExMm`)?.value) || 35.5,
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
    airflow_restriction_pct: parseFloat(document.getElementById("inpAirflowRestrPct")?.value) || 0,
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
    updateActiveConfigSummaryBar();
    renderMuSlipCurve(data.mu_slip_curve, data.surface);
    renderKpiStrip(data);
    renderCalDiffBanner(data);
    renderShiftAndCalTables(data);
    syncChassisTimelineWithResult(data);
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

  drawScopeChart("canvasEcu4", telA, telB, {
    tMin: -0.5,
    tMax: null,
    leftUnit: "EMAP PSI",
    leftMin: 0,
    leftMax: 65,
    rightUnit: "Turbo kRPM",
    rightMin: 0,
    rightMax: 240,
    seriesLeft: { key: "emap_psi", label: "Exhaust Manifold Backpressure EMAP (psi)", colorA: "#ff5252", colorB: "#ff9100" },
    seriesRight: { key: "turbo_krpm", label: "Turbo Shaft Speed (kRPM)", colorA: "#00e5ff", colorB: "#00e676" },
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

  // Tab: 3D Chassis Thermal & Stress Timeline Scopes
  drawScopeChart("canvasThermal1", telA, telB, {
    tMin: null,
    tMax: null,
    leftUnit: "EGT °C",
    leftMin: 350,
    leftMax: 1000,
    rightUnit: "Fluid/Clutch °C",
    rightMin: 40,
    rightMax: 130,
    seriesLeft: { key: "temp_egt_c", label: "Exhaust Manifold EGT (°C)", colorA: "#ff5252", colorB: "#ff9100" },
    seriesRight: { key: "temp_trans_c", label: "8AT Trans ATF Temp (°C)", colorA: "#00e5ff", colorB: "#00e676" },
  });

  drawScopeChart("canvasThermal2", telA, telB, {
    tMin: null,
    tMax: null,
    leftUnit: "Stress %",
    leftMin: 0,
    leftMax: 105,
    rightUnit: "Slip kW",
    rightMin: 0,
    rightMax: 180,
    seriesLeft: { key: "stress_trans_pct", label: "8AT / Drivetrain Stress (%)", colorA: "#00e5ff", colorB: "#ff9100" },
    seriesRight: { key: "tc_slip_kw", label: "TC + Clutch Slip Dissipation (kW)", colorA: "#b388ff", colorB: "#ffd740" },
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
  let txt = `t=${sa.t >= 0 ? "+" : ""}${sa.t.toFixed(2)}s | [A] ${sa.v_mph.toFixed(1)} mph (G${sa.gear}, ${sa.rpm.toFixed(0)} RPM, ${sa.eng_nm.toFixed(0)} Nm, Boost ${sa.boost_psi.toFixed(1)} / EMAP ${(sa.emap_psi || 0).toFixed(1)} psi, Spark ${sa.iga_deg.toFixed(1)}° / KR -${(sa.kr_deg || 0).toFixed(1)}°, CamOvl ${(sa.cam_overlap_deg || 0).toFixed(0)}°CA)`;
  if (sb) {
    txt += `  vs  [B] ${sb.v_mph.toFixed(1)} mph (G${sb.gear}, ${sb.rpm.toFixed(0)} RPM, ${sb.eng_nm.toFixed(0)} Nm, Boost ${sb.boost_psi.toFixed(1)} / EMAP ${(sb.emap_psi || 0).toFixed(1)} psi, Spark ${sb.iga_deg.toFixed(1)}° / KR -${(sb.kr_deg || 0).toFixed(1)}°, CamOvl ${(sb.cam_overlap_deg || 0).toFixed(0)}°CA)`;
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

// ============================================================================
// INTERACTIVE 3D KIA STINGER GT (CK) WIREFRAME, SUB-MODULE STRESS & THERMAL STUDIO
// ============================================================================
const chassis3DState = {
  yaw: -0.52,
  pitch: 0.44,
  dist: 5.1,
  targetX: 0.0,
  targetY: 0.42,
  targetZ: 0.0,
  goalYaw: -0.52,
  goalPitch: 0.44,
  goalDist: 5.1,
  goalTargetX: 0.0,
  goalTargetY: 0.42,
  goalTargetZ: 0.0,
  focusModule: "all",
  activeRun: "B",
  heatMode: "stress",
  frameIdx: 0,
  playing: false,
  playSpeed: 0.5,
  wheelAngleFront: 0,
  wheelAngleRear: 0,
  airflowPhase: 0,
  dragging: false,
  lastMouseX: 0,
  lastMouseY: 0,
  animRafId: null,
  lastTickMs: 0,
};

const MODULE_CAMERA_TARGETS = {
  all: { x: 0.0, y: 0.42, z: 0.0, dist: 5.1, yaw: -0.52, pitch: 0.44, label: "ALL MODULES [1-7] (FULL STINGER GT)" },
  engine: { x: 1.42, y: 0.46, z: 0.0, dist: 2.25, yaw: -0.45, pitch: 0.52, label: "[1] 3.3L TWIN-TURBO V6 & FMIC" },
  trans: { x: 0.66, y: 0.36, z: 0.0, dist: 2.05, yaw: -0.35, pitch: 0.48, label: "[2] A8LR1 8AT & TORQUE CONVERTER" },
  awd: { x: 0.16, y: 0.36, z: 0.16, dist: 1.75, yaw: -0.30, pitch: 0.58, label: "[5] MAGNA ATC AWD TRANSFER CASE" },
  driveline_f: { x: 0.88, y: 0.35, z: 0.24, dist: 2.35, yaw: -0.25, pitch: 0.62, label: "[6,7] FRONT PROPSHAFT, DIFF & SUMP AXLE" },
  driveline_r: { x: -0.82, y: 0.35, z: 0.0, dist: 2.45, yaw: 0.35, pitch: 0.46, label: "[3,4] REAR PROPSHAFT, LSD & AXLES" },
  tires: { x: 0.0, y: 0.34, z: 0.0, dist: 4.6, yaw: -0.38, pitch: 0.35, label: "TIRES, WHEELS & BREMBO BRAKES" },
};

function setupChassis3DStudio() {
  const canvas = document.getElementById("canvasChassis3D");
  if (!canvas) return;

  // Mouse Orbit & Wheel Zoom
  canvas.addEventListener("mousedown", (e) => {
    chassis3DState.dragging = true;
    chassis3DState.lastMouseX = e.clientX;
    chassis3DState.lastMouseY = e.clientY;
  });
  window.addEventListener("mousemove", (e) => {
    if (!chassis3DState.dragging) return;
    const dx = e.clientX - chassis3DState.lastMouseX;
    const dy = e.clientY - chassis3DState.lastMouseY;
    chassis3DState.lastMouseX = e.clientX;
    chassis3DState.lastMouseY = e.clientY;
    chassis3DState.yaw -= dx * 0.0085;
    chassis3DState.pitch = Math.max(-1.25, Math.min(1.52, chassis3DState.pitch + dy * 0.0075));
    chassis3DState.goalYaw = chassis3DState.yaw;
    chassis3DState.goalPitch = chassis3DState.pitch;
    renderChassis3DScene();
  });
  window.addEventListener("mouseup", () => {
    chassis3DState.dragging = false;
  });
  canvas.addEventListener("wheel", (e) => {
    e.preventDefault();
    const factor = e.deltaY > 0 ? 1.09 : 0.91;
    chassis3DState.dist = Math.max(1.35, Math.min(9.0, chassis3DState.dist * factor));
    chassis3DState.goalDist = chassis3DState.dist;
    renderChassis3DScene();
  });

  // Sub-Module Focus Isolation Buttons
  document.querySelectorAll(".mod-focus-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
      setChassisSubmoduleFocus(btn.dataset.focus || "all");
    });
  });

  // Camera Angle Presets (Aligned 1-for-1 with the Stinger Service Manual Schematic)
  document.querySelectorAll(".cam-preset-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
      const mode = btn.dataset.cam;
      const basePreset = MODULE_CAMERA_TARGETS[chassis3DState.focusModule] || MODULE_CAMERA_TARGETS.all;
      chassis3DState.goalTargetX = basePreset.x;
      chassis3DState.goalTargetY = basePreset.y;
      chassis3DState.goalTargetZ = basePreset.z;

      if (mode === "iso") {
        // 3/4 Isometric View (Front [1] on Left, Rear [3] on Right, Transfer Case [5] & Front Propshaft [6] on Top-Left)
        chassis3DState.goalYaw = -0.52;
        chassis3DState.goalPitch = 0.44;
        chassis3DState.goalDist = basePreset.dist;
      } else if (mode === "side") {
        // Pure Side Squat Profile (Front [1] on Left, Rear [3] on Right, Roof +Y on Top)
        chassis3DState.goalYaw = 0.0;
        chassis3DState.goalPitch = 0.05;
        chassis3DState.goalDist = chassis3DState.focusModule === "all" ? 5.0 : basePreset.dist;
      } else if (mode === "top") {
        // Exact Top-Down Schematic Plan View (Matches uploaded diagram: [1] Front Left, [3] Rear Right, [5,6,7] Top RH side)
        chassis3DState.goalYaw = 0.0;
        chassis3DState.goalPitch = 1.50;
        chassis3DState.goalDist = chassis3DState.focusModule === "all" ? 5.2 : basePreset.dist * 1.15;
      } else if (mode === "under") {
        // Underbody AWD & Exhaust View (Looking up from below ground)
        chassis3DState.goalYaw = -0.38;
        chassis3DState.goalPitch = -0.58;
        chassis3DState.goalDist = chassis3DState.focusModule === "all" ? 4.8 : basePreset.dist;
      }
      startChassisSmoothLoop();
    });
  });

  // Run Selector & Shader Mode
  const selRun = document.getElementById("selChassisRun");
  if (selRun) {
    selRun.addEventListener("change", (e) => {
      chassis3DState.activeRun = e.target.value;
      renderChassis3DScene();
    });
  }
  const selHeat = document.getElementById("selChassisHeatMode");
  if (selHeat) {
    selHeat.addEventListener("change", (e) => {
      chassis3DState.heatMode = e.target.value;
      renderChassis3DScene();
    });
  }

  // Timeline Scrubber & Play/Pause
  const rng = document.getElementById("rngChassisTimeline");
  if (rng) {
    rng.addEventListener("input", (e) => {
      chassis3DState.frameIdx = parseInt(e.target.value, 10) || 0;
      const tel = getActiveChassisTelemetry();
      if (tel[chassis3DState.frameIdx]) {
        hoverTimeSec = tel[chassis3DState.frameIdx].t;
        updateCursorReadout(hoverTimeSec);
      }
      renderChassis3DScene();
      renderAllCharts();
    });
  }

  const btnPlay = document.getElementById("btnPlayTimeline");
  if (btnPlay) {
    btnPlay.addEventListener("click", () => {
      chassis3DState.playing = !chassis3DState.playing;
      btnPlay.innerHTML = chassis3DState.playing ? "&#9208; PAUSE" : "&#9654; PLAY RUN";
      if (chassis3DState.playing) {
        const tel = getActiveChassisTelemetry();
        if (chassis3DState.frameIdx >= tel.length - 1) {
          chassis3DState.frameIdx = 0;
        }
        chassis3DState.lastTickMs = performance.now();
        startChassisSmoothLoop();
      }
    });
  }

  const selSpeed = document.getElementById("selPlaySpeed");
  if (selSpeed) {
    selSpeed.addEventListener("change", (e) => {
      chassis3DState.playSpeed = parseFloat(e.target.value) || 0.5;
    });
  }

  // Quick-Jump Timeline Event Buttons
  document.querySelectorAll(".btn-jump-t").forEach((btn) => {
    btn.addEventListener("click", () => {
      jumpChassisTimelineToEvent(btn.dataset.jump);
    });
  });
}

function setChassisSubmoduleFocus(modId) {
  chassis3DState.focusModule = modId;
  document.querySelectorAll(".mod-focus-btn").forEach((b) => {
    b.classList.toggle("active", b.dataset.focus === modId);
  });
  const preset = MODULE_CAMERA_TARGETS[modId] || MODULE_CAMERA_TARGETS.all;
  chassis3DState.goalTargetX = preset.x;
  chassis3DState.goalTargetY = preset.y;
  chassis3DState.goalTargetZ = preset.z;
  chassis3DState.goalDist = preset.dist;
  chassis3DState.goalYaw = preset.yaw;
  chassis3DState.goalPitch = preset.pitch;
  const badge = document.getElementById("badgeChassisActiveMod");
  if (badge) badge.textContent = preset.label;
  startChassisSmoothLoop();
}

function getActiveChassisRunObj() {
  if (!lastSimResult) return null;
  if (chassis3DState.activeRun === "B" && lastSimResult.run_b) {
    return lastSimResult.run_b;
  }
  return lastSimResult.run_a;
}

function getActiveChassisTelemetry() {
  const runObj = getActiveChassisRunObj();
  return runObj && runObj.telemetry ? runObj.telemetry : [];
}

function syncChassisTimelineWithResult(data) {
  if (!data) return;
  const selRun = document.getElementById("selChassisRun");
  if (selRun) {
    if (!data.run_b) {
      chassis3DState.activeRun = "A";
      selRun.value = "A";
    }
  }
  const tel = getActiveChassisTelemetry();
  const rng = document.getElementById("rngChassisTimeline");
  if (rng && tel.length > 0) {
    rng.max = String(tel.length - 1);
    // Default timeline cursor to launch + 1.8s (near 1->2 shift / peak G) if at 0
    if (chassis3DState.frameIdx === 0 || chassis3DState.frameIdx >= tel.length) {
      const launchIdx = tel.findIndex((s) => s.t >= 1.6);
      chassis3DState.frameIdx = launchIdx >= 0 ? launchIdx : 0;
    }
    rng.value = String(chassis3DState.frameIdx);
  }
  renderChassis3DScene();
}

function jumpChassisTimelineToEvent(jumpType) {
  const runObj = getActiveChassisRunObj();
  if (!runObj || !runObj.telemetry || runObj.telemetry.length === 0) return;
  const tel = runObj.telemetry;
  const shifts = runObj.shifts || [];
  let targetT = 0.05;

  if (jumpType === "stall") {
    targetT = -0.35;
  } else if (jumpType === "launch") {
    targetT = 0.06;
  } else if (jumpType === "shift12") {
    const s = shifts.find((sh) => sh.from_gear === 1);
    targetT = s ? s.start_time_sec + 0.06 : 2.1;
  } else if (jumpType === "shift23") {
    const s = shifts.find((sh) => sh.from_gear === 2);
    targetT = s ? s.start_time_sec + 0.06 : 4.2;
  } else if (jumpType === "shift34") {
    const s = shifts.find((sh) => sh.from_gear === 3);
    targetT = s ? s.start_time_sec + 0.06 : 6.8;
  } else if (jumpType === "finish") {
    targetT = runObj.summary?.quarter_mile_sec > 0 ? runObj.summary.quarter_mile_sec : tel[tel.length - 1].t;
  }

  let bestIdx = 0;
  let bestD = Infinity;
  tel.forEach((s, idx) => {
    const d = Math.abs(s.t - targetT);
    if (d < bestD) {
      bestD = d;
      bestIdx = idx;
    }
  });
  chassis3DState.frameIdx = bestIdx;
  const rng = document.getElementById("rngChassisTimeline");
  if (rng) rng.value = String(bestIdx);
  hoverTimeSec = tel[bestIdx].t;
  updateCursorReadout(hoverTimeSec);
  renderChassis3DScene();
  renderAllCharts();
}

function startChassisSmoothLoop() {
  if (chassis3DState.animRafId) return;
  chassis3DState.lastTickMs = performance.now();

  const step = (nowMs) => {
    const dtSec = Math.min(0.1, (nowMs - chassis3DState.lastTickMs) / 1000.0);
    chassis3DState.lastTickMs = nowMs;

    // Smooth camera interpolation toward goal
    const lerp = 0.18;
    let camMoving = false;
    const dYaw = chassis3DState.goalYaw - chassis3DState.yaw;
    const dPitch = chassis3DState.goalPitch - chassis3DState.pitch;
    const dDist = chassis3DState.goalDist - chassis3DState.dist;
    const dTx = chassis3DState.goalTargetX - chassis3DState.targetX;
    const dTy = chassis3DState.goalTargetY - chassis3DState.targetY;
    const dTz = chassis3DState.goalTargetZ - chassis3DState.targetZ;

    if (
      Math.abs(dYaw) > 0.002 ||
      Math.abs(dPitch) > 0.002 ||
      Math.abs(dDist) > 0.005 ||
      Math.abs(dTx) > 0.002 ||
      Math.abs(dTy) > 0.002 ||
      Math.abs(dTz) > 0.002
    ) {
      chassis3DState.yaw += dYaw * lerp;
      chassis3DState.pitch += dPitch * lerp;
      chassis3DState.dist += dDist * lerp;
      chassis3DState.targetX += dTx * lerp;
      chassis3DState.targetY += dTy * lerp;
      chassis3DState.targetZ += dTz * lerp;
      camMoving = true;
    }

    // Advance timeline if playing
    if (chassis3DState.playing) {
      const tel = getActiveChassisTelemetry();
      if (tel.length > 1) {
        const curSample = tel[chassis3DState.frameIdx] || tel[0];
        const nextT = curSample.t + dtSec * chassis3DState.playSpeed;
        let nextIdx = chassis3DState.frameIdx;
        while (nextIdx < tel.length - 1 && tel[nextIdx].t < nextT) {
          nextIdx++;
        }
        chassis3DState.frameIdx = nextIdx;
        const rng = document.getElementById("rngChassisTimeline");
        if (rng) rng.value = String(nextIdx);
        hoverTimeSec = tel[nextIdx].t;
        updateCursorReadout(hoverTimeSec);

        // Rotate wheels and advance airflow streamlines
        const vfMs = (tel[nextIdx].v_wheel_f_kmh || 0) / 3.6;
        const vrMs = (tel[nextIdx].v_wheel_r_kmh || 0) / 3.6;
        chassis3DState.wheelAngleFront = (chassis3DState.wheelAngleFront + (vfMs / 0.334) * dtSec * 0.6) % (Math.PI * 2);
        chassis3DState.wheelAngleRear = (chassis3DState.wheelAngleRear + (vrMs / 0.334) * dtSec * 0.6) % (Math.PI * 2);
        chassis3DState.airflowPhase = (chassis3DState.airflowPhase + dtSec * (0.8 + (tel[nextIdx].v_mph || 0) * 0.025)) % 1.0;

        renderAllCharts();
        if (nextIdx >= tel.length - 1) {
          chassis3DState.playing = false;
          const btnPlay = document.getElementById("btnPlayTimeline");
          if (btnPlay) btnPlay.innerHTML = "&#9654; PLAY RUN";
        }
      }
    }

    renderChassis3DScene();

    if (chassis3DState.playing || camMoving) {
      chassis3DState.animRafId = requestAnimationFrame(step);
    } else {
      chassis3DState.animRafId = null;
    }
  };

  chassis3DState.animRafId = requestAnimationFrame(step);
}

// Convert normalized 0..100% stress/thermal index to glowing RGBA wireframe color
function stressIndexToColor(pct, alpha = 1.0) {
  const s = Math.max(0, Math.min(100, pct || 0));
  let r = 0, g = 229, b = 255;
  if (s < 35) {
    // Cyan (#00e5ff) -> Emerald (#00e676)
    const u = s / 35.0;
    r = Math.round(0 * (1 - u) + 0 * u);
    g = Math.round(229 * (1 - u) + 230 * u);
    b = Math.round(255 * (1 - u) + 118 * u);
  } else if (s < 70) {
    // Emerald (#00e676) -> Amber (#ffab00)
    const u = (s - 35.0) / 35.0;
    r = Math.round(0 * (1 - u) + 255 * u);
    g = Math.round(230 * (1 - u) + 171 * u);
    b = Math.round(118 * (1 - u) + 0 * u);
  } else {
    // Amber (#ffab00) -> Critical Hot-Spot Red/Magenta (#ff2a2a)
    const u = (s - 70.0) / 30.0;
    r = 255;
    g = Math.round(171 * (1 - u) + 38 * u);
    b = Math.round(0 * (1 - u) + 48 * u);
  }
  return `rgba(${r},${g},${b},${alpha.toFixed(2)})`;
}

// Convert component temperature (deg C) into 0..100% thermal scale for temperature shader mode
function tempToNormalizedPct(tempC, minC, maxC) {
  return Math.max(0, Math.min(100, (((tempC || minC) - minC) / Math.max(1, maxC - minC)) * 100.0));
}

function renderChassis3DScene() {
  const canvas = document.getElementById("canvasChassis3D");
  if (!canvas) return;
  if (canvas.clientWidth > 100 && canvas.clientHeight > 100) {
    if (canvas.width !== canvas.clientWidth || canvas.height !== canvas.clientHeight) {
      canvas.width = canvas.clientWidth;
      canvas.height = canvas.clientHeight;
    }
  }
  const ctx = canvas.getContext("2d");
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  const tel = getActiveChassisTelemetry();
  const sample = tel[chassis3DState.frameIdx] || tel[0] || {
    t: 0,
    v_mph: 0,
    v_kmh: 0,
    accel_g: 0,
    gear: 1,
    rpm: 2500,
    eng_nm: 450,
    boost_psi: 12,
    emap_psi: 24,
    brake_psi: 0,
    awd_actual_nm: 500,
    awd_split_pct: 40,
    slip_r_pct: 5,
    slip_f_pct: 3,
    temp_engine_c: 86,
    temp_egt_c: 650,
    temp_iat_c: 32,
    temp_trans_c: 82,
    temp_awd_c: 62,
    temp_rear_diff_c: 56,
    temp_front_diff_c: 52,
    temp_tire_r_c: 38,
    temp_tire_f_c: 35,
    tc_slip_kw: 15,
    awd_slip_kw: 4,
    stress_engine_pct: 45,
    stress_trans_pct: 40,
    stress_awd_pct: 42,
    stress_drive_r_pct: 38,
    stress_drive_f_pct: 30,
    stress_tire_r_pct: 35,
    stress_tire_f_pct: 28,
  };

  // Update timeline readout labels
  const lblSec = document.getElementById("lblTimelineSec");
  if (lblSec) {
    const phaseTag =
      sample.t < 0
        ? "PRE-STAGE BRAKE STALL"
        : sample.shift
        ? `UPSHIFT INERTIA PHASE (GEAR ${sample.gear})`
        : sample.t < 0.35
        ? "LAUNCH BREAKAWAY"
        : `WOT PULL (GEAR ${sample.gear})`;
    lblSec.textContent = `t = ${sample.t >= 0 ? "+" : ""}${sample.t.toFixed(2)} s [${phaseTag}]`;
  }
  const lblState = document.getElementById("lblTimelineState");
  if (lblState) {
    lblState.textContent = `${sample.v_mph.toFixed(1)} mph (${sample.v_kmh.toFixed(1)} km/h) • ${Math.round(sample.rpm)} RPM • ${sample.accel_g.toFixed(2)} G • Boost: ${sample.boost_psi.toFixed(1)} psi • EMAP: ${(sample.emap_psi || 0).toFixed(1)} psi`;
  }

  // Update the 6 Sub-Module Inspector Cards on the right
  renderSubmoduleInspectorCards(sample);

  // Orthonormal Right-Handed Turntable LookAt Camera Basis (Matched to Stinger Service Schematic):
  // World Axes:
  //   +X = Front of Vehicle ([1] V6 Engine, [7] Front Diff) -> Projects to LEFT of screen at yaw=0
  //   -X = Rear of Vehicle ([3] Rear Diff)                  -> Projects to RIGHT of screen at yaw=0
  //   +Y = Vertical Roof Up                                 -> Projects to TOP of screen in Side/Iso views
  //   +Z = RH / Passenger Side ([5] Transfer Case arm, [6] Front Propshaft, [7] Front Diff) -> TOP of screen in Top X-Ray view
  //   -Z = LH / Driver Side (Through-Sump Front Axle Tube)  -> BOTTOM of screen in Top X-Ray view
  const cosY = Math.cos(chassis3DState.yaw);
  const sinY = Math.sin(chassis3DState.yaw);
  const cosP = Math.cos(chassis3DState.pitch);
  const sinP = Math.sin(chassis3DState.pitch);
  const dist = chassis3DState.dist;

  // Camera Eye Position in World Space
  const eyeX = chassis3DState.targetX - dist * cosP * sinY;
  const eyeY = chassis3DState.targetY + dist * sinP;
  const eyeZ = chassis3DState.targetZ - dist * cosP * cosY;

  // Camera Forward Unit Vector F = normalize(Target - Eye)
  const fx = cosP * sinY;
  const fy = -sinP;
  const fz = cosP * cosY;

  // Camera Right Unit Vector R = F x WorldUp (Horizontal Right on Screen; -X Rear is Right, +X Front is Left)
  const rxCam = -cosY;
  const ryCam = 0.0;
  const rzCam = sinY;

  // Camera True Up Unit Vector U = R x F (Vertical Up on Screen; U_y = cosP > 0, U_z = +sinP at yaw=0)
  const uxCam = -sinY * sinP;
  const uyCam = cosP;
  const uzCam = cosY * sinP;

  const fov = Math.min(w, h) * 1.12;

  // Dynamic suspension squat pitch under longitudinal acceleration (nose rises +Y, rear squats -Y under +G)
  const squatPitchRad = (sample.accel_g || 0) * 0.026;
  const cosSq = Math.cos(squatPitchRad);
  const sinSq = Math.sin(squatPitchRad);

  const projectPoint = (x, y, z, applySquat = true) => {
    let wx = x;
    let wy = y;
    let wz = z;
    if (applySquat && y > 0.16) {
      const rotX = wx * cosSq - (wy - 0.334) * sinSq;
      const rotY = (wy - 0.334) * cosSq + wx * sinSq + 0.334;
      wx = rotX;
      wy = rotY;
    }

    // Vector from Camera Eye to World Point
    const vx = wx - eyeX;
    const vy = wy - eyeY;
    const vz = wz - eyeZ;

    // Project onto Orthonormal Camera Basis (R, U, F)
    const xCam = vx * rxCam + vy * ryCam + vz * rzCam;
    const yCam = vx * uxCam + vy * uyCam + vz * uzCam;
    const zCam = vx * fx + vy * fy + vz * fz;

    if (zCam < 0.25) return null;
    const scale = fov / zCam;
    return {
      sx: w * 0.5 + xCam * scale,
      sy: h * 0.52 - yCam * scale,
      depth: zCam,
      scale,
    };
  };

  // 1. Draw Ground Reference Grid (Track Surface)
  ctx.lineWidth = 1;
  ctx.strokeStyle = "rgba(30, 41, 59, 0.55)";
  for (let gx = -3.0; gx <= 3.0; gx += 0.5) {
    const p1 = projectPoint(gx, 0.0, -1.35, false);
    const p2 = projectPoint(gx, 0.0, 1.35, false);
    if (p1 && p2) {
      ctx.beginPath();
      ctx.moveTo(p1.sx, p1.sy);
      ctx.lineTo(p2.sx, p2.sy);
      ctx.stroke();
    }
  }
  for (let gz = -1.2; gz <= 1.2; gz += 0.4) {
    const p1 = projectPoint(-3.0, 0.0, gz, false);
    const p2 = projectPoint(3.0, 0.0, gz, false);
    if (p1 && p2) {
      ctx.beginPath();
      ctx.moveTo(p1.sx, p1.sy);
      ctx.lineTo(p2.sx, p2.sy);
      ctx.stroke();
    }
  }

  // Compute hot-spot values (0..100%) per sub-component based on shader mode (stress vs temp)
  const isTempMode = chassis3DState.heatMode === "temp";
  const heatVals = {
    engine_block: isTempMode
      ? tempToNormalizedPct(sample.temp_engine_c, 80, 112)
      : sample.stress_engine_pct || 35,
    turbos_emap: isTempMode
      ? tempToNormalizedPct(sample.temp_egt_c, 420, 940)
      : Math.min(100, (sample.stress_engine_pct || 35) * 0.55 + ((sample.emap_psi || 15) / 55.0) * 55.0),
    fmic_intake: isTempMode
      ? tempToNormalizedPct(sample.temp_iat_c, 18, 68)
      : tempToNormalizedPct(sample.temp_iat_c, 18, 65),
    trans_tc: isTempMode
      ? tempToNormalizedPct(sample.temp_trans_c, 75, 118)
      : Math.min(100, (sample.stress_trans_pct || 30) + ((sample.tc_slip_kw || 0) > 60 ? 18 : 0)),
    trans_case: isTempMode
      ? tempToNormalizedPct(sample.temp_trans_c, 75, 118)
      : sample.stress_trans_pct || 30,
    awd_case: isTempMode
      ? tempToNormalizedPct(sample.temp_awd_c, 55, 115)
      : sample.stress_awd_pct || 30,
    driveline_f: isTempMode
      ? tempToNormalizedPct(sample.temp_front_diff_c, 48, 95)
      : sample.stress_drive_f_pct || 25,
    driveline_r: isTempMode
      ? tempToNormalizedPct(sample.temp_rear_diff_c, 50, 105)
      : sample.stress_drive_r_pct || 35,
    tire_f: isTempMode
      ? tempToNormalizedPct(sample.temp_tire_f_c, 22, 75)
      : sample.stress_tire_f_pct || 25,
    tire_r: isTempMode
      ? tempToNormalizedPct(sample.temp_tire_r_c, 22, 85)
      : sample.stress_tire_r_pct || 35,
    brakes: Math.min(100, ((sample.brake_psi || 0) / 1200.0) * 95.0),
  };

  // Build 3D wireframe line segments for Stinger GT body shell + [1..7] driveline sub-modules
  const airflowRestr = parseFloat(document.getElementById("inpAirflowRestrPct")?.value) || 0;
  const segments = buildStingerWireframeSegments(sample, heatVals, airflowRestr);

  // Project and depth-sort segments (Painter's algorithm back-to-front)
  const projectedSegs = [];
  const focus = chassis3DState.focusModule;

  for (let i = 0; i < segments.length; i++) {
    const seg = segments[i];
    const p1 = projectPoint(seg.x1, seg.y1, seg.z1, seg.squat !== false);
    const p2 = projectPoint(seg.x2, seg.y2, seg.z2, seg.squat !== false);
    if (!p1 || !p2) continue;

    let alpha = seg.alpha ?? 0.85;
    let width = seg.width ?? 1.5;
    if (focus !== "all") {
      if (seg.module === focus) {
        alpha = Math.min(1.0, alpha * 1.25);
        width *= 1.35;
      } else if (seg.module === "chassis") {
        alpha = 0.09;
        width = 0.8;
      } else {
        alpha = 0.14;
        width = 0.9;
      }
    }

    projectedSegs.push({
      sx1: p1.sx,
      sy1: p1.sy,
      sx2: p2.sx,
      sy2: p2.sy,
      depth: (p1.depth + p2.depth) * 0.5,
      color: seg.color || stressIndexToColor(seg.heat || 0, alpha),
      width,
      glow: seg.glow && (focus === "all" || seg.module === focus),
    });
  }

  projectedSegs.sort((a, b) => b.depth - a.depth);

  // Render sorted 3D wireframe segments
  for (let i = 0; i < projectedSegs.length; i++) {
    const s = projectedSegs[i];
    ctx.beginPath();
    ctx.moveTo(s.sx1, s.sy1);
    ctx.lineTo(s.sx2, s.sy2);
    ctx.strokeStyle = s.color;
    ctx.lineWidth = s.width;
    if (s.glow) {
      ctx.shadowColor = s.color;
      ctx.shadowBlur = 7;
    } else {
      ctx.shadowBlur = 0;
    }
    ctx.stroke();
  }
  ctx.shadowBlur = 0;

  // Render Numbered [1..7] Schematic Sub-Module Callout Pins & Live Hot-Spot Readouts
  const callouts = [
    {
      mod: "engine",
      x: 1.44,
      y: 0.68,
      z: -0.18,
      title: "[1] 3.3T V6 ENGINE (G6DP)",
      sub: `Stress ${(sample.stress_engine_pct || 0).toFixed(0)}% | EGT ${(sample.temp_egt_c || 0).toFixed(0)}°C | IAT ${(sample.temp_iat_c || 0).toFixed(0)}°C`,
      heat: Math.max(heatVals.engine_block, heatVals.turbos_emap),
    },
    {
      mod: "trans",
      x: 0.68,
      y: 0.50,
      z: -0.18,
      title: "[2] 8AT TRANS (A8LR1) & TC",
      sub: `Stress ${(sample.stress_trans_pct || 0).toFixed(0)}% | ATF ${(sample.temp_trans_c || 0).toFixed(1)}°C | Slip ${(sample.tc_slip_kw || 0).toFixed(0)}kW`,
      heat: heatVals.trans_tc,
    },
    {
      mod: "awd",
      x: 0.16,
      y: 0.52,
      z: 0.34,
      title: "[5] MAGNA ATC AWD CASE",
      sub: `Stress ${(sample.stress_awd_pct || 0).toFixed(0)}% | ${(sample.temp_awd_c || 0).toFixed(1)}°C | ${Math.round(sample.awd_actual_nm || 0)}Nm`,
      heat: heatVals.awd_case,
    },
    {
      mod: "driveline_f",
      x: 0.85,
      y: 0.42,
      z: 0.40,
      title: "[6,7] FRONT PROPSHAFT & DIFF",
      sub: `Stress ${(sample.stress_drive_f_pct || 0).toFixed(0)}% | Diff ${(sample.temp_front_diff_c || 0).toFixed(1)}°C`,
      heat: heatVals.driveline_f,
    },
    {
      mod: "driveline_r",
      x: -0.62,
      y: 0.42,
      z: -0.05,
      title: "[4] REAR PROPSHAFT & [3] LSD",
      sub: `Stress ${(sample.stress_drive_r_pct || 0).toFixed(0)}% | LSD ${(sample.temp_rear_diff_c || 0).toFixed(1)}°C`,
      heat: heatVals.driveline_r,
    },
    {
      mod: "tires",
      x: -1.45,
      y: 0.68,
      z: 0.82,
      title: "REAR TIRES (255/35R19)",
      sub: `Stress ${(sample.stress_tire_r_pct || 0).toFixed(0)}% | Tread ${(sample.temp_tire_r_c || 0).toFixed(1)}°C | Slip ${(sample.slip_r_pct || 0).toFixed(1)}%`,
      heat: heatVals.tire_r,
    },
  ];

  callouts.forEach((c) => {
    if (focus !== "all" && focus !== c.mod) return;
    const pt = projectPoint(c.x, c.y, c.z, true);
    if (!pt) return;
    const col = stressIndexToColor(c.heat, 0.95);

    // Pin dot
    ctx.fillStyle = col;
    ctx.beginPath();
    ctx.arc(pt.sx, pt.sy, 3.5, 0, Math.PI * 2);
    ctx.fill();

    // Leader line & callout pill
    const lx = pt.sx + 10;
    const ly = pt.sy - 14;
    ctx.strokeStyle = col;
    ctx.lineWidth = 1.2;
    ctx.beginPath();
    ctx.moveTo(pt.sx, pt.sy);
    ctx.lineTo(lx, ly);
    ctx.stroke();

    ctx.font = "bold 9.5px Consolas, monospace";
    const w1 = ctx.measureText(c.title).width;
    ctx.font = "9px Consolas, monospace";
    const w2 = ctx.measureText(c.sub).width;
    const boxW = Math.max(w1, w2) + 10;
    const boxH = 26;

    ctx.fillStyle = "rgba(6, 10, 18, 0.86)";
    ctx.fillRect(lx, ly - boxH + 4, boxW, boxH);
    ctx.strokeStyle = col;
    ctx.lineWidth = 1;
    ctx.strokeRect(lx, ly - boxH + 4, boxW, boxH);

    ctx.fillStyle = col;
    ctx.font = "bold 9.5px Consolas, monospace";
    ctx.textAlign = "left";
    ctx.fillText(c.title, lx + 5, ly - 11);
    ctx.fillStyle = "#e2e8f0";
    ctx.font = "9px Consolas, monospace";
    ctx.fillText(c.sub, lx + 5, ly - 1);
  });

  // Render 3D Orientation Triad Gizmo in Bottom-Right Corner (+X FRONT, +Y UP, +Z RH)
  const gx = w - 68;
  const gy = h - 48;
  const axisLen = 28;
  const drawAxisVec = (ax, ay, az, color, label) => {
    const sx = gx + (ax * rxCam + ay * ryCam + az * rzCam) * axisLen;
    const sy = gy - (ax * uxCam + ay * uyCam + az * uzCam) * axisLen;
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(gx, gy);
    ctx.lineTo(sx, sy);
    ctx.stroke();
    ctx.fillStyle = color;
    ctx.font = "bold 9px Consolas, monospace";
    ctx.textAlign = "center";
    ctx.fillText(label, sx + (sx >= gx ? 8 : -8), sy + (sy >= gy ? 8 : -4));
  };
  drawAxisVec(1, 0, 0, "#00e5ff", "+X FRONT");
  drawAxisVec(0, 1, 0, "#00e676", "+Y UP");
  drawAxisVec(0, 0, 1, "#ffab00", "+Z RH");
}

// Construct the 3D Kia Stinger GT Fastback Chassis Shell + [1..7] Schematic-Matched Powertrain Sub-Modules
function buildStingerWireframeSegments(sample, heatVals, airflowRestrPct) {
  const segs = [];

  const addSeg = (x1, y1, z1, x2, y2, z2, opts) => {
    segs.push({ x1, y1, z1, x2, y2, z2, ...opts });
  };

  const addBox = (cx, cy, cz, dx, dy, dz, opts) => {
    const x0 = cx - dx / 2, x1 = cx + dx / 2;
    const y0 = cy - dy / 2, y1 = cy + dy / 2;
    const z0 = cz - dz / 2, z1 = cz + dz / 2;
    // Bottom rectangle
    addSeg(x0, y0, z0, x1, y0, z0, opts);
    addSeg(x1, y0, z0, x1, y0, z1, opts);
    addSeg(x1, y0, z1, x0, y0, z1, opts);
    addSeg(x0, y0, z1, x0, y0, z0, opts);
    // Top rectangle
    addSeg(x0, y1, z0, x1, y1, z0, opts);
    addSeg(x1, y1, z0, x1, y1, z1, opts);
    addSeg(x1, y1, z1, x0, y1, z1, opts);
    addSeg(x0, y1, z1, x0, y1, z0, opts);
    // Vertical pillars
    addSeg(x0, y0, z0, x0, y1, z0, opts);
    addSeg(x1, y0, z0, x1, y1, z0, opts);
    addSeg(x1, y0, z1, x1, y1, z1, opts);
    addSeg(x0, y0, z1, x0, y1, z1, opts);
  };

  const addCylinderX = (x0, x1, cy, cz, r, sides, opts) => {
    for (let i = 0; i < sides; i++) {
      const a0 = (i / sides) * Math.PI * 2;
      const a1 = ((i + 1) / sides) * Math.PI * 2;
      const y0 = cy + Math.cos(a0) * r, z0 = cz + Math.sin(a0) * r;
      const y1 = cy + Math.cos(a1) * r, z1 = cz + Math.sin(a1) * r;
      addSeg(x0, y0, z0, x0, y1, z1, opts);
      addSeg(x1, y0, z0, x1, y1, z1, opts);
      addSeg(x0, y0, z0, x1, y0, z0, opts);
    }
  };

  const addCylinderY = (cx, y0, y1, cz, r, sides, opts) => {
    for (let i = 0; i < sides; i++) {
      const a0 = (i / sides) * Math.PI * 2;
      const a1 = ((i + 1) / sides) * Math.PI * 2;
      const x0 = cx + Math.cos(a0) * r, z0 = cz + Math.sin(a0) * r;
      const x1 = cx + Math.cos(a1) * r, z1 = cz + Math.sin(a1) * r;
      addSeg(x0, y0, z0, x1, y0, z1, opts);
      addSeg(x0, y1, z0, x1, y1, z1, opts);
      if (i % 2 === 0) addSeg(x0, y0, z0, x0, y1, z0, opts);
    }
  };

  const addCylinderZ = (cx, cy, z0, z1, r, sides, rotAngle, opts) => {
    for (let i = 0; i < sides; i++) {
      const a0 = rotAngle + (i / sides) * Math.PI * 2;
      const a1 = rotAngle + ((i + 1) / sides) * Math.PI * 2;
      const x0 = cx + Math.cos(a0) * r, y0 = cy + Math.sin(a0) * r;
      const x1 = cx + Math.cos(a1) * r, y1 = cy + Math.sin(a1) * r;
      addSeg(x0, y0, z0, x1, y1, z0, opts);
      addSeg(x0, y0, z1, x1, y1, z1, opts);
      addSeg(x0, y0, z0, x0, y0, z1, opts);
    }
  };

  // ==========================================================================
  // 0. KIA STINGER GT (CK) FASTBACK BODY WIREFRAME SHELL ("chassis")
  // ==========================================================================
  const chassisOpts = { module: "chassis", color: "rgba(56, 189, 248, 0.22)", width: 1.1 };
  const chassisAccent = { module: "chassis", color: "rgba(56, 189, 248, 0.36)", width: 1.3 };

  const bodyStations = [
    [2.36, 0.17, 0.56, 0.58, 0.76, 0.62], // Front splitter & Tiger-Nose bumper
    [2.12, 0.17, 0.72, 0.75, 0.86, 0.72], // Headlights & hood leading edge
    [1.45, 0.18, 0.86, 0.89, 0.93, 0.80], // Front axle / strut towers
    [0.82, 0.18, 0.93, 0.96, 0.92, 0.78], // Base of windshield / cowl
    [0.15, 0.18, 0.96, 1.36, 0.91, 0.68], // Front header / A-pillar top
    [-0.55, 0.18, 0.98, 1.38, 0.91, 0.67], // B-pillar roof peak
    [-1.45, 0.19, 1.00, 1.24, 0.94, 0.64], // Rear axle / wide haunches / fastback C-pillar
    [-2.15, 0.21, 0.97, 1.02, 0.86, 0.58], // Fastback rear glass taper
    [-2.44, 0.25, 0.92, 0.95, 0.76, 0.54], // Ducktail liftback decklid & rear diffuser
  ];

  for (let i = 0; i < bodyStations.length; i++) {
    const [x, yb, ym, yr, zw, zrw] = bodyStations[i];
    addSeg(x, yb, -zw, x, yb, zw, chassisOpts);
    addSeg(x, yb, -zw, x, ym, -zw, chassisOpts);
    addSeg(x, yb, zw, x, ym, zw, chassisOpts);
    addSeg(x, ym, -zw, x, yr, -zrw, chassisOpts);
    addSeg(x, ym, zw, x, yr, zrw, chassisOpts);
    addSeg(x, yr, -zrw, x, yr, zrw, chassisAccent);

    if (i < bodyStations.length - 1) {
      const [nx, nyb, nym, nyr, nzw, nzrw] = bodyStations[i + 1];
      addSeg(x, yb, -zw, nx, nyb, -nzw, chassisOpts);
      addSeg(x, yb, zw, nx, nyb, nzw, chassisOpts);
      addSeg(x, ym, -zw, nx, nym, -nzw, chassisAccent);
      addSeg(x, ym, zw, nx, nym, nzw, chassisAccent);
      addSeg(x, yr, -zrw, nx, nyr, -nzrw, chassisAccent);
      addSeg(x, yr, zrw, nx, nyr, nzrw, chassisAccent);
      addSeg(x, yr, 0, nx, nyr, 0, chassisOpts);
    }
  }

  // Kia Stinger Signature Tiger-Nose Grille & Dual Hood Vents
  addBox(2.36, 0.42, 0.0, 0.04, 0.16, 0.82, chassisAccent);
  addBox(1.58, 0.89, -0.34, 0.34, 0.02, 0.14, chassisAccent);
  addBox(1.58, 0.89, 0.34, 0.34, 0.02, 0.14, chassisAccent);

  // Quad Oval Stinger GT Exhaust Tips at Rear Diffuser (X = -2.46 m)
  [-0.58, -0.44, 0.44, 0.58].forEach((zTip) => {
    addCylinderX(-2.50, -2.35, 0.26, zTip, 0.045, 6, {
      module: "engine",
      heat: heatVals.turbos_emap,
      width: 1.6,
    });
  });

  // ==========================================================================
  // 0b. COOLING AIRFLOW STREAMLINES (Sensitive to AirflowRestrictionPct)
  // ==========================================================================
  const afColor =
    airflowRestrPct >= 50
      ? "rgba(255, 82, 82, 0.65)"
      : airflowRestrPct >= 20
      ? "rgba(255, 171, 0, 0.65)"
      : "rgba(0, 229, 255, 0.55)";
  const phase = chassis3DState.airflowPhase || 0;
  [-0.45, -0.22, 0.0, 0.22, 0.45].forEach((zStr) => {
    const startX = 2.65 - phase * 0.45;
    addSeg(startX, 0.34, zStr, startX - 0.28, 0.35, zStr, {
      module: "engine",
      color: afColor,
      width: 1.5,
    });
    const underX = 1.6 - phase * 1.4;
    addSeg(underX, 0.16, zStr * 0.5, underX - 0.35, 0.16, zStr * 0.5, {
      module: "awd",
      color: afColor,
      width: 1.2,
    });
  });

  // ==========================================================================
  // [1] SUB-MODULE: 3.3L V6 TWIN-TURBO ENGINE (G6DP), 6 CYLINDERS, FMIC & TURBOS
  // Positioned directly over the front axle (X = +1.06 .. +1.80 m, Z = -0.24 .. +0.24 m)
  // ==========================================================================
  const engOpts = { module: "engine", heat: heatVals.engine_block, width: 1.9, glow: heatVals.engine_block > 72 };
  const cylOpts = { module: "engine", heat: Math.min(100, heatVals.engine_block + 8), width: 2.0, glow: true };
  const turboOpts = { module: "engine", heat: heatVals.turbos_emap, width: 2.0, glow: true };
  const fmicOpts = { module: "engine", heat: heatVals.fmic_intake, width: 1.75 };

  // V6 Engine Block Deck & Upper Oil Pan (Straddling Front Axle X = +1.45 m, Z = -0.24 .. +0.24 m)
  addBox(1.43, 0.42, 0.0, 0.72, 0.28, 0.48, engOpts);
  // Green/Adapter Bellhousing Flange at rear of V6 block (X = +0.98 .. +1.07 m, matching schematic [1] rear step)
  addBox(1.025, 0.41, 0.0, 0.09, 0.26, 0.44, engOpts);

  // 6 Distinct V6 Cylinder Domes on top of Block [1] (3 on RH +Z bank, 3 on LH -Z bank, matching schematic [1])
  [1.65, 1.43, 1.21].forEach((cx) => {
    addCylinderY(cx, 0.56, 0.63, 0.12, 0.095, 10, cylOpts);  // RH Bank (Cylinders 1, 3, 5)
    addCylinderY(cx, 0.56, 0.63, -0.12, 0.095, 10, cylOpts); // LH Bank (Cylinders 2, 4, 6)
  });

  // Front-Mount Intercooler (FMIC) Core at Nose (X = +2.16 m) & Twin Turbochargers
  addBox(2.16, 0.34, 0.0, 0.10, 0.22, 0.88, fmicOpts);
  addSeg(2.11, 0.34, -0.40, 1.50, 0.44, -0.32, fmicOpts);
  addSeg(2.11, 0.34, 0.40, 1.50, 0.44, 0.32, fmicOpts);
  // Twin Garrett MGT1549DSL Turbochargers tucked on lower flanks of V6 block
  addCylinderX(1.16, 1.36, 0.38, -0.32, 0.09, 8, turboOpts);
  addCylinderX(1.16, 1.36, 0.38, 0.32, 0.09, 8, turboOpts);

  // ==========================================================================
  // [2] SUB-MODULE: A8LR1 8-SPEED AUTOMATIC TRANSMISSION & TORQUE CONVERTER
  // Bolted directly behind [1] along centerline Z = 0.0 (X = +0.34 .. +0.98 m)
  // ==========================================================================
  const tcOpts = { module: "trans", heat: heatVals.trans_tc, width: 2.1, glow: heatVals.trans_tc > 68 };
  const transOpts = { module: "trans", heat: heatVals.trans_case, width: 1.9, glow: heatVals.trans_case > 72 };

  // Torque Converter Bellhousing (X = +0.78 .. +0.98 m, Z = 0.0 m)
  addCylinderX(0.78, 0.98, 0.37, 0.0, 0.175, 10, tcOpts);
  // 8AT Main Planetary Case (X = +0.34 .. +0.78 m, Z = 0.0 m, matching silver cylinder [2] in schematic)
  addCylinderX(0.34, 0.78, 0.37, 0.0, 0.155, 10, transOpts);
  addBox(0.60, 0.23, 0.0, 0.48, 0.06, 0.28, transOpts);

  // ==========================================================================
  // [5] SUB-MODULE: MAGNA ATC ACTIVE AWD TRANSFER CASE (95447-4J105)
  // Asymmetric L-shaped housing at rear of 8AT (X = -0.02 .. +0.34 m) with
  // lateral chain-drive drop-arm extending to RH (+Z = +0.36 m) & 3-bolt rear guibo flange
  // ==========================================================================
  const awdOpts = { module: "awd", heat: heatVals.awd_case, width: 2.2, glow: heatVals.awd_case > 62 };

  // Main Centerline Wet Multi-Plate Clutch Housing (X = +0.04 .. +0.34 m, Z = 0.0 m)
  addBox(0.22, 0.37, 0.0, 0.24, 0.28, 0.34, awdOpts);
  addCylinderX(0.02, 0.26, 0.37, 0.0, 0.135, 10, awdOpts);
  // Rear 3-Bolt Guibo Output Flange at X = -0.02 .. +0.02 m (matching inset CAD diagram)
  addCylinderX(-0.02, 0.02, 0.37, 0.0, 0.105, 6, awdOpts);

  // Lateral Chain-Drive Drop-Arm extending upward in Top View to RH side (Z = 0.0 .. +0.44 m, X = +0.12 .. +0.28 m)
  addBox(0.20, 0.36, 0.22, 0.16, 0.22, 0.48, awdOpts);
  // Front Output Gear Housing & Flange on RH side at Z = +0.36 m facing Forward Propshaft [6]
  addCylinderX(0.10, 0.30, 0.36, 0.36, 0.10, 8, awdOpts);
  addCylinderX(0.30, 0.34, 0.36, 0.36, 0.115, 6, awdOpts);
  // Electro-Hydraulic VCM Actuator Motor Boss on outer RH tip (Z = +0.45 m)
  addCylinderX(0.08, 0.24, 0.35, 0.45, 0.065, 8, awdOpts);

  // ==========================================================================
  // [6] & [7] SUB-MODULE: FRONT PROPSHAFT [6], FRONT DIFF [7] & THROUGH-SUMP AXLE
  // Matches schematic: [6] runs parallel to 8AT on RH (+Z = +0.36 m) into [7] Front Diff
  // beside V6 block [1]; LH front axle passes through V6 oil pan via silver jackshaft tube!
  // ==========================================================================
  const drvFOpts = { module: "driveline_f", heat: heatVals.driveline_f, width: 1.95, glow: heatVals.driveline_f > 65 };
  const flangeFOpts = { module: "driveline_f", heat: Math.min(100, heatVals.driveline_f + 12), width: 2.2 };

  // [6] Front Propeller Shaft (X = +0.34 .. +1.18 m at Z = +0.36 m, parallel to 8AT [2])
  addCylinderX(0.34, 1.18, 0.36, 0.36, 0.048, 8, drvFOpts);
  // Red/accented coupling flanges at both ends of Front Propshaft [6]
  addCylinderX(0.34, 0.39, 0.36, 0.36, 0.085, 6, flangeFOpts);
  addCylinderX(1.13, 1.18, 0.36, 0.36, 0.085, 6, flangeFOpts);

  // [7] Front Differential Carrier mounted on RH (+Z = +0.36 m) side of V6 Engine [1] at Front Axle X = +1.45 m
  addCylinderX(1.18, 1.32, 0.35, 0.36, 0.09, 8, flangeFOpts); // Input pinion neck
  addBox(1.45, 0.35, 0.36, 0.26, 0.22, 0.22, flangeFOpts);     // Main Front Diff housing [7]

  // RH (Top in Top-View) Short Front CV Halfshaft (Z = +0.47 .. +0.76 m at X = +1.45 m)
  addCylinderZ(1.45, 0.334, 0.47, 0.76, 0.032, 6, chassis3DState.wheelAngleFront, drvFOpts);

  // LH (Bottom in Top-View) Through-Sump Intermediate Jackshaft Housing emerging from bottom of V6 [1] (Z = -0.24 .. -0.48 m)
  addCylinderZ(1.45, 0.334, -0.48, -0.24, 0.058, 8, 0, drvFOpts);
  // LH Front CV Halfshaft from Intermediate Shaft to LH Front Wheel Hub (Z = -0.76 .. -0.48 m)
  addCylinderZ(1.45, 0.334, -0.76, -0.48, 0.032, 6, chassis3DState.wheelAngleFront, drvFOpts);

  // ==========================================================================
  // [4] & [3] SUB-MODULE: REAR PROPSHAFT [4], REAR LSD DIFF [3] & REAR HALFSHAFTS
  // Matches schematic: [4] runs straight down centerline Z = 0.0 from Transfer Case [5]
  // to Rear Diff [3] at X = -1.45 m, with equal-length halfshafts to rear wheels.
  // ==========================================================================
  const drvROpts = { module: "driveline_r", heat: heatVals.driveline_r, width: 2.0, glow: heatVals.driveline_r > 65 };
  const flangeROpts = { module: "driveline_r", heat: Math.min(100, heatVals.driveline_r + 12), width: 2.25 };

  // [4] Rear Propeller Shaft along centerline Z = 0.0 (X = -1.16 .. -0.02 m)
  addCylinderX(-1.16, -0.02, 0.36, 0.0, 0.058, 8, drvROpts);
  // Coupling flanges at both ends of Rear Propshaft [4]
  addCylinderX(-0.08, -0.02, 0.36, 0.0, 0.095, 6, flangeROpts);
  addCylinderX(-1.16, -1.10, 0.36, 0.0, 0.095, 6, flangeROpts);

  // [3] Rear Limited-Slip Differential (LSD) Carrier at X = -1.45 m, Z = 0.0 m
  addCylinderX(-1.30, -1.16, 0.35, 0.0, 0.095, 8, flangeROpts); // Forward pinion nose
  addBox(-1.45, 0.35, 0.0, 0.30, 0.24, 0.28, flangeROpts);      // Main Rear Diff housing [3]
  addCylinderZ(-1.45, 0.34, -0.22, 0.22, 0.085, 8, 0, flangeROpts); // Side output bosses

  // Rear Heavy-Duty Left & Right Halfshafts out to Rear Wheel Hubs (Z = -0.76 .. -0.22 & +0.22 .. +0.76 m)
  addCylinderZ(-1.45, 0.334, -0.76, -0.22, 0.036, 6, chassis3DState.wheelAngleRear, drvROpts);
  addCylinderZ(-1.45, 0.334, 0.22, 0.76, 0.036, 6, chassis3DState.wheelAngleRear, drvROpts);

  // ==========================================================================
  // 6. SUB-MODULE: FRONT & REAR TIRES, WHEELS & BREMBO BRAKES ("tires")
  // ==========================================================================
  const tireFOpts = { module: "tires", heat: heatVals.tire_f, width: 1.85, squat: false, glow: heatVals.tire_f > 68 };
  const tireROpts = { module: "tires", heat: heatVals.tire_r, width: 2.0, squat: false, glow: heatVals.tire_r > 68 };
  const brakeOpts = {
    module: "tires",
    heat: Math.max(heatVals.brakes, 25),
    width: 1.7,
    squat: false,
    glow: heatVals.brakes > 40,
  };

  // Front Left & Right Wheels (225/40R19 -> R = 0.334m, Width = 0.23m)
  addCylinderZ(1.45, 0.334, -0.92, -0.68, 0.334, 16, chassis3DState.wheelAngleFront, tireFOpts);
  addCylinderZ(1.45, 0.334, 0.68, 0.92, 0.334, 16, chassis3DState.wheelAngleFront, tireFOpts);
  // Front Brembo Brake Rotors
  addCylinderZ(1.45, 0.334, -0.74, -0.70, 0.19, 10, chassis3DState.wheelAngleFront, brakeOpts);
  addCylinderZ(1.45, 0.334, 0.70, 0.74, 0.19, 10, chassis3DState.wheelAngleFront, brakeOpts);

  // Rear Left & Right Wheels (255/35R19 -> R = 0.334m, Width = 0.26m)
  addCylinderZ(-1.45, 0.334, -0.94, -0.66, 0.334, 16, chassis3DState.wheelAngleRear, tireROpts);
  addCylinderZ(-1.45, 0.334, 0.66, 0.94, 0.334, 16, chassis3DState.wheelAngleRear, tireROpts);
  // Rear Brembo Brake Rotors
  addCylinderZ(-1.45, 0.334, -0.73, -0.69, 0.18, 10, chassis3DState.wheelAngleRear, brakeOpts);
  addCylinderZ(-1.45, 0.334, 0.69, 0.73, 0.18, 10, chassis3DState.wheelAngleRear, brakeOpts);

  return segs;
}

function renderSubmoduleInspectorCards(s) {
  const stack = document.getElementById("submoduleCardsStack");
  if (!stack || !s) return;

  const cards = [
    {
      id: "engine",
      name: "[1] 3.3L Twin-Turbo V6 (6-Cyl) & FMIC",
      stress: s.stress_engine_pct || 0,
      tempLabel: `EGT ${(s.temp_egt_c || 0).toFixed(0)}&deg;C | IAT ${(s.temp_iat_c || 0).toFixed(1)}&deg;C`,
      m1: `Torque: ${(s.eng_nm || 0).toFixed(0)} Nm @ ${Math.round(s.rpm || 0)} RPM`,
      m2: `Boost: ${(s.boost_psi || 0).toFixed(1)} psi | EMAP: ${(s.emap_psi || 0).toFixed(1)} psi | KR: ${(s.kr_deg || 0).toFixed(1)}&deg;`,
    },
    {
      id: "trans",
      name: "[2] A8LR1 8AT & Torque Converter",
      stress: s.stress_trans_pct || 0,
      tempLabel: `ATF ${(s.temp_trans_c || 0).toFixed(1)}&deg;C`,
      m1: `Gear: ${s.gear || 1} ${s.shift ? "(SHIFT INERTIA PHASE)" : "(COUPLED)"}`,
      m2: `TC/Clutch Slip Heat: ${(s.tc_slip_kw || 0).toFixed(1)} kW | Coolant: ${(s.temp_engine_c || 85).toFixed(1)}&deg;C`,
    },
    {
      id: "awd",
      name: "[5] Magna ATC Transfer Case (RH Drop)",
      stress: s.stress_awd_pct || 0,
      tempLabel: `Clutch ${(s.temp_awd_c || 0).toFixed(1)}&deg;C`,
      m1: `Coupling: ${(s.awd_actual_nm || 0).toFixed(0)} Nm (${(s.awd_split_pct || 0).toFixed(0)}% Front)`,
      m2: `Clutch Slip Diss: ${(s.awd_slip_kw || 0).toFixed(1)} kW ${s.awd_brake_gated ? "| [64.5 PSI GATED]" : ""}`,
    },
    {
      id: "driveline_f",
      name: "[6,7] Front Propshaft, Diff & Sump Axle",
      stress: s.stress_drive_f_pct || 0,
      tempLabel: `Diff ${(s.temp_front_diff_c || 0).toFixed(1)}&deg;C`,
      m1: `Front Prop Torque: ${(s.front_prop_nm || 0).toFixed(0)} Nm`,
      m2: `Front Normal Load Fz: ${(s.fz_front_n || 0).toFixed(0)} N | Fx: ${(s.fx_front_n || 0).toFixed(0)} N`,
    },
    {
      id: "driveline_r",
      name: "[3,4] Rear Propshaft, LSD Diff & Axles",
      stress: s.stress_drive_r_pct || 0,
      tempLabel: `LSD ${(s.temp_rear_diff_c || 0).toFixed(1)}&deg;C`,
      m1: `Rear Prop Torque: ${(s.rear_prop_nm || 0).toFixed(0)} Nm`,
      m2: `Rear Normal Load Fz: ${(s.fz_rear_n || 0).toFixed(0)} N | Fx: ${(s.fx_rear_n || 0).toFixed(0)} N`,
    },
    {
      id: "tires",
      name: "Tires, Wheels & Brembo Brakes",
      stress: Math.max(s.stress_tire_r_pct || 0, s.stress_tire_f_pct || 0),
      tempLabel: `R ${(s.temp_tire_r_c || 0).toFixed(1)}&deg;C | F ${(s.temp_tire_f_c || 0).toFixed(1)}&deg;C`,
      m1: `Wheel Slip: Rear ${(s.slip_r_pct || 0).toFixed(1)}% | Front ${(s.slip_f_pct || 0).toFixed(1)}%`,
      m2: `Grip mu: R ${(s.mu_rear || 0).toFixed(2)} / F ${(s.mu_front || 0).toFixed(2)} | Brake: ${(s.brake_psi || 0).toFixed(0)} psi`,
    },
  ];

  stack.innerHTML = cards
    .map((c) => {
      const col = stressIndexToColor(c.stress, 1.0);
      const isAct = chassis3DState.focusModule === c.id;
      return `
        <div class="submod-card ${isAct ? "active" : ""}" onclick="setChassisSubmoduleFocus('${c.id}')">
          <div class="submod-top">
            <span>${c.name}</span>
            <span class="submod-temp-badge" style="color:${col}; border: 1px solid ${col};">${c.tempLabel}</span>
          </div>
          <div class="submod-bar-bg">
            <div class="submod-bar-fill" style="width:${Math.min(100, c.stress).toFixed(1)}%; background:${col};"></div>
          </div>
          <div class="submod-metrics">
            <span><strong>Stress: ${c.stress.toFixed(0)}%</strong> &bull; ${c.m1}</span>
            <span>${c.m2}</span>
          </div>
        </div>
      `;
    })
    .join("");
}
