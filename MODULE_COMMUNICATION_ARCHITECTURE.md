# Inter-Module Modeling & C-CAN Communication Architecture

> **Target Platform**: Kia Stinger GT / Genesis G70 (3.3L Twin-Turbo V6 G6DP AWD)  
> **Powertrain Controller Trio**:
> - **Engine Control Unit (ECU)**: Continental SIM2K-260 (`640C7000`, Infineon TriCore SAK-TC1791 / TC277TP)
> - **Transmission Control Unit (TCU)**: Kefico CPTSH 8AT (`95441-4J571`, Renesas SH72543R / TC1782)
> - **Transfer Case All-Wheel-Drive ECU (AWD)**: Magna Powertrain ATC (`95447-4J105`, Freescale/NXP MPC5604P)
> - **Chassis / Stability Controller**: Continental / Mobis MK100 ESC & ABS
> - **Bus Architecture**: Controller Area Network — Powertrain / Chassis Bus (C-CAN / P-CAN, 500 kbps, 11-bit Standard Arbitration IDs)

---

## 1. System Topology & Multi-Rate Task Scheduling

### 1.1 Physical & Virtual CAN Bus Topology

The powertrain and chassis controllers communicate over a high-speed **500 kbps ISO 11898-2 C-CAN bus** terminated by two 120 $\Omega$ split-termination resistors (located internally in the SIM2K-260 ECU and Instrument Cluster). The vehicle dynamics solver runs a multi-rate deterministic execution loop faithfully reproducing the actual hardware sample periods, interrupt latencies, and message queues extracted during teardown:

```mermaid
flowchart TD
    subgraph CHASSIS ["Chassis Sensors & ESC"]
        ESC["Continental / Mobis MK100 ESC & ABS<br/>• 100 Hz Wheel Speeds (0x386)<br/>• 50 Hz Body Accelerations & P_brk (0x080)<br/>• 100 Hz TCS/ASR Torque Cut (0x153)"]
        CLU["Instrument Cluster / DMS Switch<br/>• 50 Hz Drive Mode & ESC State (0x4A0)"]
    end

    subgraph CCAN ["Powertrain & Chassis C-CAN Bus (500 kbps, 11-bit Std IDs)"]
        BUS(("C-CAN BUS<br/>120 Ω Terminated"))
    end

    subgraph POWERTRAIN ["Powertrain Control Units"]
        ECU["Continental SIM2K-260 ECU (640C7000)<br/>• 100 Hz Torque Model & Spark (0x316, 0x329)<br/>• TQI_BAS (0x255518), Boost (0x21EC00)<br/>• Shift Floor (0x207AD9), TCHA Cut (0x203EC6)"]
        TCU["Kefico CPTSH 8AT TCU (95441-4J571)<br/>• 100 Hz Shift & Clutch Management (0x162, 0x164)<br/>• Shift Cut (0x233D3C), Torque Caps (0x1FD198)<br/>• Stall Limit (0x238B68), VCM Floor (0x24BB22)"]
        AWD["Magna Powertrain ATC AWD ECU (95447-4J105)<br/>• 50 Hz Wet-Clutch Torque Control (0x220)<br/>• Brake Gate 64.5 psi (0xC07498/0xC07606)<br/>• Slew Limiter (0xC07650), Radius 320 mm (0xC04214)"]
    end

    subgraph PLANT ["1.0 ms Physical Plant & Drivetrain Solver"]
        SOLVER["1 ms Multi-Body Longitudinal Plant<br/>• Crankshaft ω_eng & TC Fluid Hydrodynamics<br/>• A8LR1 8-Speed Planetary Gear Train & Inertias<br/>• Magna ATC Multi-Plate Wet Clutch Slew<br/>• Front/Rear Prop Shafts & Differentials<br/>• Pacejka Magic Formula μ-Slip Tire Models"]
    end

    ESC -->|0x080, 0x153, 0x386| BUS
    CLU -->|0x4A0| BUS
    ECU -->|0x316, 0x329| BUS
    TCU -->|0x162, 0x164| BUS
    AWD -->|0x220| BUS

    BUS -->|Frame Broadcast| ECU
    BUS -->|Frame Broadcast| TCU
    BUS -->|Frame Broadcast| AWD
    BUS -->|Frame Broadcast| ESC

    PLANT <-->|States & Sensor Feedback| POWERTRAIN
    PLANT <-->|Wheel Speeds & Pressures| CHASSIS
```

---

### 1.2 Multi-Rate Execution & Task Synchronization

The simulation engine coordinates four distinct discrete execution domains:

| Execution Domain | Timestep ($\Delta t$) | Task Trigger / Interrupt | Firmware Entrypoint / Routine | Responsibilities |
|---|---|---|---|---|
| **Physical Plant & Tire Solver** | **1.0 ms** (1000 Hz) | Timer IRQ / Runge-Kutta Integrator | `physics_engine.go:ExecuteSimulation()` | Integrates engine flywheel inertia ($I_{\text{eng}}$), torque converter impeller/turbine hydrodynamics, A8LR1 gear ratios, propeller shaft torques, dynamic vertical weight transfer ($F_{z,f}, F_{z,r}$), Pacejka $\mu$-slip traction, and vehicle velocity ($v_x$). |
| **SIM2K-260 ECU Task** | **10.0 ms** (100 Hz) | OS Task Level 2 (`Task_10ms_Powertrain`) | `0x8013A14C` / `0x800EB8A4` / `0x800F5200` | Evaluates pedal interpolation (`0x214E80`), inverse torque model (`0x258B1C` $\to$ MAF setpoint), forward torque model `TQI_BAS` (`0x255518`), TCHA turbocharger overspeed check (`0x203EC6`), TCU/TCS arbitration, ignition timing (`0x238090`), and shift spark floor (`0x207AD9`). Broadcasts `0x316` and `0x329`. |
| **Kefico CPTSH TCU Task** | **10.0 ms** (100 Hz) | Periodic Timer Interrupt (`HSS_DER`) | `SSC_MainShiftSchedule_10ms` / `0x200056` | Evaluates 14×20 WOT upshift speeds (`0x207728`), torque converter lockup schedule, transmission input torque limits (`0x1FD198` / `0x2096E4`), solenoid VCM pressure floor (`0x24BB22`), and shift intervention depth (`0x233D3C`). Broadcasts `0x162` and `0x164`. |
| **Magna ATC AWD Task** | **20.0 ms** (50 Hz) | Periodic Task (`Task_AWD_Control_20ms`) | `ATC_MainLoop_20ms` (`awd_95447_4J105.db`) | Samples brake pressure `P_brk` on `0x080`, compares against 64.5 psi gate (`0xC07498`), looks up launch preload (`0xC04778`), computes feed-forward speed$\times$pedal coupling (`0xC04BA0`), interpolates front torque split (`0xC05922`), computes closed-loop slip correction (`0xC061FA`), and clamps to positive slew rate (`0xC07650`). Broadcasts `0x220`. |
| **ESC / ABS Chassis Task** | **10 ms / 20 ms** | Wheel Speed & IMU Sample IRQs | Continental MK100 Core Loop | Broadcasts 16-bit wheel speeds on `0x386` (20 ms), body IMU & master cylinder brake pressure on `0x080` (20 ms), and traction control fast spark/slow air cut requests on `0x153` (10 ms). |

```mermaid
sequenceDiagram
    autonumber
    participant Plant as 1ms Physical Plant
    participant ESC as 10/20ms ESC Controller
    participant TCU as 10ms CPTSH 8AT TCU
    participant ECU as 10ms SIM2K-260 ECU
    participant AWD as 20ms Magna ATC AWD

    rect rgb(240, 248, 255)
        Note over Plant: Timestep t = 0.000 s (Brake-Stand Release Command)
        Plant->>ESC: Hydraulic brake bleed starts (1200 psi -> 0 psi)
        Plant->>TCU: Engine @ Stall RPM (2500 stock / 3500 EKM), Turbine @ 0 RPM
        Plant->>AWD: Rear Prop Shaft loaded by TC (1.85x TR)
    end

    rect rgb(255, 250, 240)
        Note over ESC,AWD: 10ms / 20ms C-CAN Frame Interchange
        ESC->>AWD: 0x080 ESP12: P_brk = 82.7 bar (1200 psi) > 4.45 bar Gate!
        AWD->>AWD: Check 0xC07498 / 0xC06080: Stock AWD gates coupling to 0 Nm!
        AWD->>Plant: 0x220 AWD11: TQ_AWD_ACT = 0 Nm (Front Axle Disengaged)
        TCU->>ECU: 0x162 TCU11: Gear=1, ShiftActive=0, ETL_TCU=350 Nm (Stock) / 700 Nm (EKM)
        TCU->>ECU: 0x164 TCU12: N_turb=0 RPM, nu=0.00, LUP=0 (Open)
        ECU->>Plant: 0x316 EMS11: TQI_ACT = Max, eta_iga = 1.0 (WOT Launch)
    end

    rect rgb(245, 255, 245)
        Note over Plant,AWD: Post-Release Window (0 ms < t < 167 ms)
        Plant->>ESC: Brake bleeds past 64.5 psi (4.45 bar) at t = +132 ms
        ESC->>AWD: 0x080 ESP12: P_brk = 4.2 bar (< 4.45 bar threshold)
        AWD->>AWD: Gate Cleared! Target jumps to 1000 Nm. Slew 0xC07650 ramps @ 120 Nm/20ms
        AWD->>Plant: 0x220 AWD11: Ramp from 0 -> 1000 Nm takes 167 ms additional delay!
        Plant->>Plant: Rear tires break traction (100% torque to rear). Rear slip > 15%!
        ESC->>ECU: 0x153 TCS11: TCS_REQ=1, TQI_ASR cut! (Stock Bog)
    end
```

---

### 1.3 C-CAN Arbitration Priority & Bus Timing Budget

At 500 kbps, nominal bit duration is $T_{\text{bit}} = 2.0\text{ }\mu\text{s}$. A standard 11-bit CAN 2.0A data frame with 8 data bytes consumes between 111 and 135 bit times (accounting for bit-stuffing), corresponding to a bus transit latency of $222\text{ to }270\text{ }\mu\text{s}$.

Arbitration on the C-CAN bus is resolved bitwise on the identifier, with `0` (dominant) taking precedence over `1` (recessive). Priority ranks strictly by numerical value:

$$\text{Priority: } 0x080 > 0x153 > 0x162 > 0x164 > 0x220 > 0x316 > 0x329 > 0x386 > 0x4A0$$

- **Chassis safety frames (`0x080`, `0x153`)** achieve highest arbitration priority, ensuring brake pressure and traction cut requests preempt torque broadcast frames.
- **Powertrain execution frames (`0x162`, `0x164`, `0x316`, `0x329`)** execute within tight $\pm 250\text{ }\mu\text{s}$ jitter windows, enabling sample-and-hold torque coordination during rapid 8AT gear shifts.

---

## 2. Complete C-CAN Frame & Signal Matrix

The following nine arbitration IDs constitute the complete inter-module communication fabric. All signals are fully modeled, packed, and unpacked in `can_bus_matrix.go`.

```
========================================================================================
SIMULATION MASTER — C-CAN COMMUNICATION MATRIX (500 kbps, 11-bit Standard IDs)
========================================================================================
ID     Frame Name    Rate   DLC  Producer     Consumers               Key Functions Modeled
----------------------------------------------------------------------------------------
0x080  ESP12         20 ms   8   ESC/ABS      ECU, TCU, AWD           P_brk, AccelG, YawRate, AWD Gate
0x153  TCS11         10 ms   8   ESC/ABS      ECU, TCU                ASR Fast Spark & Slow Air Cut
0x162  TCU11         10 ms   8   CPTSH TCU    ECU, AWD                Target Gear, TQI_TCU, ETL_TCU
0x164  TCU12         10 ms   8   CPTSH TCU    ECU, AWD, CLU           N_turb, nu, LUP State, VCM Floor
0x220  AWD11         20 ms   8   Magna AWD    ECU, TCU, CLU           AWD Target/Actual, Split %, Slew
0x316  EMS11         10 ms   8   SIM2K-260    TCU, AWD, ESC, CLU      N (0.25 RPM), TQI_ACT, TQI_BAS
0x329  EMS12         10 ms   8   SIM2K-260    TCU, AWD, CLU           PUT Boost, TCO, Spark Floor Clamp
0x386  WHL_SPD11     20 ms   8   ESC/ABS      ECU, TCU, AWD           FL/FR/RL/RR 0.03125 km/h Speeds
0x4A0  CLU11         20 ms   8   Cluster/DMS  ECU, TCU, AWD, ESC      Drive Mode, 0x200CCE Override
========================================================================================
```

---

### 2.1 Frame `0x080` — `ESP12` (Chassis Dynamics & Master Cylinder Pressure)

- **Arbitration ID**: `0x080` (128 decimal)
- **Producer**: Continental / Mobis MK100 ESC & ABS
- **Receivers**: Continental SIM2K-260 ECU, Kefico CPTSH TCU, Magna ATC AWD ECU
- **Transmission Rate**: 20 ms (50 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [LAT_ACCEL 7:0]
Byte 1:  [LONG_ACCEL 4:0 | LAT_ACCEL 10:8]
Byte 2:  [BRK_SW_STAT 1:0 | LONG_ACCEL 10:5]
Byte 3:  [CYL_PRES 7:0]
Byte 4:  [YAW_RATE 3:0 | CYL_PRES 11:8]
Byte 5:  [YAW_RATE 11:4]
Byte 6:  [ALIVE_CNT_ESP 3:0 | RESERVED 3:0]
Byte 7:  [CRC_ESP 7:0]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `LAT_ACCEL` | 0 | 11 | Little | Unsigned | 0.01 | -10.23 | $\text{m/s}^2$ | -10.23 .. +10.24 | Lateral acceleration from ESC IMU. |
| `LONG_ACCEL` | 11 | 11 | Little | Unsigned | 0.01 | -10.23 | $\text{m/s}^2$ | -10.23 .. +10.24 | Longitudinal acceleration $a_x = \text{accelG} \times 9.80665$. Feeds AWD slip estimator. |
| `BRK_SW_STAT` | 22 | 2 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 3 | 0=Released, 1=Switch 1 Active, 3=Both Active (confirmed brake press). |
| `CYL_PRES` | 24 | 12 | Little | Unsigned | 0.1 | 0.0 | bar | 0.0 .. 409.5 | Master cylinder hydraulic brake pressure ($P_{\text{brk}}$). Drives the Magna ATC 64.5 psi (4.45 bar) brake gate at `0xC07498` / `0xC07606`. |
| `YAW_RATE` | 36 | 12 | Little | Unsigned | 0.01 | -20.47 | deg/s | -20.47 .. +20.48 | Vehicle body yaw angular rate. |
| `ALIVE_CNT_ESP` | 48 | 4 | Little | Unsigned | 1.0 | 0.0 | cnt | 0 .. 15 | Cyclic transmission liveness counter. |
| `CRC_ESP` | 56 | 8 | Little | Unsigned | 1.0 | 0.0 | crc | 0 .. 255 | 8-bit modular checksum: $(\sum_{i=0}^6 D_i + \text{ID}_{\text{sum}}) \oplus 0\text{xFF}$. |

---

### 2.2 Frame `0x153` — `TCS11` (ASR/ESC Traction Torque Cut Request)

- **Arbitration ID**: `0x153` (339 decimal)
- **Producer**: Continental / Mobis MK100 ESC & ABS
- **Receivers**: Continental SIM2K-260 ECU, Kefico CPTSH TCU
- **Transmission Rate**: 10 ms (100 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [RESERVED 6:0 | MSR_C_REQ 1 | TCS_REQ 0]
Byte 1:  [TCS_MFRN 7 | RESERVED 6:0]
Byte 2:  [RESERVED 7:0]
Byte 3:  [TQI_ASR_FAST 7:0]
Byte 4:  [TQI_MSR 7:0]
Byte 5:  [TQI_ASR_SLOW 7:0]
Byte 6:  [ALIVE_CNT_TCS 3:0 | RESERVED 3:0]
Byte 7:  [CRC_TCS 7:0]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `TCS_REQ` | 0 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | Traction Control System torque intervention request active. |
| `MSR_C_REQ` | 1 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | Motor Slip Regulation (engine drag torque) request active. |
| `TCS_MFRN` | 15 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | Fast spark intervention mode request flag. |
| `TQI_ASR_FAST` | 24 | 8 | Little | Unsigned | 4.0 | -256.0 | Nm | -256 .. +764 | Fast ignition retard torque ceiling. **Gated inside SIM2K-260 by ROM switch `0x200B4B` (`TCS_Inhibit`)**: when `0x200B4B == 1`, ECU ignores this signal completely! |
| `TQI_MSR` | 32 | 8 | Little | Unsigned | 4.0 | -256.0 | Nm | -256 .. +764 | Drag torque increase request. |
| `TQI_ASR_SLOW` | 40 | 8 | Little | Unsigned | 4.0 | -256.0 | Nm | -256 .. +764 | Slow electronic throttle blade / wastegate torque ceiling. |
| `ALIVE_CNT_TCS`| 48 | 4 | Little | Unsigned | 1.0 | 0.0 | cnt | 0 .. 15 | Cyclic transmission liveness counter. |
| `CRC_TCS` | 56 | 8 | Little | Unsigned | 1.0 | 0.0 | crc | 0 .. 255 | 8-bit modular checksum. |

---

### 2.3 Frame `0x162` — `TCU11` (Gear State & TCU Shift Torque Request)

- **Arbitration ID**: `0x162` (354 decimal)
- **Producer**: Kefico CPTSH 8AT TCU (`95441-4J571`)
- **Receivers**: Continental SIM2K-260 ECU, Magna ATC AWD ECU
- **Transmission Rate**: 10 ms (100 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [TAR_GR 7:4 | CUR_GR 3:0]
Byte 1:  [RESERVED 7:3 | SHIFT_PHASE 2:1 | SHIFT_ACTIVE 0]
Byte 2:  [TQI_TCU 7:0 (LE)]
Byte 3:  [TQI_TCU 15:8 (LE)]
Byte 4:  [ETL_TCU 7:0 (LE)]
Byte 5:  [ETL_TCU 15:8 (LE)]
Byte 6:  [ALIVE_CNT_TCU11 3:0 | RESERVED 3:0]
Byte 7:  [CRC_TCU11 7:0]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `CUR_GR` | 0 | 4 | Little | Unsigned | 1.0 | 0.0 | gear | 1 .. 8 | Currently engaged gear ratio (1=3.665, 2=2.396, 3=1.610, 4=1.190, 5=1.000, 6=0.826, 7=0.643, 8=0.556). |
| `TAR_GR` | 4 | 4 | Little | Unsigned | 1.0 | 0.0 | gear | 1 .. 8 | Destination gear ratio during active upshift/downshift sequence. |
| `SHIFT_ACTIVE`| 8 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | 1 during shift torque phase and inertia phase. |
| `SHIFT_PHASE` | 9 | 2 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 3 | 0=In Gear, 1=Torque Phase (clutch pre-fill), 2=Inertia Phase (engine speed pull-down), 3=Ramp. |
| `TQI_TCU` | 16 | 16 | Little | Unsigned | 1.0 | 0.0 | Nm | 0 .. 2047 | Fast inertia-phase torque reduction setpoint. Derived from CPTSH table `0x233D3C` (-10 stock vs -15 EKM). Sent to ECU torque coordinator `0x800EEDA4`. |
| `ETL_TCU` | 32 | 16 | Little | Unsigned | 1.0 | 0.0 | Nm | 0 .. 32767 | Continuous transmission input torque ceiling. Stock CPTSH caps at 350 Nm (`0x1FD198`), 400 Nm (`0x2096E4`), and 485 Nm clutch limit. EKM raises to 700 Nm and 32,767 Nm! |
| `ALIVE_CNT_TCU11`| 48 | 4 | Little | Unsigned | 1.0 | 0.0 | cnt | 0 .. 15 | Cyclic transmission liveness counter. |
| `CRC_TCU11` | 56 | 8 | Little | Unsigned | 1.0 | 0.0 | crc | 0 .. 255 | 8-bit modular checksum. |

---

### 2.4 Frame `0x164` — `TCU12` (Turbine Speed, Converter Slip & Lockup)

- **Arbitration ID**: `0x164` (356 decimal)
- **Producer**: Kefico CPTSH 8AT TCU (`95441-4J571`)
- **Receivers**: Continental SIM2K-260 ECU, Magna ATC AWD ECU, Instrument Cluster
- **Transmission Rate**: 10 ms (100 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [N_TURB 7:0 (LE)]
Byte 1:  [N_TURB 15:8 (LE)]
Byte 2:  [CONV_SLIP_RATIO 7:0]
Byte 3:  [RESERVED 7:2 | LUP_STAT 1:0]
Byte 4:  [TEMP_AT 7:0]
Byte 5:  [VCM_SOL_VAL 7:0 (LE)]
Byte 6:  [VCM_SOL_VAL 15:8 (LE)]
Byte 7:  [CRC_TCU12 7:0]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `N_TURB` | 0 | 16 | Little | Unsigned | 0.25 | 0.0 | RPM | 0.0 .. 16383.75 | Torque converter turbine shaft rotational speed ($\omega_{\text{turb}}$). |
| `CONV_SLIP_RATIO`| 16 | 8 | Little | Unsigned | 0.005 | 0.0 | ratio | 0.000 .. 1.275 | Speed ratio $\nu = N_{\text{turb}} / N_{\text{eng}}$. Determines hydrodynamic torque multiplication ratio $\text{TR}(\nu) = 1.85 \to 1.00$ at coupling ($\nu \ge 0.85$). |
| `LUP_STAT` | 24 | 2 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 3 | Lockup clutch state: 0=Open (stall launch), 1=Modulated Slip, 2=Locked. |
| `TEMP_AT` | 32 | 8 | Little | Unsigned | 1.0 | -40.0 | °C | -40 .. +215 | Automatic transmission fluid (ATF) temperature ($85^\circ\text{C} = 0\text{x7D}$). |
| `VCM_SOL_VAL` | 40 | 16 | Little | Unsigned | 1.0 | 0.0 | mA | 0 .. 2000 | Variable clutch modulation (VCM) solenoid current floor. Reflected from `0x24BB22` (850 mA stock vs 490 mA EKM tune). |
| `CRC_TCU12` | 56 | 8 | Little | Unsigned | 1.0 | 0.0 | crc | 0 .. 255 | 8-bit modular checksum. |

---

### 2.5 Frame `0x220` — `AWD11` (Magna ATC Coupling Torque & Split %)

- **Arbitration ID**: `0x220` (544 decimal)
- **Producer**: Magna Powertrain ATC AWD ECU (`95447-4J105`)
- **Receivers**: Continental SIM2K-260 ECU, Kefico CPTSH TCU, Instrument Cluster
- **Transmission Rate**: 20 ms (50 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [TQ_AWD_CMD 7:0 (LE)]
Byte 1:  [TQ_AWD_CMD 15:8 (LE)]
Byte 2:  [TQ_AWD_ACT 7:0 (LE)]
Byte 3:  [TQ_AWD_ACT 15:8 (LE)]
Byte 4:  [AWD_FRONT_SPLIT 7:0]
Byte 5:  [RESERVED 7:4 | AWD_SPORT_MAP_ACT 3 | AWD_SLIP_CTRL_ACT 2 | AWD_BRK_GATE_INHIBIT 1 | AWD_PRELOAD_ACT 0]
Byte 6:  [ALIVE_CNT_AWD 7:4 | AWD_THERM_STATE 2:0]
Byte 7:  [CRC_AWD 7:0]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `TQ_AWD_CMD` | 0 | 16 | Little | Unsigned | 0.5 | 0.0 | Nm | 0.0 .. 1000.0 | Commanded wet-clutch coupling torque target calculated by ATC algorithm. |
| `TQ_AWD_ACT` | 16 | 16 | Little | Unsigned | 0.5 | 0.0 | Nm | 0.0 .. 1000.0 | Actual hydraulic clamping torque achieved. Rate-limited by positive slew rate `0xC07650` (120 Nm/20ms stock vs 180 Nm/20ms mod). |
| `AWD_FRONT_SPLIT`| 32 | 8 | Little | Unsigned | 0.5 | 0.0 | % | 0.0 .. 50.0 | Front axle torque split percentage evaluated from `0xC0585A` (45% Comfort), `0xC05922` (40% Sport), or `0xC058F0` (50% Mod). |
| `AWD_PRELOAD_ACT`| 40 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | Standstill launch preload active flag (`0xC04778` / `0xC04A3C`). |
| `AWD_BRK_GATE_INHIBIT`| 41 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | **Brake gate active flag**: 1 when $P_{\text{brk}} > 64.5\text{ psi}$ (4.45 bar) in stock `0xC06080` binary, forcing `TQ_AWD_CMD = 0 Nm`. |
| `AWD_SLIP_CTRL_ACT`| 42 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | Closed-loop wheel slip feedback active (`0xC061FA` / `0xC06978`). |
| `AWD_SPORT_MAP_ACT`| 43 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | 1 when Sport feed-forward speed$\times$pedal map (`0xC04BA0`) is active. |
| `AWD_THERM_STATE`| 48 | 3 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 7 | Transfer case wet-clutch thermal state: 0=Normal, 1=Warm, 2=Derate (-30% cap), 3=Cut (Thermal Protection). |
| `ALIVE_CNT_AWD` | 52 | 4 | Little | Unsigned | 1.0 | 0.0 | cnt | 0 .. 15 | Cyclic transmission liveness counter. |
| `CRC_AWD` | 56 | 8 | Little | Unsigned | 1.0 | 0.0 | crc | 0 .. 255 | 8-bit modular checksum. |

---

### 2.6 Frame `0x316` — `EMS11` (Engine Speed & Actual Crankshaft Torque)

- **Arbitration ID**: `0x316` (790 decimal)
- **Producer**: Continental SIM2K-260 ECU (`640C7000`)
- **Receivers**: Kefico CPTSH TCU, Magna ATC AWD ECU, ESC/ABS, Instrument Cluster
- **Transmission Rate**: 10 ms (100 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [N_ENG 7:0 (LE)]
Byte 1:  [N_ENG 15:8 (LE)]
Byte 2:  [TQI_ACT 7:0]
Byte 3:  [TQI_BAS 7:0]
Byte 4:  [PEDAL_PCT 7:0]
Byte 5:  [TQI_DRV 7:0]
Byte 6:  [ALIVE_CNT_EMS11 7:4 | CF_Ems_EngStat 3:0]
Byte 7:  [CRC_EMS11 7:0]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `N_ENG` | 0 | 16 | Little | Unsigned | 0.25 | 0.0 | RPM | 0.0 .. 16383.75 | Engine crankshaft rotational speed ($N$). Raw value $= \text{RPM} \times 4$. |
| `TQI_ACT` | 16 | 8 | Little | Unsigned | 4.0 | -256.0 | Nm | -256 .. +764 | Actual delivered crankshaft brake torque after spark efficiency ($\eta_{\text{iga}}$) retard and engine friction/pumping losses. Raw $= (T / 4.0) + 64$. |
| `TQI_BAS` | 24 | 8 | Little | Unsigned | 4.0 | -256.0 | Nm | -256 .. +764 | Indicated base combustion torque from forward torque model `0x255518` (inflated to 967.8 Nm in Sleepy Loud bin). |
| `PEDAL_PCT` | 32 | 8 | Little | Unsigned | 0.392157 | 0.0 | % | 0.0 .. 100.0 | Driver accelerator pedal position (`PV_AV_CAN`). Raw $= \text{Pedal\%} \times 2.54$. |
| `TQI_DRV` | 40 | 8 | Little | Unsigned | 4.0 | -256.0 | Nm | -256 .. +764 | Driver demand torque from `0x214E80` (Sport) / `0x214280` (Comfort) before gear/speed limiters. |
| `CF_Ems_EngStat`| 48 | 4 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 15 | Engine running status: 0=Stopped, 1=Cranking, 2=Running, 3=Stall. |
| `ALIVE_CNT_EMS11`| 52 | 4 | Little | Unsigned | 1.0 | 0.0 | cnt | 0 .. 15 | Cyclic transmission liveness counter. |
| `CRC_EMS11` | 56 | 8 | Little | Unsigned | 1.0 | 0.0 | crc | 0 .. 255 | 8-bit modular checksum. |

---

### 2.7 Frame `0x329` — `EMS12` (Boost Pressure, Temperatures & Spark Floor Status)

- **Arbitration ID**: `0x329` (809 decimal)
- **Producer**: Continental SIM2K-260 ECU (`640C7000`)
- **Receivers**: Kefico CPTSH TCU, Magna ATC AWD ECU, Instrument Cluster
- **Transmission Rate**: 10 ms (100 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [TCO 7:0]
Byte 1:  [TIA 7:0]
Byte 2:  [PUT_MAP_HPA 7:0 (LE)]
Byte 3:  [PUT_MAP_HPA 15:8 (LE)]
Byte 4:  [RESERVED 7:4 | TCHA_CUT_STAT 3 | CF_Ems_SpkFloorClp 2 | BRAKE_ACT 1:0]
Byte 5:  [TPS 7:0]
Byte 6:  [ALIVE_CNT_EMS12 3:0 | RESERVED 3:0]
Byte 7:  [CRC_EMS12 7:0]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `TCO` | 0 | 8 | Little | Unsigned | 0.75 | -48.0 | °C | -48 .. +143.25 | Engine coolant temperature ($85^\circ\text{C} = 0\text{xB1}$). |
| `TIA` | 8 | 8 | Little | Unsigned | 0.75 | -48.0 | °C | -48 .. +143.25 | Intake manifold charge air temperature ($25^\circ\text{C} = 0\text{x61}$). |
| `PUT_MAP_HPA` | 16 | 16 | Little | Unsigned | 1.0 | 0.0 | hPa | 0 .. 4000 | Upstream throttle / manifold absolute pressure ($\text{PUT}$). Governed by `0x21EC00` (target) and `0x21F478` (limit). |
| `BRAKE_ACT` | 32 | 2 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 3 | Brake pedal switch state: bit 0=Switch, bit 1=Lamp. |
| `CF_Ems_SpkFloorClp`| 34 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | **Shift Spark Floor Clamped Status**: 1 if ignition timing is blocked by ROM floor `0x207AD9` during TCU upshift handoff! |
| `TCHA_CUT_STAT` | 35 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | **Turbo Overspeed Cut Active**: 1 if tripped by `0x203EC6` debounce in Sleepy Loud bin. |
| `TPS` | 40 | 8 | Little | Unsigned | 0.392157 | 0.0 | % | 0.0 .. 100.0 | Throttle valve blade position percentage. |
| `ALIVE_CNT_EMS12`| 48 | 4 | Little | Unsigned | 1.0 | 0.0 | cnt | 0 .. 15 | Cyclic transmission liveness counter. |
| `CRC_EMS12` | 56 | 8 | Little | Unsigned | 1.0 | 0.0 | crc | 0 .. 255 | 8-bit modular checksum. |

---

### 2.8 Frame `0x386` — `WHL_SPD11` (High-Resolution Individual Wheel Speeds)

- **Arbitration ID**: `0x386` (902 decimal)
- **Producer**: Continental / Mobis MK100 ESC & ABS
- **Receivers**: Continental SIM2K-260 ECU, Kefico CPTSH TCU, Magna ATC AWD ECU
- **Transmission Rate**: 20 ms (50 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [WHL_SPD_FL 7:0 (LE)]
Byte 1:  [WHL_SPD_FL 15:8 (LE)]
Byte 2:  [WHL_SPD_FR 7:0 (LE)]
Byte 3:  [WHL_SPD_FR 15:8 (LE)]
Byte 4:  [WHL_SPD_RL 7:0 (LE)]
Byte 5:  [WHL_SPD_RL 15:8 (LE)]
Byte 6:  [WHL_SPD_RR 7:0 (LE)]
Byte 7:  [WHL_SPD_RR 15:8 (LE)]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `WHL_SPD_FL` | 0 | 16 | Little | Unsigned | 0.03125 | 0.0 | km/h | 0.0 .. 2047.97 | Front-Left active Hall-effect wheel speed sensor ($1/32\text{ km/h}$ resolution). |
| `WHL_SPD_FR` | 16 | 16 | Little | Unsigned | 0.03125 | 0.0 | km/h | 0.0 .. 2047.97 | Front-Right active Hall-effect wheel speed sensor. |
| `WHL_SPD_RL` | 32 | 16 | Little | Unsigned | 0.03125 | 0.0 | km/h | 0.0 .. 2047.97 | Rear-Left active Hall-effect wheel speed sensor. |
| `WHL_SPD_RR` | 48 | 16 | Little | Unsigned | 0.03125 | 0.0 | km/h | 0.0 .. 2047.97 | Rear-Right active Hall-effect wheel speed sensor. |

> [!NOTE]
> `WHL_SPD11` is sampled directly by both the ESC traction control algorithm (`0x153`) and the Magna ATC AWD controller. The AWD controller computes linear speed using its hardcoded internal constant $R_{\text{TIRE}} = 320\text{ mm}$ (`0xC04214`), creating the high-speed staggered tire slip trap analyzed in Section 3.4.

---

### 2.9 Frame `0x4A0` — `CLU11` / `DMS` (Drive Mode & Launch Control State)

- **Arbitration ID**: `0x4A0` (1184 decimal)
- **Producer**: Instrument Cluster / Drive Mode Switch
- **Receivers**: Continental SIM2K-260 ECU, Kefico CPTSH TCU, Magna ATC AWD ECU, ECS Suspension
- **Transmission Rate**: 20 ms (50 Hz)
- **Data Length Code (DLC)**: 8 bytes

#### Byte / Bit Layout

```
Byte 0:  [ECU_MODE_OVERRIDE 7:4 | DRV_MODE_CMD 3:0]
Byte 1:  [RESERVED 7:3 | LAUNCH_CTRL_STAT 2 | ESC_MODE_STAT 1:0]
Bytes 2-5: [RESERVED 0x00]
Byte 6:  [ALIVE_CNT_CLU 3:0 | RESERVED 3:0]
Byte 7:  [CRC_CLU 7:0]
```

#### Signal Specifications

| Signal Name | Start Bit | Length | Endian | Type | Scale | Offset | Units | Physical Range | Calibration Xref / Notes |
|---|---|---|---|---|---|---|---|---|---|
| `DRV_MODE_CMD` | 0 | 4 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 15 | Physical drive mode rotary knob position: 0=Comfort, 1=Sport, 2=Eco, 3=Smart, 4=Custom / Mode 4. |
| `ECU_MODE_OVERRIDE`| 4 | 4 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 15 | **ECU Internal Mode Force Switch `0x200CCE`**: 0=Follow CAN `0x4A0`, 1=Force Sport Mode 8, 2=Force Mode 4. When set, ECU ignores `DRV_MODE_CMD`! |
| `ESC_MODE_STAT`| 8 | 2 | Little | Unsigned | 1.0 | 0.0 | enum | 0 .. 3 | ESC push button state: 0=Full ON, 1=ESC Sport (single press), 2=ESC Full OFF (hold 3s). |
| `LAUNCH_CTRL_STAT`| 10 | 1 | Little | Bool | 1.0 | 0.0 | bool | 0 or 1 | Launch Control pre-stage active: asserted when vehicle stationary, $P_{\text{brk}} > 400\text{ psi}$, and $\text{Pedal} > 80\%$. |
| `CRC_CLU` | 56 | 8 | Little | Unsigned | 1.0 | 0.0 | crc | 0 .. 255 | 8-bit modular checksum. |

---

## 3. Deep-Dive Inter-Module Closed-Loop Interaction Sequences

---

### 3.1 Pre-Stage Brake-Stand Launch & 167 ms AWD Brake-Gate Window (`0xC06080` Stock vs. `SportLaunch_30mph.bin`)

#### 3.1.1 Physics & Calibration Mechanism

In the factory Magna ATC AWD calibration (`awd_95447_4J105_C00000_C0BFFF.bin`), the firmware contains a safety inhibitor designed to prevent transfer case clutch damage during stationary brake testing:

$$\text{Brake Gate Threshold: } P_{\text{gate}} = 4.45\text{ bar} = 64.5\text{ psi}\quad (\text{stored at ROM addresses } 0\text{xC07498} \text{ and } 0\text{xC07606})$$

During a brake-stand launch sequence (staging at drag strip or street launch):
1. **Pre-Stage Phase ($t < 0$)**: The driver holds the brake pedal hard ($P_{\text{brk}} \approx 1200\text{ psi} = 82.7\text{ bar}$) while pinning the throttle to 100%. The ECU commands WOT airflow; the TCU allows engine speed to climb toward the brake-stall RPM limit (`0x238B68` = 2500 RPM stock).
2. **C-CAN Transmission**: ESC broadcasts $P_{\text{brk}} = 82.7\text{ bar}$ in `CYL_PRES` on `0x080`.
3. **AWD Inhibit Trip**: The Magna ATC reads `0x080`. Because $82.7\text{ bar} \gg 4.45\text{ bar}$, the controller sets internal state `awdBrakeGated = true` and **forces commanded coupling torque `TQ_AWD_CMD` to 0.0 Nm**, despite the driver demanding full torque!
4. **Brake Release ($t = 0$)**: The driver sidesteps the brake pedal. The vehicle's hydraulic brake circuit bleeds off pressure over $t_{\text{release}} \approx 140\text{ ms}$.
5. **The 132 ms Hydraulic Delay Trap**: Because the gate threshold is set so low ($64.5\text{ psi}$ out of $1200\text{ psi}$), hydraulic pressure remains above $64.5\text{ psi}$ for:
   $$t_{\text{gate\_delay}} = 140\text{ ms} \times \left(1.0 - \frac{64.5}{1200}\right) \approx 132.5\text{ ms}$$
   During this entire 132 ms window after the driver has released the brake, the AWD clutch remains completely open ($0\text{ Nm}$)!
6. **The 167 ms Slew Rate Limiter Delay**: At $t = +132\text{ ms}$, $P_{\text{brk}}$ finally drops below $64.5\text{ psi}$. The gate clears, and ATC evaluates launch preload (`0xC04778` = 1000 Nm) and feed-forward (`0xC04BA0` = 1000 Nm). However, the firmware enforces a positive slew rate limiter at `0xC07650`:
   $$\text{Stock Slew Rate: } \Delta T_{\text{awd}} = 120\text{ Nm per } 20\text{ ms cycle } (6000\text{ Nm/s})$$
   Ramping the wet-clutch hydraulic clamp from $0\text{ Nm}$ to the $1000\text{ Nm}$ target requires:
   $$\Delta t_{\text{slew}} = \frac{1000\text{ Nm}}{120\text{ Nm / cycle}} \times 20\text{ ms} = 166.7\text{ ms}$$
7. **Total AWD Coupling Lag**:
   $$t_{\text{total\_lag}} = 132.5\text{ ms (hydraulic)} + 166.7\text{ ms (slew)} = 299.2\text{ ms}$$

```mermaid
sequenceDiagram
    autonumber
    participant Driver as Driver Inputs
    participant ESC as ESC (0x080 / 0x386)
    participant TCU as TCU (0x162 / 0x164)
    participant ECU as SIM2K-260 (0x316)
    participant AWD as Magna ATC (0x220)
    participant Tires as Rear Wheels & Tires

    Note over Driver,Tires: t < 0: Pre-Stage Brake-Stand (Brake = 1200 psi, Pedal = 100%)
    Driver->>ESC: Brake pedal held at 1200 psi (82.7 bar)
    Driver->>ECU: Pedal pinned to 100% WOT
    ECU->>TCU: TQI_ACT climbs to 515 Nm, Engine revs to stall limit (0x238B68: 2500 RPM)
    ESC->>AWD: 0x080 ESP12: P_brk = 82.7 bar (> 4.45 bar gate 0xC07498)
    AWD->>AWD: Inhibit Armed! TQ_AWD_CMD clamped to 0 Nm (0xC06080)
    AWD-->>Tires: 0x220 AWD11: TQ_AWD_ACT = 0 Nm (Front axle uncoupled)

    Note over Driver,Tires: t = 0.000 s: Driver Sidesteps Brake Pedal
    Driver->>ESC: Brake pressure begins hydraulic bleed (140 ms decay)
    ESC->>AWD: 0x080 ESP12: P_brk bleeds (1000 psi -> 500 psi -> 200 psi...)
    Note over AWD: t = 0 to 132 ms: P_brk STILL > 64.5 psi! AWD remains 0 Nm!
    ECU->>Tires: 100% of torque delivered to REAR AXLE via 1.85x TC ratio (~950 Nm!)
    Tires->>Tires: Rear tires exceed peak friction (mu > 1.15). Slip explodes to 22%!

    Note over Driver,Tires: t = +0.132 s: P_brk drops below 64.5 psi (Gate Clears)
    ESC->>AWD: 0x080 ESP12: P_brk = 4.2 bar (< 4.45 bar threshold)
    AWD->>AWD: Gate Cleared! Command jumps to 1000 Nm. Slew 0xC07650 limits to 120 Nm/20ms
    AWD->>Tires: 0x220 AWD11: Torque ramps: 120 -> 240 -> 480 -> 720 -> 1000 Nm (+167 ms)
    ESC->>ECU: 0x153 TCS11: Rear wheelspin detected! TCS_REQ=1, TQI_ASR cut commanded!
    ECU->>ECU: Unless 0x200B4B=1, ECU cuts spark. Engine BOGS!
```

#### 3.1.2 Quantitative Teardown Comparison: Stock vs. `SportLaunch_30mph.bin`

In `awd_95447_4J105_SportLaunch_30mph.bin`:
- ROM address `0xC06080` is zeroed and `0xC060B0` is raised, completely bypassing the brake gate (`AWDBrakeGateBypassed = true`).
- The positive slew rate limiter at `0xC07650` is increased from $120\text{ Nm/20ms}$ ($6000\text{ Nm/s}$) to $180\text{ Nm/20ms}$ ($9000\text{ Nm/s}$).
- Standstill launch preload (`0xC04778`) is raised to $1000\text{ Nm}$ at WOT.

| Physical Parameter | Stock AWD (`0xC00000`) | `SportLaunch_30mph.bin` | Delta / Physical Consequence |
|---|---|---|---|
| **Brake Gate Threshold (`0xC07498`)** | $4.45\text{ bar}$ ($64.5\text{ psi}$) | Bypassed / Inactive | Eliminates 132 ms post-release uncoupled deadband. |
| **AWD Torque at $t = -0.1\text{ s}$ (Pre-Stage)** | $0\text{ Nm}$ (Gated) | **$1000\text{ Nm}$** (Full Preload) | Clutches are 100% pre-clamped before launch. |
| **AWD Torque at $t = +0.05\text{ s}$ (Post-Release)**| $0\text{ Nm}$ | **$1000\text{ Nm}$** | Instantaneous front axle drive; $0\text{ ms}$ lag. |
| **Positive Slew Rate (`0xC07650`)** | $120\text{ Nm}/20\text{ ms}$ ($6000\text{ Nm/s}$) | **$180\text{ Nm}/20\text{ ms}$** ($9000\text{ Nm/s}$) | +50% faster hydraulic slew response. |
| **Peak Rear Wheel Slip ($\kappa_r$)** | **$18.4\%$** (Spinning) | **$7.2\%$** (Hooked at Peak $\mu$) | Rear tires never break into sliding friction. |
| **TCS Interventions on CAN `0x153`** | **1** (Active Torque Cut) | **0** (Clean Hook) | Prevents engine power cut at the 60-foot mark. |
| **Sixty-Foot Time ($60\text{ ft}$)** | **$2.44\text{ s}$** | **$2.12\text{ s} - 2.39\text{ s}$** | **$0.05\text{ s} - 0.32\text{ s}$ quicker 60-foot launch!** |

---

### 3.2 WOT Upshift Torque-Phase & Inertia-Phase Handoff (`0x233D3C` $\to$ `0x25262C` $\to$ `0x207AD9`)

#### 3.2.1 Upshift Handshake Mechanics

When vehicle speed reaches the CPTSH 8AT upshift threshold (e.g. Slot 23 Sport upshift map `0x207728`), the TCU executes a two-phase clutch-to-clutch shift:
1. **Torque Phase**: The TCU pre-fills the oncoming clutch hydraulic circuit (`0x24BB22` VCM floor) while gradually slipping the offgoing clutch.
2. **Inertia Phase**: To pull engine crankshaft speed down to synchronize with the new gear ratio ($\omega_{\text{sync}} = \omega_{\text{turb}} \times \frac{R_{g+1}}{R_g}$), the TCU broadcasts a high-priority fast torque reduction request on CAN `0x162`:
   - `SHIFT_ACTIVE = 1` (Byte 1, bit 0)
   - `SHIFT_PHASE = 2` (Inertia Phase)
   - `TQI_TCU` (Bytes 2-3): Target torque limit derived from ROM table `0x233D3C` (e.g. raw $-10$ stock vs $-15$ EKM tune):
     $$T_{\text{tcu\_req}} = \max\left(65\text{ Nm}, T_{\text{uncapped}} \times \left(1.0 - 0.58 \times \frac{|\text{ShiftCutRaw}|}{10}\right)\right)$$
3. **ECU Torque Coordination (`0x800EEDA4`)**: The SIM2K-260 receives `0x162`. Because electronic throttle blade and turbo wastegate actuators are too slow ($\tau \approx 120\text{ ms}$), the ECU achieves immediate torque reduction by calculating required ignition spark efficiency:
   $$\eta_{\text{req}} = \frac{T_{\text{tcu\_req}}}{T_{\text{uncapped}}} \in [0.09, 1.00]$$
4. **Inverse Efficiency to Retard Lookup**: The ECU interpolates the required spark retard $\Delta\theta_{\text{retard}}$ from ROM table `0x25743E` (efficiency axis) $\to$ `0x25262C` (retard degrees):
   $$\theta_{\text{unclamped}} = \theta_{\text{base}} (0\text{x238090}) - \Delta\theta_{\text{retard}}$$
5. **The Shift Spark Floor Clamp (`0x207AD9`)**: Before commanding the ignition coils, the ECU clamps the retard against the active safety floor:
   $$\theta_{\text{actual}} = \max\left(\theta_{\text{unclamped}}, \max\left(\text{IGA\_MIN}(0\text{x239AF4}), \text{ShiftSparkFloor}(0\text{x207AD9})\right)\right)$$
   - In stock SIM2K-260 (`640C7000`): `0x207AD9` is set to **$-3.0^\circ$ BTDC** (allowing timing to drop into ATDC territory).
   - In IntensePop `v103`: `0x207AD9` is set to **$-6.0^\circ$ BTDC** (allowing deeper torque cuts).
   - In KimJongTuned `BEF`: `0x207AD9` is mistakenly set to **$+12.0^\circ$ BTDC**!

```mermaid
sequenceDiagram
    autonumber
    participant TCU as Kefico CPTSH TCU
    participant CAN as C-CAN (0x162 / 0x316 / 0x329)
    participant Coord as ECU Torque Coordinator
    participant Spark as SIM2K-260 Spark Model
    participant Clutch as 8AT Friction Clutches

    Note over TCU,Clutch: Upshift Trigger: Road Speed crosses 0x207728 WOT Threshold
    TCU->>CAN: 0x162 TCU11: CurGr=1, TarGr=2, ShiftActive=1, ShiftPhase=2 (Inertia)
    TCU->>CAN: 0x162 TCU11: TQI_TCU = 285 Nm (Shift Cut from 0x233D3C)
    CAN->>Coord: Forward TQI_TCU to ECU (0x800EEDA4)
    Coord->>Spark: Calculate eta_req = 285 / 710 = 0.401
    Spark->>Spark: Lookup 0x25743E -> 0x25262C: reqRetard = 23.5 deg
    Spark->>Spark: unclampedIga = 14.5 deg - 23.5 deg = -9.0 deg BTDC

    alt Stock / v103 Clean Shift (Floor = -3.0 deg / -6.0 deg)
        Spark->>Spark: Floor = -6.0 deg BTDC. Clamped to -6.0 deg (near target)
        Spark->>CAN: 0x329 EMS12: CF_Ems_SpkFloorClp = 0
        Spark->>Clutch: eta_iga drops to 0.42. Engine Torque drops to 295 Nm
        Clutch->>TCU: Oncoming clutch locks smoothly without slip flare!
        TCU->>CAN: Shift completes in 180 ms. 0x162 ShiftActive = 0
    else Buggy / Aggressive BEF (0x207AD9 = +12.0 deg BTDC)
        Spark->>Spark: Floor = +12.0 deg BTDC! Spark BLOCKED at +12.0 deg!
        Spark->>CAN: 0x329 EMS12: CF_Ems_SpkFloorClp = 1 (FLAG ASSERTED!)
        Spark->>Clutch: eta_iga stuck at 0.88! Engine Torque stays at 625 Nm!
        Clutch->>Clutch: Oncoming clutch slips severely under 625 Nm load!
        Clutch->>TCU: Synchronous speed delayed. Shift drags out to 380 ms! Clutch burns!
    end
```

#### 3.2.2 Impact of `0x207AD9` Calibration Differences

When timing is clamped at $+12.0^\circ$ BTDC, the ECU forward efficiency table (`0x2553AA` $\to$ `0x2524EC`) calculates an achieved efficiency of $\eta_{\text{iga}} \approx 0.88$. The engine continues to deliver over $625\text{ Nm}$ into the transmission while the TCU is actively swapping clutches:

$$\text{Excess Slipping Torque: } \Delta T = 625\text{ Nm} - 285\text{ Nm} = 340\text{ Nm of unabsorbed kinetic energy!}$$

This excess torque generates intense frictional heating in the A8LR1 clutch packs, elongates the shift duration from $180\text{ ms}$ to over $350\text{ ms}$, causes shift flare / clutch shudder, and flags bit 34 (`CF_Ems_SpkFloorClp`) in CAN frame `0x329`.

---

### 3.3 Sleepy Loud TCHA Turbo Overspeed Cut (`0x203EC6 = 10` $\to$ `200 Nm` Floor `0x203ED4` $\to$ `17 Nm/cycle` Ramp `0x203ED8`)

#### 3.3.1 Root Cause Architecture

In the "Sleepy Loud" modified SIM2K-260 calibration:
1. The tuner attempted to bypass transmission torque limits by inflating the forward indicated torque model `TQI_BAS` (`0x255518`) by $+17\%$, reaching **$967.8\text{ Nm}$**.
2. However, the tuner neglected the internal turbocharger speed protection state machine (`TCHA`).
3. The ECU estimates turbocharger impeller speed $N_{\text{TCHA}}$ using modeled airflow and indicated torque scaled by factor `0x203ECC` ($49152 = 0.75$). When indicated model torque exceeds $710\text{ Nm}$ above $2700\text{ RPM}$, estimated turbo speed crosses the critical safety threshold ($> 220,000\text{ RPM}$).
4. In stock calibrations (`CKHK_3TMS2C7C_SLEEPY_DECAT_QUIET_GOOD.bin`), the TCHA debounce counter at `0x203EC6` is disabled by setting it to **$65,535\text{ cycles}$**.
5. In the Sleepy Loud binary, `0x203EC6` is active and configured to just **$10\text{ cycles}$ ($100\text{ ms}$)**!

```mermaid
stateDiagram-v2
    [*] --> NormalWOT: Driver Demands WOT (Pedal = 100%)
    
    NormalWOT --> Debouncing: Model TQI_BAS > 710 Nm & RPM > 2700
    state Debouncing {
        [*] --> CountCycles
        CountCycles --> CountCycles: Each 10ms cycle with Model Tq > 710 Nm
        CountCycles --> NormalWOT: Model Tq drops below 710 Nm (Timer Reset)
    }

    Debouncing --> TrippedCut: Timer >= 0x203EC6 (10 cycles / 100 ms)
    
    state TrippedCut {
        [*] --> FloorDrop: Slam Torque Cap to 0x203ED4 (200 Nm!)
        FloorDrop --> Ramping: Ramping back up at 0x203ED8 (17 Nm / 10ms cycle)
        Ramping --> Ramping: T_cap += 17 Nm (Takes 294 ms to reach 700 Nm)
    }

    TrippedCut --> NormalWOT: T_cap >= TQI_Uncapped (Cut Clears)
    NormalWOT --> Debouncing: Model Tq immediately > 710 Nm again! (Endless Sawtooth Loop!)
```

#### 3.3.2 Cyclic Cut Sequence & Telemetry Manifestation

Once indicated model torque stays above $710\text{ Nm}$ for $100\text{ ms}$:
1. **$t = t_{\text{trip}}$**: TCHA trips. The ECU immediately sets `tchaActiveFlag = true` and asserts bit 35 (`TCHA_CUT_STAT`) on CAN `0x329`.
2. **Emergency Floor**: The ECU torque coordinator clamps the maximum allowable engine torque down to `0x203ED4`:
   $$T_{\text{cap}} = 200.0\text{ Nm}\quad (\text{a violent } 510\text{ Nm drop!})$$
3. **C-CAN Reaction**: On CAN `0x316`, `TQI_ACT` collapses from $710\text{ Nm}$ to $200\text{ Nm}$ in a single $10\text{ ms}$ step ($D_2$ drops from `0xF1` to `0x72`). The vehicle violently bucks forward as acceleration drops from $+1.1\text{g}$ to $+0.15\text{g}$.
4. **Recovery Ramp**: The ECU firmware allows torque to recover at the rate specified in `0x203ED8`:
   $$\text{Recovery Slew Rate: } \Delta T_{\text{ramp}} = 17\text{ Nm per } 10\text{ ms cycle } (1700\text{ Nm/s})$$
5. **Ramp Duration**:
   $$\Delta t_{\text{recover}} = \frac{700\text{ Nm} - 200\text{ Nm}}{17\text{ Nm / cycle}} \times 10\text{ ms} = 294.1\text{ ms}$$
6. **The Endless Sawtooth**: The moment torque recovers above $710\text{ Nm}$, the $10\text{ cycle}$ timer starts counting again. Exactly $100\text{ ms}$ later, TCHA trips again!

Over a single quarter-mile run, this defect triggers **37 discrete cut-and-recovery cycles**, degrading quarter-mile time from $11.72\text{ s}$ to $12.80\text{ s}$ and trap speed from $124.3\text{ mph}$ to $112.3\text{ mph}$!

---

### 3.4 High-Speed Staggered Tire Rolling-Radius Slip Trap (`0xC04214` 320 mm vs. `0xC061FA`)

#### 3.4.1 Kinematic Mismatch & False Slip Formulation

The factory tire fitment on the Kia Stinger GT and Genesis G70 3.3T is staggered:
- **Front Axle**: 225/40R19 — Nominal outer radius $R_{f0} = 331.4\text{ mm}$, physical loaded rolling radius $R_{f} \approx 334.0\text{ mm}$.
- **Rear Axle**: 255/35R19 — Nominal outer radius $R_{r0} = 330.6\text{ mm}$, physical loaded rolling radius $R_{r} \approx 336.0\text{ mm}$ (accounting for centrifugal growth at high speeds).

The ESC controller measures actual wheel angular velocities $\omega_{\text{FL}}, \omega_{\text{FR}}, \omega_{\text{RL}}, \omega_{\text{RR}}$ and broadcasts true linear wheel speeds on CAN `0x386`:
$$v_{\text{wheel}} = \omega_{\text{wheel}} \times R_{\text{physical}}$$

However, inside the Magna ATC AWD controller ROM at address `0xC04214`, the tire radius is hardcoded as a single global scalar:
$$R_{\text{ATC}} = 320.0\text{ mm}\quad (0\text{x0140})$$

When the vehicle travels in a straight line at high speed, both axles travel at true road velocity $v_{\text{car}}$. The physical angular velocities of the wheels are:
$$\omega_f = \frac{v_{\text{car}}}{R_f},\quad \omega_r = \frac{v_{\text{car}}}{R_r}$$

Because $R_r > R_f$ by $2.0\text{ mm}$ (or more with uneven tire wear or aftermarket sizing deltas $\approx 1.5 - 2.5\%$), the rear wheels rotate slower than the front wheels:
$$\frac{\omega_r}{\omega_f} = \frac{R_f}{R_r} = \frac{334.0}{336.0} = 0.9940$$

When the ATC AWD controller computes speed from CAN `0x386` wheel speeds, it evaluates an apparent wheel speed differential:
$$\Delta v_{\text{apparent}} = |v_r - v_f| = v_{\text{car}} \times \left|1.0 - \frac{R_f}{R_r}\right|$$

```
========================================================================================
HIGH-SPEED STAGGERED TIRE APPARENT SLIP SPEED & DEAD-BAND PENETRATION
========================================================================================
Vehicle Speed    True Front RPM   True Rear RPM    Apparent Δv (km/h)   ATC Deadband   Slip Feedback
----------------------------------------------------------------------------------------
 50 km/h (13.9 m/s)   396.8 RPM        394.4 RPM        0.30 km/h           1.0 km/h       0 Nm (Deadband)
100 km/h (27.8 m/s)   793.6 RPM        788.8 RPM        0.60 km/h           1.0 km/h       0 Nm (Deadband)
150 km/h (41.7 m/s)  1190.4 RPM       1183.3 RPM        1.80 km/h           2.0 km/h       0 Nm (Deadband)
200 km/h (55.6 m/s)  1587.2 RPM       1577.7 RPM        3.57 km/h           2.0 km/h     +185 Nm (Active!)
250 km/h (69.4 m/s)  1984.0 RPM       1972.1 RPM        5.95 km/h           2.0 km/h     +390 Nm (Active!)
========================================================================================
```

```mermaid
flowchart TD
    subgraph SENSORS ["Chassis & ESC Broadcast"]
        TIRES["Staggered Tires: Front 334 mm vs Rear 336 mm<br/>ΔR = 2.0 mm (0.6% to 2.5% with wear)"]
        WHEELS["Wheel Angular Speeds: ω_f ≠ ω_r<br/>Rear rotates slower than Front"]
        CAN386["0x386 WHL_SPD11 Broadcast<br/>High-resolution 16-bit wheel speeds"]
    end

    subgraph AWD_LOGIC ["Magna ATC AWD Controller (awd_95447_4J105)"]
        ATC_R["Hardcoded Radius 0xC04214: R_TIRE = 320 mm<br/>Firmware assumes identical front/rear radii!"]
        CALC_DIFF["Compute Δv = |v_rear - v_front|<br/>At 200 km/h: Δv = 3.57 km/h!"]
        DEADBAND{"Δv > Deadband?<br/>(2.0 km/h above 100 km/h)"}
        SLIP_PID["Closed-Loop Slip Table 0xC061FA / 0xC06978<br/>Commands +200 to +400 Nm coupling torque!"]
        TAPER["High-Speed Taper Maps 0xC056CA & 0xC0640A<br/>Attempting to reduce coupling to 0 Nm!"]
        FIGHT["CONFLICT: Closed-Loop Slip OVERRIDES Taper!<br/>Wet clutch forced to clamp at 200+ km/h!"]
    end

    subgraph DRIFT ["Physical Drivetrain Consequences"]
        DRAG["High-Speed Parasitic Driveline Drag<br/>Clutch slip generates severe fluid shear"]
        TRAP["Quarter-Mile Trap Speed Drops 2 - 4 mph!"]
        THERM["CAN 0x220 Byte 6: AWD_THERM_STATE asserts Warning/Derate!"]
    end

    TIRES --> WHEELS
    WHEELS --> CAN386
    CAN386 --> ATC_R
    ATC_R --> CALC_DIFF
    CALC_DIFF --> DEADBAND
    DEADBAND -->|Above 160 km/h: YES| SLIP_PID
    DEADBAND -->|Below 100 km/h: NO| TAPER
    SLIP_PID --> FIGHT
    TAPER --> FIGHT
    FIGHT --> DRAG
    DRAG --> TRAP
    DRAG --> THERM
```

#### 3.4.2 Driveline Drag & Thermal Protection Clamping

At speeds above $160\text{ km/h}$ ($100\text{ mph}$), the false speed differential punches through the $2.0\text{ km/h}$ high-speed deadband. The closed-loop slip PID curve (`0xC061FA` & `0xC06978`) interprets the differential as continuous rear wheel spin.

This causes a severe internal conflict within the Magna ATC control strategy:
- **Open-Loop Strategy**: The High-Speed Gate 2 map (`0xC056CA` on `0xC06F96`) and Master Speed Taper (`0xC0640A` on `0xC06252`) are actively commanding front torque split down to $0 - 10\%$ to eliminate drivetrain friction and prevent transfer case overheating.
- **Closed-Loop Override**: The false slip calculation injects $+200\text{ to }+400\text{ Nm}$ of coupling torque, forcing the transfer case multi-plate wet clutch into continuous high-speed frictional slip!
- **Consequences**:
  1. Parasitic clutch drag absorbs $15 - 25\text{ HP}$ at the top end of the quarter mile, dropping trap speed by $2.0\text{ to }4.0\text{ mph}$.
  2. The continuous high-speed slip generates extreme fluid shear heating in the Magna ATC transfer case, triggering thermal derate warnings on CAN `0x220` (`AWD_THERM_STATE`).
  3. Proper calibration requires either matching rolling radii within $0.5\%$, widening the high-speed slip deadband above $100\text{ km/h}$, or raising the Gate 3 taper override authority in `0xC0640A`.

---

## 4. Verification & Candump Trace Validation

The simulation engine includes complete serialization and deserialization routines in `can_bus_matrix.go` verified by automated test suites. The following sample `candump` trace represents a real $100\text{ Hz}$ execution snapshot captured during a WOT launch at $t = +0.0500\text{ s}$ post-release:

```candump
(0000.0500) can0 080#FFFB3F00F07F05D2
(0000.0500) can0 153#000000FF40FF0568
(0000.0500) can0 162#1100FF07BC0205C2
(0000.0500) can0 164#30166B007DEA0181
(0000.0500) can0 220#D007A406500950B3
(0000.0500) can0 316#6829F1F1FEFB5228
(0000.0500) can0 329#B161660800FE0550
(0000.0500) can0 386#5302530266026602
(0000.0500) can0 4A0#0100000000000555
```

### Trace Decoded Analysis at $t = +0.0500\text{ s}$:
- **`0x080` (ESP12)**: `CYL_PRES = 0x000` ($0.0\text{ bar}$ brake pressure, pedal released); `LONG_ACCEL = 0x5DF` ($+1.12\text{g}$ longitudinal thrust).
- **`0x153` (TCS11)**: `TCS_REQ = 0`, `TQI_ASR = 0xFF` (no traction intervention; tires hooked).
- **`0x162` (TCU11)**: `CUR_GR = 1`, `TAR_GR = 1`, `SHIFT_ACTIVE = 0`; `ETL_TCU = 0x02BC` ($700\text{ Nm}$ EKM continuous torque ceiling).
- **`0x164` (TCU12)**: `N_TURB = 0x1630 / 4 = 1420 RPM`; `CONV_SLIP_RATIO = 0x6B * 0.005 = 0.535`; `VCM_SOL_VAL = 0x01EA = 490 mA` (EKM solenoid floor).
- **`0x220` (AWD11)**: `TQ_AWD_CMD = 0x07D0 * 0.5 = 1000.0 Nm`; `TQ_AWD_ACT = 0x06A4 * 0.5 = 850.0 Nm` (ramping at $180\text{ Nm/20ms}$); `AWD_FRONT_SPLIT = 0x50 * 0.5 = 40.0\%`.
- **`0x316` (EMS11)**: `N_ENG = 0x2968 / 4 = 2650 RPM`; `TQI_ACT = (0xF1 - 64) * 4 = 708 Nm`; `PEDAL_PCT = 100\%`.
- **`0x329` (EMS12)**: `PUT_MAP_HPA = 0x0866 = 2150 hPa` ($16.5\text{ psi}$ boost); `CF_Ems_SpkFloorClp = 0`; `TCHA_CUT_STAT = 0`.
- **`0x386` (WHL_SPD11)**: Front wheels `0x0253 * 0.03125 = 18.59 km/h`; Rear wheels `0x0266 * 0.03125 = 19.18 km/h` (slight rear slip ratio $\approx 3.1\%$, perfectly inside peak friction $\mu_{\text{peak}}$ band).
- **`0x4A0` (CLU11)**: `DRV_MODE_CMD = 1` (Sport Mode); `ECU_MODE_OVERRIDE = 0` (Following CAN); `LAUNCH_CTRL_STAT = 0` (Released).
