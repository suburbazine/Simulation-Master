# Simulation Master — Tri-ECU Powertrain & Longitudinal Physics Emulation Engine

**Simulation Master** is a closed-loop longitudinal vehicle physics and multi-controller firmware emulation studio for the **Kia Stinger GT / Genesis G70 3.3L Twin-Turbo V6 (`G6DP`) + 8-Speed Automatic (`A8LR1`) + Active AWD (`Magna ATC`)** platform.

It correlates binary calibration maps and firmware control gates across three powertrain controllers against straight-line launch, shift, acceleration, and speed performance:

1. **Continental SIM2K-260 Engine ECU (`640C7000`, Infineon TriCore TC1791)**
2. **Hyundai Kefico CPTSH 8-Speed Automatic TCU (`95441-4J571`, Renesas SH72549R)**
3. **Magna Powertrain Active Transfer Case AWD ECU (`95447-4J105`, Infineon C166 /XC22xx)**

---

## Key Capabilities

- **Multi-Rate Closed-Loop Physics & Firmware Solver (`1 ms` / `10 ms` / `20 ms`)**:
  - **1 ms Longitudinal Dynamics**: Dynamic longitudinal weight transfer ($F_{z,f}, F_{z,r}$), Pacejka Magic Formula $\mu(\kappa)$ tire friction with carcass relaxation, aerodynamic drag, rolling resistance, and independent front/rear wheel angular acceleration.
  - **10 ms SIM2K-260 ECU & Kefico CPTSH TCU Loop**: Air-path boost/load dynamics (`0x23D810` `M_AIR_CYL_MAX`, `0x21DA7C` `PUT_SP`, `0x21EB38` `IP_PQ_CHA_MAX`), forward indicated torque model (`0x255518` `TQI_BAS` & `0x253D14` `TQI_MAX`), Continental spark efficiency curve (`0x25262C` / `0x2524EC`) clamped by shift spark floor (`0x207AD9`), TCHA turbo overspeed limiter (`0x203EC6` $\to$ `0x203ED4` / `0x203ED8`), torque converter hydrodynamic multiplication ($TR(\nu), K(\nu)$), and 8AT shift schedule / inertia-phase torque cuts (`0x233D3C`, `0x20FC52`).
  - **20 ms Magna ATC AWD & ESC Loop**: Pre-stage brake-pressure inhibit gate (`0xC07498` / `0xC07606` `64.5 psi` / `4.45 bar` gate vs. `0xC06080` bypass), coupling positive slew rate (`0xC07650`: `6,000` vs. `9,000 Nm/s`), speed-based feed-forward (`0xC04BA0`), dynamic front split (`0xC05922` / `0xC058F0`), and staggered tire rolling-radius slip comparator (`0xC04214` / `0xC061FA`).
- **Browser File Picker & Organized Local `storage/` Folder Tree**:
  - Built-in **`📂 IMPORT DB / BIN`** browser file picker (and drag-and-drop uploader) imports `.db` (BinGo SQLite teardown databases) and `.bin` (ECU/TCU/AWD ROM binaries) directly into `./storage/db/{ecu,tcu,awd}` and `./storage/bins/{ecu,tcu,awd}`.
  - All `.db` and `.bin` files are excluded from Git via `.gitignore`.
- **Interactive A/B Web Studio & 3D Chassis Stress/Thermal Simulator**:
  - Side-by-side Run A vs. Run B drag-strip timeslips (`60-ft`, `0–60 mph`, `0–100 km/h`, `1/8 mile`, `1/4 mile`, `100–200 km/h`, `+50 ms` launch AWD torque, and peak WHP).
  - Interactive 3D Kia Stinger GT AWD driveline schematic (`[1..7]` sub-modules) with live mechanical stress & temperature shaders and built-in HD (`1280×580` @ 12.5 FPS) GIF89a run recorder.
  - Multi-pane synchronized oscilloscopes with per-chart trace legends and live hover readouts.
  - 2D SQLite calibration map browser & binary hex diff heatmap inspector.
  - 9-ID 500 kbps C-CAN (`candump`) log exporter (`0x080`, `0x153`, `0x162`, `0x164`, `0x220`, `0x316`, `0x329`, `0x386`, `0x4A0`).

---

## Architecture & Engineering Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) — System architecture & ROM offset correlation reference
- [MODULE_COMMUNICATION_ARCHITECTURE.md](MODULE_COMMUNICATION_ARCHITECTURE.md) — 500 kbps C-CAN bus topology, multi-rate task timing, and bit-level frame matrix (`0x080`..`0x4A0`)
- [POWERTRAIN_33T_8AT_AWD_MODEL.md](POWERTRAIN_33T_8AT_AWD_MODEL.md) — Physical thermodynamic, hydrodynamic, and electro-hydraulic equations for the `G6DP` 3.3T V6, `A8LR1` 8AT, and `Magna ATC` AWD transfer case
- [storage/README.md](storage/README.md) — Local `storage/` directory hierarchy for `.db` and `.bin` files

---

## Quick Start

```powershell
# Run tests
go test -v ./...

# Build and start the Web Studio on http://localhost:8085
go build -o simulation_master.exe .
.\simulation_master.exe -port 8085
```

---

## License

Copyright (C) 2026 suburbazine / Xtremission LLC. All rights reserved.

**Simulation Master** is dual-licensed:
1. **Non-Commercial Use (GPLv3)**: Licensed under the **GNU General Public License v3.0 (GPLv3)** strictly for personal, educational, academic, and non-commercial research use.
2. **Commercial Use Prohibited Without Separate License**: Any commercial use—including selling the software or derivative works, using the software to provide paid ECU/TCU/AWD tuning, calibration, or simulation services, bundling it with commercial hardware/software, or operating it as a commercial hosted service—is **strictly prohibited** without a separate written Commercial License from the copyright holder.

See [LICENSE](LICENSE) for full terms and conditions.

