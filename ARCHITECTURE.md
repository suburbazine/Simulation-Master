# Simulation Master — Tri-Module Physical & Firmware Emulation Engine Architecture

> **Objective**: Correlate binary-level calibration tables and RAM state machines across the **Continental SIM2K-260 ECU (`640C7000`)**, **Kefico CPTSH 8AT TCU (`95441-4J571`)**, and **Magna ATC AWD ECU (`95447-4J105`)** with a closed-loop physical longitudinal vehicle dynamics and tire-road friction ($\mu$-slip) solver. Any edit made to an ECU, TCU, or AWD binary (or staged in memory via BinGo DB definitions) directly alters the simulated launch, shift, traction, and straight-line acceleration performance.

---

## 1. System Topology & Multi-Rate Execution Loop

The emulation engine runs a deterministic, fixed-timestep multi-rate solver synchronized over a **Virtual P-CAN Bus (500 kbps)**, matching the hardware task scheduling extracted during teardown:

- **1.0 ms Physical Plant & Tire-Road Solver**:
  - Integrates engine crankshaft speed ($\omega_{\text{eng}}$), torque converter turbine speed ($\omega_{\text{turb}}$), transmission output shaft speed ($\omega_{\text{out}}$), front/rear wheel speeds ($\omega_f, \omega_r$), vehicle velocity ($v_x$), distance ($x$), turbocharger speed ($N_{\text{TCHA}}$), manifold pressure ($\text{MAP}$), master-cylinder brake bleed ($P_{\text{brk}}$), and AWD wet-clutch hydraulic slew ($T_{\text{awd}}$).
- **10.0 ms SIM2K-260 ECU Control Loop (`0x8013A14C` / `0x800EB8A4` / `0x800F5200`)**:
  - Reads `flash_analysis.db` and any 4 MB SIM2K-260 `.bin` image.
  - Evaluates drive mode (`0x4A0` / `0x200CCE`), driver demand (`0x214280` / `0x214E80` $\times$ gear `0x213B70` $\times$ speed `0x215150`), TCS intervention (`0x800DB614` / `0x200B4B`), TCU intervention (`0x800EEDA4`), torque coordination (`0x80142D24`), inverse torque model (`0x258B1C..` $\to$ `MAF_SP`), boost setpoint (`0x21EC00` / `0x21F478`) & TCHA-speed limiter (`0x800F8E24` / `0x203EC6`), forward torque model (`0x255518` $\to$ `TQI_BAS`), spark efficiency (`0x2524EC` / `0x25262C`), and spark floors (`IGA_MIN` `0x237890`/`0x239AF4` & shift floor `0x207AD9`).
- **10.0 ms Kefico CPTSH 8AT TCU Control Loop (`SSC_MainShiftSchedule_10ms` / `HSS_DER`)**:
  - Reads `tcu_cptsh_95441_4J571.db` and any 768 KB CPTSH `.bin` image.
  - Evaluates brake-stall RPM limit (`0x238B68`), torque converter multiplication & TCC lockup (`HSS_DBM`), 43 14×20 shift maps (`0x204350..0x207728`), clutch/gear torque ceilings (`0x1FBC5C..`, `0x1FD198`, `0x1FD1B0`, `0x2096E4` $\to$ `ETL_TCU`), and 14 4×3 shift torque intervention maps (`0x233D3C..0x233F5E` $\to$ `TQI_TCU`).
- **20.0 ms Magna ATC AWD Control Loop & 10.0 ms ESC/TCS Loop**:
  - Reads `awd_95447_4J105.db` and any 32 KB / 48 KB AWD `.bin` image.
  - Evaluates brake pressure gate (`0xC07498` / `0xC07606` $= 64.5\text{ psi}$), positive slew rate limiter (`0xC07650` $= 120\text{ Nm}/20\text{ ms}$), standstill launch preload (`0xC04598..`), off-idle preload (`0xC04A3C` / `0xC04ABC`), feed-forward speed $\times$ pedal maps (`0xC04B3C` Comfort/Eco vs `0xC04BA0` Sport), 5×5 split matrices (`0xC0585A` $45\%$ vs `0xC05922` $40\%$), high-speed gates (`46.6 / 87.0 / 155.3 mph`), and wheel-slip PID (`0xC06978` / `0xC061FA`).

---

## 2. Virtual P-CAN Frame Bus Correlation

| CAN ID | Frame Name | Rate | Producer | Consumer(s) | Signals Modeled |
|---|---|---|---|---|---|
| `0x111` | `TCU11` | 10 ms | TCU | ECU | `TQI_TCU` (`b3`, fast torque cut % of 750 Nm), `TQI_TCU_INC` (`b0`), `TEMP_AT` (`b4`), `N_TC` turbine RPM (`b5-6`) |
| `0x112` | `TCU12` | 10 ms | TCU | ECU, AWD | `ETL_TCU` (`b0`, slow torque cap $\times 3\text{ Nm}$), `CUR_GR` (`b1[3:0]`), `VS_TCU` (`b2`), `INH_FUEL_CUT` (`b3.5`), `SPK_RTD_TCU` (`b4`) |
| `0x113` | `TCU13` | 10 ms | TCU | ECU | `N_TGT_LUP` (`b0`), `CF_Tcu_TarGr` (`b2>>4`) |
| `0x153` | `TCS11` | 10 ms | ESC | ECU | `TCS_REQ` (`b0.0`), `MSR_C_REQ` (`b0.1`), `TCS_MFRN` (`b1.7`), `TQI_TCS` (`b3` fast), `TQI_MSR` (`b4`), `TQI_SLW_TCS` (`b5` slow) |
| `0x220` | `ESP12` | 20 ms | ESC | ECU, TCU, AWD | `CYL_PRES` (`bits 26-37`, $0.1\text{ bar}$ brake pressure), `LONG_ACCEL` (`bits 13-23`, $0.01\text{ m/s}^2 - 10.23$) |
| `0x260` | `EMS16` | 10 ms | ECU | TCU, ESC, AWD | `TQI_MIN` (`b0`), `TQI` (`b1`), `TQI_TARGET` (`b2`), `IGA` spark advance (`b6` $\times 0.375 - 35.625^\circ$) |
| `0x316` | `EMS11` | 10 ms | ECU | TCU, ESC, AWD | `N` engine RPM (`b2-3` LE $\times 0.25$), `TQI` (`b4` $\times 0.390625\%$) |
| `0x329` | `EMS12` | 10 ms | ECU | TCU, AWD | `TCO` coolant (`b1`), `BRAKE_ACT` (`b4[1:0]`), `TPS` (`b5`), `PV_AV_CAN` pedal (`b6` $\times 0.3906\%$) |
| `0x386` | `WHL_SPD11` | 20 ms | ESC | ECU, TCU, AWD | Wheel speeds `WHL_SPD_FL`, `FR`, `RL`, `RR` (`km/h`) |
| `0x428` | `_4WD11` | 20 ms | AWD | ECU, TCU, Cluster | `DUTY` (`b2` %), `R_TIRE` (`b3` $= 0x78 = 320\text{ mm}$), `TQ_AWD_CMD` coupling torque (`b6-7` LE Nm) |
| `0x492` | `EMS19` | 20 ms | ECU | Logger | `CR_Ems_BstPre` upstream-throttle pressure `PUT` (`bits 20-31` $\times 0.1322\text{ kPa}$) |
| `0x4A0` | `DMS` | 20 ms | DMS | ECU, TCU, AWD | `b0 & 3` Drive Mode (`0`=Comfort, `1`=Sport/Smart/Custom, `2`=Eco, `3`=Mode 4) |
| `0x556` / `0x557` | `EngFrzFrm` | 100 ms | ECU | Logger | `OBD_LOAD` (`0x556 b0` $\times 5\text{ mg/stk}$), `MAP` (`0x557 b4` kPa) |
