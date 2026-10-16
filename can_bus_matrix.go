package main

import (
	"encoding/binary"
	"fmt"
	"math"
)

// CANSignalSpec defines the exact bit layout, endianness, scaling factor, offset,
// engineering units, and valid range of a signal within an 8-byte C-CAN frame.
type CANSignalSpec struct {
	Name        string  `json:"name"`
	StartBit    int     `json:"start_bit"`
	BitLength   int     `json:"bit_length"`
	ByteOrder   string  `json:"byte_order"` // "little_endian" or "big_endian"
	IsSigned    bool    `json:"is_signed"`
	Scale       float64 `json:"scale"`
	Offset      float64 `json:"offset"`
	Unit        string  `json:"unit"`
	MinVal      float64 `json:"min_val"`
	MaxVal      float64 `json:"max_val"`
	Description string  `json:"description"`
}

// CANFrameSpec provides the complete frame metadata, bus transmission rate,
// producer/consumer topology, and constituent signal definitions for a C-CAN frame.
type CANFrameSpec struct {
	ArbitrationID  uint32          `json:"arb_id"`
	ArbitrationHex string          `json:"arb_hex"` // e.g. "0x080"
	Name           string          `json:"name"`    // e.g. "ESP12"
	PeriodMs       int             `json:"period_ms"`
	Producer       string          `json:"producer"`
	Receivers      []string        `json:"receivers"`
	DLC            int             `json:"dlc"`
	Description    string          `json:"description"`
	Signals        []CANSignalSpec `json:"signals"`
}

// EncodedCANFrame holds a timestamped, serialized 8-byte C-CAN payload
// alongside formatted candump strings and decoded physical signal values.
type EncodedCANFrame struct {
	TimestampSec   float64                `json:"timestamp_sec"`
	ArbitrationID  uint32                 `json:"arb_id"`
	ArbitrationHex string                 `json:"arb_hex"`
	FrameName      string                 `json:"frame_name"`
	Producer       string                 `json:"producer"`
	DLC            int                    `json:"dlc"`
	Data           [8]byte                `json:"data"`
	DataHex        string                 `json:"data_hex"`
	CandumpLine    string                 `json:"candump_line"`
	DecodedSignals map[string]interface{} `json:"decoded_signals"`
}

// GetCCANMatrixCatalog returns the complete architectural specification for all 9 C-CAN
// frames governing the Continental SIM2K-260 ECU, Kefico CPTSH 8AT TCU, Magna ATC AWD ECU,
// and ESC/ABS controller.
func GetCCANMatrixCatalog() []CANFrameSpec {
	return []CANFrameSpec{
		{
			ArbitrationID:  0x080,
			ArbitrationHex: "0x080",
			Name:           "ESP12",
			PeriodMs:       20,
			Producer:       "ESC/ABS",
			Receivers:      []string{"SIM2K-260 ECU", "Kefico CPTSH TCU", "Magna ATC AWD ECU"},
			DLC:            8,
			Description:    "Chassis dynamics, yaw rate, lateral/longitudinal acceleration, and master cylinder brake pressure (P_brk) driving the Magna ATC 64.5 psi launch gate.",
			Signals: []CANSignalSpec{
				{
					Name:        "LAT_ACCEL",
					StartBit:    0,
					BitLength:   11,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.01,
					Offset:      -10.23,
					Unit:        "m/s^2",
					MinVal:      -10.23,
					MaxVal:      10.24,
					Description: "Lateral acceleration measured by ESC IMU sensor cluster.",
				},
				{
					Name:        "LONG_ACCEL",
					StartBit:    11,
					BitLength:   11,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.01,
					Offset:      -10.23,
					Unit:        "m/s^2",
					MinVal:      -10.23,
					MaxVal:      10.24,
					Description: "Longitudinal acceleration (accelG * 9.80665) used by AWD slip predictor and TCU shift adaptation.",
				},
				{
					Name:        "BRK_SW_STAT",
					StartBit:    22,
					BitLength:   2,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      3,
					Description: "Primary and secondary brake switch circuit state.",
				},
				{
					Name:        "CYL_PRES",
					StartBit:    24,
					BitLength:   12,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.1,
					Offset:      0.0,
					Unit:        "bar",
					MinVal:      0.0,
					MaxVal:      409.5,
					Description: "Master cylinder hydraulic brake pressure (P_brk). Gates Magna ATC AWD coupling at 0xC07498 / 0xC07606 (4.45 bar = 64.5 psi).",
				},
				{
					Name:        "YAW_RATE",
					StartBit:    36,
					BitLength:   12,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.01,
					Offset:      -20.47,
					Unit:        "deg/s",
					MinVal:      -20.47,
					MaxVal:      20.48,
					Description: "Vehicle body yaw angular velocity around vertical axis.",
				},
				{
					Name:        "ALIVE_CNT_ESP",
					StartBit:    48,
					BitLength:   4,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "cnt",
					MinVal:      0,
					MaxVal:      15,
					Description: "Rolling transmission liveness counter (0..15).",
				},
				{
					Name:        "CRC_ESP",
					StartBit:    56,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "crc",
					MinVal:      0,
					MaxVal:      255,
					Description: "Standard 8-bit Hyundai/Kia CAN frame checksum.",
				},
			},
		},
		{
			ArbitrationID:  0x153,
			ArbitrationHex: "0x153",
			Name:           "TCS11",
			PeriodMs:       10,
			Producer:       "ESC/ABS",
			Receivers:      []string{"SIM2K-260 ECU", "Kefico CPTSH TCU"},
			DLC:            8,
			Description:    "Traction control system (ASR/MSR) torque reduction requests. Gated inside ECU by TCS Inhibit Switch 0x200B4B.",
			Signals: []CANSignalSpec{
				{
					Name:        "TCS_REQ",
					StartBit:    0,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "ASR traction control torque cut intervention active flag.",
				},
				{
					Name:        "MSR_C_REQ",
					StartBit:    1,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Engine drag torque control (MSR) active flag.",
				},
				{
					Name:        "TCS_MFRN",
					StartBit:    15,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Fast spark torque cut intervention request flag.",
				},
				{
					Name:        "TQI_ASR_FAST",
					StartBit:    24,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       4.0,
					Offset:      -256.0,
					Unit:        "Nm",
					MinVal:      -256.0,
					MaxVal:      764.0,
					Description: "Fast ignition retard/cut torque ceiling request to ECU torque coordinator (gated by ECU 0x200B4B).",
				},
				{
					Name:        "TQI_MSR",
					StartBit:    32,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       4.0,
					Offset:      -256.0,
					Unit:        "Nm",
					MinVal:      -256.0,
					MaxVal:      764.0,
					Description: "MSR engine drag torque increase request.",
				},
				{
					Name:        "TQI_ASR_SLOW",
					StartBit:    40,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       4.0,
					Offset:      -256.0,
					Unit:        "Nm",
					MinVal:      -256.0,
					MaxVal:      764.0,
					Description: "Slow electronic throttle / boost wastegate torque ceiling request.",
				},
				{
					Name:        "ALIVE_CNT_TCS",
					StartBit:    48,
					BitLength:   4,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "cnt",
					MinVal:      0,
					MaxVal:      15,
					Description: "Rolling transmission liveness counter (0..15).",
				},
				{
					Name:        "CRC_TCS",
					StartBit:    56,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "crc",
					MinVal:      0,
					MaxVal:      255,
					Description: "Standard 8-bit Hyundai/Kia CAN frame checksum.",
				},
			},
		},
		{
			ArbitrationID:  0x162,
			ArbitrationHex: "0x162",
			Name:           "TCU11",
			PeriodMs:       10,
			Producer:       "Kefico CPTSH TCU",
			Receivers:      []string{"SIM2K-260 ECU", "Magna ATC AWD ECU"},
			DLC:            8,
			Description:    "Gear state, shift active status, fast inertia-phase torque reduction request (0x233D3C), and continuous transmission torque limit (0x1FD198 / 0x2096E4).",
			Signals: []CANSignalSpec{
				{
					Name:        "CUR_GR",
					StartBit:    0,
					BitLength:   4,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "gear",
					MinVal:      1,
					MaxVal:      8,
					Description: "Currently engaged transmission gear ratio (1..8).",
				},
				{
					Name:        "TAR_GR",
					StartBit:    4,
					BitLength:   4,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "gear",
					MinVal:      1,
					MaxVal:      8,
					Description: "Target destination gear ratio during active shift sequence.",
				},
				{
					Name:        "SHIFT_ACTIVE",
					StartBit:    8,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Shift in progress flag (1 during torque phase and inertia phase).",
				},
				{
					Name:        "SHIFT_PHASE",
					StartBit:    9,
					BitLength:   2,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      3,
					Description: "Shift execution phase: 0=None, 1=Torque Phase (clutch swap), 2=Inertia Phase (engine speed drop), 3=Torque Ramp.",
				},
				{
					Name:        "TQI_TCU",
					StartBit:    16,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "Nm",
					MinVal:      0.0,
					MaxVal:      2047.0,
					Description: "Fast inertia-phase torque setpoint derived from 0x233D3C (-10 stock vs -15 EKM). Triggers ECU spark retard via 0x25262C.",
				},
				{
					Name:        "ETL_TCU",
					StartBit:    32,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "Nm",
					MinVal:      0.0,
					MaxVal:      32767.0,
					Description: "Continuous transmission input torque protection ceiling from 0x1FD198 (350 Nm stock / 700 Nm EKM) and 0x2096E4 (400 Nm stock / 32767 EKM).",
				},
				{
					Name:        "ALIVE_CNT_TCU11",
					StartBit:    48,
					BitLength:   4,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "cnt",
					MinVal:      0,
					MaxVal:      15,
					Description: "Rolling transmission liveness counter (0..15).",
				},
				{
					Name:        "CRC_TCU11",
					StartBit:    56,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "crc",
					MinVal:      0,
					MaxVal:      255,
					Description: "Standard 8-bit Hyundai/Kia CAN frame checksum.",
				},
			},
		},
		{
			ArbitrationID:  0x164,
			ArbitrationHex: "0x164",
			Name:           "TCU12",
			PeriodMs:       10,
			Producer:       "Kefico CPTSH TCU",
			Receivers:      []string{"SIM2K-260 ECU", "Magna ATC AWD ECU", "Instrument Cluster"},
			DLC:            8,
			Description:    "Torque converter turbine speed (N_turb), converter slip ratio (nu), lockup clutch state, ATF temperature, and VCM solenoid floor (0x24BB22).",
			Signals: []CANSignalSpec{
				{
					Name:        "N_TURB",
					StartBit:    0,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.25,
					Offset:      0.0,
					Unit:        "RPM",
					MinVal:      0.0,
					MaxVal:      16383.75,
					Description: "Torque converter turbine output shaft angular speed.",
				},
				{
					Name:        "CONV_SLIP_RATIO",
					StartBit:    16,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.005,
					Offset:      0.0,
					Unit:        "ratio",
					MinVal:      0.0,
					MaxVal:      1.275,
					Description: "Torque converter speed ratio nu = N_turb / N_eng. Governs torque multiplication TR(nu) from 1.85 to 1.00.",
				},
				{
					Name:        "LUP_STAT",
					StartBit:    24,
					BitLength:   2,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      3,
					Description: "Torque converter lockup clutch state: 0=Open (stall mode), 1=Modulated Slip, 2=Locked.",
				},
				{
					Name:        "TEMP_AT",
					StartBit:    32,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      -40.0,
					Unit:        "deg C",
					MinVal:      -40.0,
					MaxVal:      215.0,
					Description: "Automatic transmission fluid (ATF) temperature sensor reading.",
				},
				{
					Name:        "VCM_SOL_VAL",
					StartBit:    40,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "mA",
					MinVal:      0.0,
					MaxVal:      2000.0,
					Description: "Variable clutch modulation (VCM) solenoid current floor from 0x24BB22 (850 mA stock vs 490 mA EKM).",
				},
				{
					Name:        "CRC_TCU12",
					StartBit:    56,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "crc",
					MinVal:      0,
					MaxVal:      255,
					Description: "Standard 8-bit Hyundai/Kia CAN frame checksum.",
				},
			},
		},
		{
			ArbitrationID:  0x220,
			ArbitrationHex: "0x220",
			Name:           "AWD11",
			PeriodMs:       20,
			Producer:       "Magna ATC AWD ECU",
			Receivers:      []string{"SIM2K-260 ECU", "Kefico CPTSH TCU", "Instrument Cluster"},
			DLC:            8,
			Description:    "Magna ATC wet-clutch coupling commanded vs actual torque (0..1000 Nm), front torque split %, launch preload status, and thermal derate state.",
			Signals: []CANSignalSpec{
				{
					Name:        "TQ_AWD_CMD",
					StartBit:    0,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.5,
					Offset:      0.0,
					Unit:        "Nm",
					MinVal:      0.0,
					MaxVal:      1000.0,
					Description: "Commanded AWD wet-clutch coupling torque target calculated by ATC algorithm.",
				},
				{
					Name:        "TQ_AWD_ACT",
					StartBit:    16,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.5,
					Offset:      0.0,
					Unit:        "Nm",
					MinVal:      0.0,
					MaxVal:      1000.0,
					Description: "Actual hydraulic clamping torque achieved, rate-limited by positive slew 0xC07650 (120 vs 180 Nm/20ms).",
				},
				{
					Name:        "AWD_FRONT_SPLIT",
					StartBit:    32,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.5,
					Offset:      0.0,
					Unit:        "%",
					MinVal:      0.0,
					MaxVal:      50.0,
					Description: "Front axle torque split percentage evaluated from 0xC0585A (45% Comfort) / 0xC05922 (40% Sport) and high-speed tapers.",
				},
				{
					Name:        "AWD_PRELOAD_ACT",
					StartBit:    40,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Standstill launch preload active flag (0xC04778 / 0xC04A3C).",
				},
				{
					Name:        "AWD_BRK_GATE_INHIBIT",
					StartBit:    41,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Brake pressure gate inhibit active flag (1 when P_brk > 64.5 psi / 4.45 bar in 0xC06080 stock binary).",
				},
				{
					Name:        "AWD_SLIP_CTRL_ACT",
					StartBit:    42,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Closed-loop wheel slip feedback intervention active (0xC061FA / 0xC06978).",
				},
				{
					Name:        "AWD_SPORT_MAP_ACT",
					StartBit:    43,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Sport feed-forward speed x pedal map active (0xC04BA0 Sport vs 0xC04B3C Comfort).",
				},
				{
					Name:        "AWD_THERM_STATE",
					StartBit:    48,
					BitLength:   3,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      7,
					Description: "AWD wet-clutch thermal state: 0=Normal, 1=Warm, 2=Derate (-30% clamp), 3=Cut (Thermal Protection).",
				},
				{
					Name:        "CRC_AWD",
					StartBit:    56,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "crc",
					MinVal:      0,
					MaxVal:      255,
					Description: "Standard 8-bit Hyundai/Kia CAN frame checksum.",
				},
			},
		},
		{
			ArbitrationID:  0x316,
			ArbitrationHex: "0x316",
			Name:           "EMS11",
			PeriodMs:       10,
			Producer:       "SIM2K-260 ECU",
			Receivers:      []string{"Kefico CPTSH TCU", "Magna ATC AWD ECU", "ESC/ABS", "Instrument Cluster"},
			DLC:            8,
			Description:    "Engine crankshaft speed in 0.25 RPM, indicated base combustion torque (0x255518), actual brake torque, and driver accelerator pedal %.",
			Signals: []CANSignalSpec{
				{
					Name:        "N_ENG",
					StartBit:    0,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.25,
					Offset:      0.0,
					Unit:        "RPM",
					MinVal:      0.0,
					MaxVal:      16383.75,
					Description: "Engine crankshaft rotational speed.",
				},
				{
					Name:        "TQI_ACT",
					StartBit:    16,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       4.0,
					Offset:      -256.0,
					Unit:        "Nm",
					MinVal:      -256.0,
					MaxVal:      764.0,
					Description: "Actual delivered crankshaft brake torque after spark efficiency retard (eta_iga) and parasitic drag.",
				},
				{
					Name:        "TQI_BAS",
					StartBit:    24,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       4.0,
					Offset:      -256.0,
					Unit:        "Nm",
					MinVal:      -256.0,
					MaxVal:      764.0,
					Description: "Indicated base combustion torque from forward torque model 0x255518. Inflated in Sleepy Loud bin to 967.8 Nm.",
				},
				{
					Name:        "PEDAL_PCT",
					StartBit:    32,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.392157,
					Offset:      0.0,
					Unit:        "%",
					MinVal:      0.0,
					MaxVal:      100.0,
					Description: "Driver accelerator pedal position (PV_AV_CAN).",
				},
				{
					Name:        "TQI_DRV",
					StartBit:    40,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       4.0,
					Offset:      -256.0,
					Unit:        "Nm",
					MinVal:      -256.0,
					MaxVal:      764.0,
					Description: "Driver demand torque from 0x214E80 (Sport) or 0x214280 (Comfort) before speed/gear limiters.",
				},
				{
					Name:        "CF_Ems_EngStat",
					StartBit:    48,
					BitLength:   4,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      15,
					Description: "Engine running status: 0=Stopped, 1=Cranking, 2=Running, 3=Stall.",
				},
				{
					Name:        "CRC_EMS11",
					StartBit:    56,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "crc",
					MinVal:      0,
					MaxVal:      255,
					Description: "Standard 8-bit Hyundai/Kia CAN frame checksum.",
				},
			},
		},
		{
			ArbitrationID:  0x329,
			ArbitrationHex: "0x329",
			Name:           "EMS12",
			PeriodMs:       10,
			Producer:       "SIM2K-260 ECU",
			Receivers:      []string{"Kefico CPTSH TCU", "Magna ATC AWD ECU", "Instrument Cluster"},
			DLC:            8,
			Description:    "Engine coolant temp (TCO), intake air temp (TIA), manifold boost pressure (PUT), shift spark floor clamp status (0x207AD9), and TCHA overspeed cut status (0x203EC6).",
			Signals: []CANSignalSpec{
				{
					Name:        "TCO",
					StartBit:    0,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.75,
					Offset:      -48.0,
					Unit:        "deg C",
					MinVal:      -48.0,
					MaxVal:      143.25,
					Description: "Engine coolant temperature sensor reading.",
				},
				{
					Name:        "TIA",
					StartBit:    8,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.75,
					Offset:      -48.0,
					Unit:        "deg C",
					MinVal:      -48.0,
					MaxVal:      143.25,
					Description: "Intake manifold charge air temperature.",
				},
				{
					Name:        "PUT_MAP_HPA",
					StartBit:    16,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "hPa",
					MinVal:      0.0,
					MaxVal:      4000.0,
					Description: "Upstream throttle / manifold absolute pressure (PUT) in hPa absolute.",
				},
				{
					Name:        "BRAKE_ACT",
					StartBit:    32,
					BitLength:   2,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      3,
					Description: "Brake pedal switch engagement: bit 0=Switch, bit 1=Lamp.",
				},
				{
					Name:        "CF_Ems_SpkFloorClp",
					StartBit:    34,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Shift spark floor clamp status: 1 if ignition timing is blocked by 0x207AD9 during TCU torque phase/inertia phase cut.",
				},
				{
					Name:        "TCHA_CUT_STAT",
					StartBit:    35,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Turbocharger overspeed protection cut active flag (tripped by 0x203EC6 debounce in Sleepy Loud bin).",
				},
				{
					Name:        "TPS",
					StartBit:    40,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.392157,
					Offset:      0.0,
					Unit:        "%",
					MinVal:      0.0,
					MaxVal:      100.0,
					Description: "Throttle valve blade position percentage.",
				},
				{
					Name:        "CRC_EMS12",
					StartBit:    56,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "crc",
					MinVal:      0,
					MaxVal:      255,
					Description: "Standard 8-bit Hyundai/Kia CAN frame checksum.",
				},
			},
		},
		{
			ArbitrationID:  0x386,
			ArbitrationHex: "0x386",
			Name:           "WHL_SPD11",
			PeriodMs:       20,
			Producer:       "ESC/ABS",
			Receivers:      []string{"SIM2K-260 ECU", "Kefico CPTSH TCU", "Magna ATC AWD ECU"},
			DLC:            8,
			Description:    "High-resolution 16-bit LE individual wheel speeds (FL, FR, RL, RR) in 0.03125 km/h. Feeds TCS 0x153 and Magna ATC 0xC04214 / 0xC061FA slip comparator.",
			Signals: []CANSignalSpec{
				{
					Name:        "WHL_SPD_FL",
					StartBit:    0,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.03125,
					Offset:      0.0,
					Unit:        "km/h",
					MinVal:      0.0,
					MaxVal:      2047.96875,
					Description: "Front-Left wheel speed measured by active hall-effect sensor (1/32 km/h resolution).",
				},
				{
					Name:        "WHL_SPD_FR",
					StartBit:    16,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.03125,
					Offset:      0.0,
					Unit:        "km/h",
					MinVal:      0.0,
					MaxVal:      2047.96875,
					Description: "Front-Right wheel speed measured by active hall-effect sensor.",
				},
				{
					Name:        "WHL_SPD_RL",
					StartBit:    32,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.03125,
					Offset:      0.0,
					Unit:        "km/h",
					MinVal:      0.0,
					MaxVal:      2047.96875,
					Description: "Rear-Left wheel speed measured by active hall-effect sensor.",
				},
				{
					Name:        "WHL_SPD_RR",
					StartBit:    48,
					BitLength:   16,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       0.03125,
					Offset:      0.0,
					Unit:        "km/h",
					MinVal:      0.0,
					MaxVal:      2047.96875,
					Description: "Rear-Right wheel speed measured by active hall-effect sensor.",
				},
			},
		},
		{
			ArbitrationID:  0x4A0,
			ArbitrationHex: "0x4A0",
			Name:           "CLU11",
			PeriodMs:       20,
			Producer:       "Instrument Cluster / DMS",
			Receivers:      []string{"SIM2K-260 ECU", "Kefico CPTSH TCU", "Magna ATC AWD ECU", "ECS Suspension"},
			DLC:            8,
			Description:    "Drive Mode switch selection (Comfort/Eco/Sport/Smart/Custom), ECU 0x200CCE override status, ESC mode button state, and Launch Control status.",
			Signals: []CANSignalSpec{
				{
					Name:        "DRV_MODE_CMD",
					StartBit:    0,
					BitLength:   4,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      15,
					Description: "Drive Mode selector switch position: 0=Comfort, 1=Sport, 2=Eco, 3=Smart, 4=Custom/Mode 4.",
				},
				{
					Name:        "ECU_MODE_OVERRIDE",
					StartBit:    4,
					BitLength:   4,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      15,
					Description: "ECU internal drive mode force switch 0x200CCE: 0=Follow CAN 0x4A0, 1=Force Sport Mode 8, 2=Force Mode 4.",
				},
				{
					Name:        "ESC_MODE_STAT",
					StartBit:    8,
					BitLength:   2,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "enum",
					MinVal:      0,
					MaxVal:      3,
					Description: "Electronic Stability Control button state: 0=Full ON, 1=ESC Sport (partial off), 2=ESC Full OFF.",
				},
				{
					Name:        "LAUNCH_CTRL_STAT",
					StartBit:    10,
					BitLength:   1,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "bool",
					MinVal:      0,
					MaxVal:      1,
					Description: "Launch Control active / pre-stage staging flag (1 during pre-stage brake-stand).",
				},
				{
					Name:        "CRC_CLU",
					StartBit:    56,
					BitLength:   8,
					ByteOrder:   "little_endian",
					IsSigned:    false,
					Scale:       1.0,
					Offset:      0.0,
					Unit:        "crc",
					MinVal:      0,
					MaxVal:      255,
					Description: "Standard 8-bit Hyundai/Kia CAN frame checksum.",
				},
			},
		},
	}
}

// calcHyundaiChecksum computes a standard Hyundai/Kia C-CAN 8-bit modular checksum across bytes 0..6.
func calcHyundaiChecksum(arbID uint32, data *[8]byte) uint8 {
	var sum uint8 = uint8(arbID & 0xFF) + uint8((arbID>>8)&0xFF)
	for i := 0; i < 7; i++ {
		sum += data[i]
	}
	return (sum ^ 0xFF) & 0xFF
}

// EncodeTelemetryToCCANFrames encodes a 100 Hz TelemetrySample into exact 8-byte C-CAN frames
// for all 9 arbitration IDs (0x080, 0x153, 0x162, 0x164, 0x220, 0x316, 0x329, 0x386, 0x4A0),
// generating valid candump log lines and signal verification dictionaries.
func EncodeTelemetryToCCANFrames(s TelemetrySample, cal ExtractedCalSummary, driveMode string) []EncodedCANFrame {
	frames := make([]EncodedCANFrame, 0, 9)

	// Rolling counter based on time (10ms steps)
	stepCount := int(math.Round(s.TimeSec * 100.0))
	if stepCount < 0 {
		stepCount = -stepCount
	}
	aliveCnt := uint8(stepCount & 0x0F)

	// -------------------------------------------------------------
	// 1. 0x080 ESP12 (20ms, ESC/ABS -> ECU, TCU, AWD)
	// -------------------------------------------------------------
	{
		var d [8]byte
		accelMs2 := s.AccelG * 9.80665
		longRaw := uint16(math.Max(0, math.Min(2047, (accelMs2+10.23)/0.01)))
		latRaw := uint16(1023) // 0.0 m/s^2 in straight line
		yawRaw := uint16(2047) // 0.0 deg/s in straight line

		brakeBar := s.BrakePsi * 0.0689476
		cylPresRaw := uint16(math.Max(0, math.Min(4095, brakeBar/0.1)))

		brkSw := uint8(0)
		if s.BrakePsi > 10.0 {
			brkSw = 3 // Both circuits active
		}

		// Byte 0: LAT_ACCEL[7:0]
		d[0] = byte(latRaw & 0xFF)
		// Byte 1: LAT_ACCEL[10:8] (bits 0..2) | (LONG_ACCEL[4:0] << 3)
		d[1] = byte((latRaw>>8)&0x07) | byte((longRaw&0x1F)<<3)
		// Byte 2: LONG_ACCEL[10:5] (bits 0..5) | (BRK_SW << 6)
		d[2] = byte((longRaw>>5)&0x3F) | byte((brkSw&0x03)<<6)
		// Byte 3: CYL_PRES[7:0]
		d[3] = byte(cylPresRaw & 0xFF)
		// Byte 4: CYL_PRES[11:8] (bits 0..3) | (YAW_RATE[3:0] << 4)
		d[4] = byte((cylPresRaw>>8)&0x0F) | byte((yawRaw&0x0F)<<4)
		// Byte 5: YAW_RATE[11:4]
		d[5] = byte((yawRaw >> 4) & 0xFF)
		// Byte 6: ALIVE_CNT
		d[6] = aliveCnt
		// Byte 7: Checksum
		d[7] = calcHyundaiChecksum(0x080, &d)

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x080,
			ArbitrationHex: "0x080",
			FrameName:      "ESP12",
			Producer:       "ESC/ABS",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 080#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"LONG_ACCEL_MS2":  accelMs2,
				"BRAKE_BAR":       brakeBar,
				"CYL_PRES_RAW":    cylPresRaw,
				"BRAKE_PSI":       s.BrakePsi,
				"GATE_ACTIVE_64P": brakeBar > 4.45,
			},
		})
	}

	// -------------------------------------------------------------
	// 2. 0x153 TCS11 (10ms, ESC/ABS -> SIM2K-260 ECU, TCU)
	// -------------------------------------------------------------
	{
		var d [8]byte
		tcsReq := byte(0)
		tcsFastReq := byte(0)
		tqiAsrRaw := byte(0xFF) // No cut by default

		if s.TCSActive {
			tcsReq = 1
			tcsFastReq = 0x80
			// Calculate torque cut raw value (scale 4 Nm, offset -256)
			// When active, request torque cut below actual engine torque
			cutNm := math.Max(50.0, s.ActualEngineNm*0.5)
			tqiAsrRaw = byte(math.Max(0, math.Min(255, (cutNm+256.0)/4.0)))
		}

		d[0] = tcsReq // bit 0 = TCS_REQ
		d[1] = tcsFastReq
		d[2] = 0x00
		d[3] = tqiAsrRaw // TQI_ASR_FAST
		d[4] = 0x40       // TQI_MSR (0 Nm offset -256)
		d[5] = tqiAsrRaw // TQI_ASR_SLOW
		d[6] = aliveCnt
		d[7] = calcHyundaiChecksum(0x153, &d)

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x153,
			ArbitrationHex: "0x153",
			FrameName:      "TCS11",
			Producer:       "ESC/ABS",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 153#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"TCS_ACTIVE":       s.TCSActive,
				"TCS_REQ_BIT":      tcsReq,
				"TQI_ASR_RAW":      tqiAsrRaw,
				"ECU_TCS_INHIBIT":  cal.ECUTCSInhibit,
				"INHIBITED_BY_ECU": cal.ECUTCSInhibit == 1,
			},
		})
	}

	// -------------------------------------------------------------
	// 3. 0x162 TCU11 (10ms, Kefico CPTSH TCU -> SIM2K-260 ECU, AWD)
	// -------------------------------------------------------------
	{
		var d [8]byte
		curGear := byte(math.Max(1, math.Min(8, float64(s.Gear))))
		tarGear := curGear
		shiftBit := byte(0)
		shiftPhase := byte(0)

		if s.ShiftActive {
			shiftBit = 1
			shiftPhase = 2 // Inertia phase
			if curGear < 8 {
				tarGear = curGear + 1
			}
		}

		d[0] = (curGear & 0x0F) | ((tarGear & 0x0F) << 4)
		d[1] = (shiftBit & 0x01) | ((shiftPhase & 0x03) << 1)

		// Fast inertia-phase torque request TQI_TCU from 0x233D3C
		tcuTqReq := uint16(math.Max(0, math.Min(2047, s.TCUTorqueLimitNm)))
		if !s.ShiftActive {
			tcuTqReq = 2047 // No cut active in-gear
		}
		binary.LittleEndian.PutUint16(d[2:4], tcuTqReq)

		// Continuous transmission torque protection cap ETL_TCU (0x1FD198 / 0x2096E4)
		contCap := uint16(math.Max(0, math.Min(32767, cal.TCUPrimaryTorqueCapNm)))
		if contCap == 0 {
			contCap = 350 // Stock fallback
		}
		binary.LittleEndian.PutUint16(d[4:6], contCap)

		d[6] = aliveCnt
		d[7] = calcHyundaiChecksum(0x162, &d)

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x162,
			ArbitrationHex: "0x162",
			FrameName:      "TCU11",
			Producer:       "Kefico CPTSH TCU",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 162#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"CUR_GEAR":       curGear,
				"TAR_GEAR":       tarGear,
				"SHIFT_ACTIVE":   s.ShiftActive,
				"TQI_TCU_REQ_NM": tcuTqReq,
				"ETL_TCU_CAP_NM": contCap,
				"TCU_BIN":        cal.TCUBinName,
			},
		})
	}

	// -------------------------------------------------------------
	// 4. 0x164 TCU12 (10ms, Kefico CPTSH TCU -> SIM2K-260 ECU, AWD, Cluster)
	// -------------------------------------------------------------
	{
		var d [8]byte
		turbRaw := uint16(math.Max(0, math.Min(65535, s.TurbineRPM*4.0)))
		binary.LittleEndian.PutUint16(d[0:2], turbRaw)

		// Converter speed ratio nu = N_turb / N_eng
		nu := 0.0
		if s.EngineRPM > 50.0 {
			nu = math.Max(0.0, math.Min(1.25, s.TurbineRPM/s.EngineRPM))
		}
		d[2] = byte(math.Max(0, math.Min(255, nu/0.005)))

		// Lockup clutch state
		lupStat := byte(0) // Open by default
		if s.Gear >= 2 && nu > 0.95 {
			lupStat = 2 // Locked
		} else if s.Gear >= 1 && nu > 0.80 {
			lupStat = 1 // Slipping
		}
		d[3] = lupStat

		// ATF fluid temp: 85 C nominal -> raw = 85 - (-40) = 125 = 0x7D
		d[4] = byte(125)

		// VCM Solenoid floor current from 0x24BB22 (850 mA stock vs 490 mA EKM)
		vcmMa := uint16(cal.TCUVCMSolenoidFloorMa)
		if vcmMa == 0 {
			vcmMa = 850 // Stock fallback
		}
		binary.LittleEndian.PutUint16(d[5:7], vcmMa)

		d[7] = calcHyundaiChecksum(0x164, &d)

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x164,
			ArbitrationHex: "0x164",
			FrameName:      "TCU12",
			Producer:       "Kefico CPTSH TCU",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 164#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"TURBINE_RPM":   s.TurbineRPM,
				"SLIP_RATIO_NU": nu,
				"LUP_STATE":     lupStat,
				"ATF_TEMP_C":    85.0,
				"VCM_FLOOR_MA":  vcmMa,
			},
		})
	}

	// -------------------------------------------------------------
	// 5. 0x220 AWD11 (20ms, Magna ATC AWD ECU -> ECU, TCU, Cluster)
	// -------------------------------------------------------------
	{
		var d [8]byte
		cmdRaw := uint16(math.Max(0, math.Min(2000, s.AWDTargetNm*2.0)))
		actRaw := uint16(math.Max(0, math.Min(2000, s.AWDActualNm*2.0)))
		binary.LittleEndian.PutUint16(d[0:2], cmdRaw)
		binary.LittleEndian.PutUint16(d[2:4], actRaw)

		// Front Split % (scale 0.5)
		d[4] = byte(math.Max(0, math.Min(100, s.AWDFrontSplitPct*2.0)))

		// Status flags
		flags := byte(0)
		if s.TimeSec < 0.0 || (s.Gear <= 2 && s.SpeedKmh < 30.0) {
			flags |= 0x01 // AWD_PRELOAD_ACT
		}
		if s.AWDBrakeGated {
			flags |= 0x02 // AWD_BRK_GATE_INHIBIT (0xC07498 / 0xC07606)
		}
		if math.Abs(s.RearWheelKmh-s.FrontWheelKmh) > 1.5 {
			flags |= 0x04 // AWD_SLIP_CTRL_ACT (0xC061FA)
		}
		if driveMode == "sport" || driveMode == "custom_m4" {
			flags |= 0x08 // AWD_SPORT_MAP_ACT (0xC04BA0)
		}
		d[5] = flags

		// Thermal state (0=Normal) and alive counter
		d[6] = (aliveCnt << 4) // bits 4..7 alive counter, bits 0..2 thermal state 0
		d[7] = calcHyundaiChecksum(0x220, &d)

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x220,
			ArbitrationHex: "0x220",
			FrameName:      "AWD11",
			Producer:       "Magna ATC AWD ECU",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 220#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"TQ_AWD_CMD_NM":       s.AWDTargetNm,
				"TQ_AWD_ACT_NM":       s.AWDActualNm,
				"FRONT_SPLIT_PCT":     s.AWDFrontSplitPct,
				"BRAKE_GATED":         s.AWDBrakeGated,
				"PRELOAD_ACTIVE":      (flags & 0x01) != 0,
				"GATE_BYPASS_IN_CAL":  cal.AWDBrakeGateBypassed,
				"SLEW_RATE_NM_20MS":   cal.AWDSlewRateNmPer20ms,
			},
		})
	}

	// -------------------------------------------------------------
	// 6. 0x316 EMS11 (10ms, SIM2K-260 ECU -> TCU, AWD, ESC, Cluster)
	// -------------------------------------------------------------
	{
		var d [8]byte
		rpmRaw := uint16(math.Max(0, math.Min(65535, s.EngineRPM*4.0)))
		binary.LittleEndian.PutUint16(d[0:2], rpmRaw)

		// Actual combustion torque (scale 4, offset -256 -> (Nm/4) + 64)
		d[2] = byte(math.Max(0, math.Min(255, (s.ActualEngineNm/4.0)+64.0)))

		// Indicated base torque TQI_BAS from forward torque model 0x255518
		d[3] = byte(math.Max(0, math.Min(255, (s.UncappedTorqueNm/4.0)+64.0)))

		// Accelerator pedal %
		d[4] = byte(math.Max(0, math.Min(255, s.PedalPct*2.54)))

		// Driver demand torque TQI_DRV from 0x214E80
		d[5] = byte(math.Max(0, math.Min(255, (s.DriverDemandNm/4.0)+64.0)))

		// Engine status: 2 = Running, upper nibble = alive counter
		d[6] = 0x02 | (aliveCnt << 4)
		d[7] = calcHyundaiChecksum(0x316, &d)

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x316,
			ArbitrationHex: "0x316",
			FrameName:      "EMS11",
			Producer:       "SIM2K-260 ECU",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 316#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"ENGINE_RPM":       s.EngineRPM,
				"TQI_ACT_NM":       s.ActualEngineNm,
				"TQI_BAS_NM":       s.UncappedTorqueNm,
				"TQI_DRV_NM":       s.DriverDemandNm,
				"PEDAL_PCT":        s.PedalPct,
				"AIRLOAD_MG":       s.AirloadMgStk,
			},
		})
	}

	// -------------------------------------------------------------
	// 7. 0x329 EMS12 (10ms, SIM2K-260 ECU -> TCU, AWD, Cluster)
	// -------------------------------------------------------------
	{
		var d [8]byte
		// Coolant temp TCO: 85 C -> raw = (85 + 48) / 0.75 = 177 = 0xB1
		d[0] = byte(math.Max(0, math.Min(255, (85.0+48.0)/0.75)))

		// Intake air temp TIA: 25 C ambient -> raw = (25 + 48) / 0.75 = 97 = 0x61
		d[1] = byte(math.Max(0, math.Min(255, (25.0+48.0)/0.75)))

		// Manifold boost pressure PUT in hPa
		putHpa := uint16(math.Max(0, math.Min(4000, s.BoostHpa)))
		binary.LittleEndian.PutUint16(d[2:4], putHpa)

		// Bit flags:
		// bit 0: Brake switch
		// bit 2: CF_Ems_SpkFloorClp (0x207AD9 shift spark floor clamped)
		// bit 3: TCHA_CUT_STAT (0x203EC6 turbo overspeed cut active)
		statusByte := byte(0)
		if s.BrakePsi > 10.0 {
			statusByte |= 0x01
		}
		sparkFloorClamped := s.ShiftActive && (s.SparkAdvanceDeg <= s.SparkFloorDeg+0.05)
		if sparkFloorClamped {
			statusByte |= 0x04
		}
		if s.TCHACutActive {
			statusByte |= 0x08
		}
		d[4] = statusByte

		// Throttle position TPS
		d[5] = byte(math.Max(0, math.Min(255, s.PedalPct*2.54)))

		d[6] = aliveCnt
		d[7] = calcHyundaiChecksum(0x329, &d)

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x329,
			ArbitrationHex: "0x329",
			FrameName:      "EMS12",
			Producer:       "SIM2K-260 ECU",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 329#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"BOOST_HPA":            s.BoostHpa,
				"BOOST_PSI":            s.BoostPsi,
				"SPARK_ADVANCE_DEG":    s.SparkAdvanceDeg,
				"SPARK_FLOOR_DEG":      s.SparkFloorDeg,
				"SPARK_FLOOR_CLAMPED":  sparkFloorClamped,
				"TCHA_CUT_ACTIVE":      s.TCHACutActive,
				"CAL_SHIFT_FLOOR_DEG":  cal.ECUShiftSparkFloorDeg,
				"CAL_TCHA_ACTIVE":      cal.ECUTCHALimitActive,
			},
		})
	}

	// -------------------------------------------------------------
	// 8. 0x386 WHL_SPD11 (20ms, ESC/ABS -> SIM2K-260 ECU, TCU, AWD)
	// -------------------------------------------------------------
	{
		var d [8]byte
		wfRaw := uint16(math.Max(0, math.Min(65535, s.FrontWheelKmh*32.0)))
		wrRaw := uint16(math.Max(0, math.Min(65535, s.RearWheelKmh*32.0)))

		binary.LittleEndian.PutUint16(d[0:2], wfRaw) // FL
		binary.LittleEndian.PutUint16(d[2:4], wfRaw) // FR
		binary.LittleEndian.PutUint16(d[4:6], wrRaw) // RL
		binary.LittleEndian.PutUint16(d[6:8], wrRaw) // RR

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x386,
			ArbitrationHex: "0x386",
			FrameName:      "WHL_SPD11",
			Producer:       "ESC/ABS",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 386#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"WHL_SPD_FRONT_KMH": s.FrontWheelKmh,
				"WHL_SPD_REAR_KMH":  s.RearWheelKmh,
				"FRONT_SLIP_PCT":    s.FrontSlipPct,
				"REAR_SLIP_PCT":     s.RearSlipPct,
				"DELTA_WHEEL_KMH":   math.Abs(s.RearWheelKmh - s.FrontWheelKmh),
				"AWD_NOMINAL_R_MM":  cal.AWDTireRadiusMm,
			},
		})
	}

	// -------------------------------------------------------------
	// 9. 0x4A0 CLU11 (20ms, Instrument Cluster / DMS -> All Nodes)
	// -------------------------------------------------------------
	{
		var d [8]byte
		modeEnum := byte(1) // Sport default
		switch driveMode {
		case "comfort":
			modeEnum = 0
		case "sport":
			modeEnum = 1
		case "eco":
			modeEnum = 2
		case "smart":
			modeEnum = 3
		case "custom_m4":
			modeEnum = 4
		}

		ecuOverride := byte(cal.ECUDriveModeForce) // 0x200CCE

		d[0] = (modeEnum & 0x0F) | ((ecuOverride & 0x0F) << 4)

		// ESC State and Launch Control Ready
		escStat := byte(0) // ESC On
		launchReady := byte(0)
		if s.TimeSec < 0.0 && s.BrakePsi > 400.0 && s.PedalPct > 80.0 {
			launchReady = 1
		}
		d[1] = (escStat & 0x03) | ((launchReady & 0x01) << 2)

		d[6] = aliveCnt
		d[7] = calcHyundaiChecksum(0x4A0, &d)

		dataHex := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
		frames = append(frames, EncodedCANFrame{
			TimestampSec:   s.TimeSec,
			ArbitrationID:  0x4A0,
			ArbitrationHex: "0x4A0",
			FrameName:      "CLU11",
			Producer:       "Instrument Cluster / DMS",
			DLC:            8,
			Data:           d,
			DataHex:        dataHex,
			CandumpLine:    fmt.Sprintf("(%09.4f) can0 4A0#%s", s.TimeSec, dataHex),
			DecodedSignals: map[string]interface{}{
				"DRV_MODE_CMD":      modeEnum,
				"DRV_MODE_STR":      driveMode,
				"ECU_MODE_OVERRIDE": ecuOverride,
				"LAUNCH_CTRL_READY": launchReady == 1,
			},
		})
	}

	return frames
}
