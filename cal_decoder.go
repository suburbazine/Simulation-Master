package main

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// ECUProfile holds all decoded SIM2K-260 calibration tables and scalars for fast physics evaluation.
type ECUProfile struct {
	BinPath string
	BinName string

	DriveModeForce int // 0x200CCE (0=CAN, 1=Force Sport Mode 8, 2=Force Mode 4)

	// Driver Demand (12 RPM x 14 Pedal -> Nm)
	DrvRPM        []float64   // 12 pts
	DrvPedal      []float64   // 14 pts (0..100%)
	DrvMapComfort [][]float64 // 0x214280 (12x14 Nm)
	DrvMapMode4   [][]float64 // 0x214760 (12x14 Nm)
	DrvMapSport   [][]float64 // 0x214E80 (12x14 Nm)

	// Gear & Speed torque limit factors (Q15 -> 0..1)
	GearAxis      []float64   // 10 pts
	GearPedalAxis []float64   // 8 pts
	GearFactorMap [][]float64 // 0x213B70 (10x8)
	SpdAxis       []float64   // 12 pts (km/h)
	SpdPedalAxis  []float64   // 8 pts
	SpdFactorMap  [][]float64 // 0x215150 (12x8)

	// Forward Torque Model (18 RPM x 20 Airload mg/stk -> Nm)
	FwdRPM      []float64   // 18 pts (550..6600)
	FwdAir      []float64   // 20 pts (50..1400 mg/stk)
	TQIBasA     [][]float64 // 0x255518 (18x20 Nm)
	TQIBasB     [][]float64 // 0x2557E8 (18x20 Nm)
	TQIMaxEnv   [][]float64 // Envelope across 8 cam-state tables 0x253D14.. (18x20 Nm)
	TQIMaxPeak  float64
	TQIBasPeak  float64
	DrvPeakNm   float64

	// Inverse Torque Model (18 RPM x 20 Torque Nm -> Airload mg/stk)
	InvRPM     []float64   // 18 pts
	InvTq      []float64   // 20 pts (20..750 Nm)
	MAFSpHigh  [][]float64 // 0x25965C (18x20 mg/stk)

	// Maximum Cylinder Air Charge & Boost Setpoint Chain (0x8016AC20 -> 0x80136F84 -> 0x800F3CB8 -> 0x800F5DA4)
	MaxCylAirRPM      []float64   // 0x23D5DE (12 RPM pts: 1300..6700)
	MaxCylAirMg       []float64   // 0x23D810 M_AIR_CYL_MAX (12 pts mg/stk: 1110..1020 Stock/BEF, 1400 Q, 1500 Loud/RAL, 1550 v103)
	PutSpRPM          []float64   // 0x203988 (6 RPM pts: 2000..6500)
	PutSpCurveHpa     []float64   // 0x21DA7C top col (6 pts hPa: 2155 Q/BEF, 2400 Loud/RAL/v103, 2300..2619 KJT)
	PutSpTablePeakHpa float64     // Peak of 0x21DA7C top col
	CompPQRPM         []float64   // 0x21EAC6 (8 RPM pts: 2000..6500)
	CompPQGrid        [][]float64 // 0x21EB38 IP_PQ_CHA_MAX (8 IAT x 8 RPM compressor pressure ratio limit)
	CompPQWarmCurve   []float64   // 0x21EB38 row 5 (30C IAT warm pressure ratio curve)
	CompPQMax         float64     // Peak of 0x21EB38

	// Basic Lambda Setpoint & Torque Protection Cap
	LambdaSetptMin float64 // 0x24CE7C (16x16 Q14 basic lambda setpoint min at WOT)
	ClutchLimitNm  float64 // Effective ECU torque ceiling (0x253D14 TQIMaxPeak)

	// Boost & TCHA Turbo Speed Limiter
	PutSpMaxHpa       float64 // Effective WOT MAP/PUT peak target (hPa) from SIM2K-260 air-path chain
	BoostLimitAHpa    float64 // 0x21F478 CAP_H overboost diagnosis ceiling (hPa)
	TCHALimitActive   bool    // 0x203EC6 != 65535
	TCHADebounceCyc   int     // 0x203EC6 (10 cycles in Loud vs 65535 in Q)
	TCHAMinTorqueNm   float64 // 0x203ED4 (200 Nm in Loud)
	TCHARampNmPerCyc  float64 // 0x203ED8 (17 Nm/cycle in Loud)
	TCHAMaxTorqueCap  float64 // 0x203E26 (750 Nm in Loud vs 2048 Nm in Q)
	TCHASpeedFactor   float64 // 0x203ECC (49152 Loud vs 52428 Q)
	TCHAInhibitSwitch int     // 0x204149

	// Ignition Timing & Efficiency Model
	IgnRPM            []float64   // 18 pts (550..6600)
	IgnLoad           []float64   // 20 pts (50..1400 mg/stk)
	IgnBaseLow        [][]float64 // 0x236D50 (18x20 deg)
	IgnBaseHigh       [][]float64 // 0x238090 (18x20 deg, active at WOT)
	IgnOptHigh        [][]float64 // 0x238874 (18x20 deg, MBT optimum)
	IGAMinCoolant     []float64   // 16 pts (-33..100 C)
	IGAMinLoad        []float64   // 16 pts (49..1203 mg/stk)
	IGAMinHigh        [][]float64 // 0x239AF4 (16x16 deg)
	IGAMinWarmWOTDeg  float64
	ShiftSparkFloor   float64     // 0x207AD9 (-3.0 Q, -6.0 v103, +12.0 KJT)
	EffToRetardAxis   []float64   // 0x25743E (16 efficiency pts 0.09..1.00)
	EffToRetardDeg    []float64   // 0x25262C (16 retard deg pts 55.1..0.0)
	RetardToEffAxis   []float64   // 0x2553AA (16 retard deg pts 0.0..55.1)
	RetardToEffVal    []float64   // 0x2524EC (16 efficiency pts 1.00..0.09)

	// Traction Control & Antilag
	TCSInhibit       int  // 0x200B4B (0=Stock TCS, 1=TCS inhibited)
	TCSMinRPM        float64
	RALPatchDetected bool // 0x3DC000 (0x808DC000) non-FF code caves
}

// TCUProfile holds all decoded Kefico CPTSH 8AT calibration tables and scalars.
type TCUProfile struct {
	BinPath string
	BinName string

	PedalAxis20       []float64   // 20 pts (0..100%)
	ComfortGrid       [][]float64 // Slot 3 (20 pedal x 14 shift cols in km/h)
	EcoGrid           [][]float64 // Slot 9
	SmartGrid         [][]float64 // Slot 17
	SportGrid         [][]float64 // Slot 23
	ComfortWOTUpKmh   []float64   // 7 upshifts (1->2 .. 7->8)
	SportWOTUpKmh     []float64   // 7 upshifts (1->2 .. 7->8)

	LaunchStallRPM    float64   // 0x238B68 (2500 RPM stock vs 3500 RPM EKM)
	PrimaryTqCapNm    float64   // 0x1FD198 (350 Nm stock vs 700 Nm EKM)
	PerGearCapNm      []float64 // 0x1FD1B0 (8 gears: 350 stock vs 700 EKM)
	GlobalTqCapNm     float64   // 0x2096E4 (400 Nm stock vs 32767 EKM)
	ClutchLimitPeakNm float64   // 0x1FBC5C (485 Nm stock vs 600 Nm EKM)

	ShiftCutRaw       int // 0x233D3C (-10 stock vs -15 EKM)
	ShiftTimerMs      int // 0x20FC52 (65535 stock vs 90 ms EKM)
	VCMSolenoidFloor  int // 0x24BB22 (850 mA stock vs 490 mA EKM)
}

// AWDProfile holds all decoded Magna ATC AWD (95447-4J105) calibration tables and scalars.
type AWDProfile struct {
	BinPath string
	BinName string

	TireRadiusMm       float64     // 0xC04214 (320 mm)
	BaseClampNm        float64     // 0xC04334 (150 Nm)
	LaunchPedalAxis    []float64   // [10, 20, 30, 50, 80]%
	LaunchPreloadEco   [][]float64 // 0xC046D8 (16 gears x 5 pedal Nm)
	LaunchPreloadSport [][]float64 // 0xC04778 (16 gears x 5 pedal Nm)
	OffIdleNormalNm    []float64   // 0xC04A3C row 1 (4 pts Nm)
	OffIdleEcoNm       []float64   // 0xC04ABC row 1 (4 pts Nm)

	FFComfortSpeedKmh  []float64   // 0xC0667E (10 pts: 0..90 km/h)
	FFSportSpeedKmh    []float64   // 0xC06692 (10 pts: 0..75 km/h)
	FFPedalAxis        []float64   // [0, 20, 40, 60, 100]%
	FFComfortMapNm     [][]float64 // 0xC04B3C (10 speed x 5 pedal Nm)
	FFSportMapNm       [][]float64 // 0xC04BA0 (10 speed x 5 pedal Nm)

	SplitSpeedAxisKmh  []float64   // [0, 30, 60, 100, 160] km/h
	SplitPedalAxis     []float64   // [0, 25, 50, 75, 100]%
	SplitComfortPct    [][]float64 // 0xC0585A (5x5 %)
	SplitMod38Pct      [][]float64 // 0xC058F0 (5x5 %)
	SplitSportPct      [][]float64 // 0xC05922 (5x5 %)

	CeilingComfortNm   float64 // 0xC059EA peak (850 Nm)
	CeilingSportNm     float64 // 0xC05A1C peak (1000 Nm)

	BrakeGateBar       float64 // 0xC07498 (4.4 bar = 63.8 psi)
	BrakeGateQ88Bar    float64 // 0xC07606 (4.45 bar = 64.5 psi)
	BrakeGateBypassed  bool    // True if 0xC06080 zeroed & 0xC060B0 raised

	SlewRateNmPer20ms  float64 // 0xC07650 (120 Nm/20ms stock vs 180 Nm/20ms mod)
	SlewRateNmPerSec   float64 // 6000 Nm/s stock vs 9000 Nm/s mod

	HighSpdGate1Kmh    float64   // 0xC066F6 (75 km/h cruise cutoff)
	HighSpdGate2Axis   []float64 // 0xC06F96 ([0, 20, 100, 140, 160] km/h)
	HighSpdGate2Split  []float64 // 0xC056CA WOT col ([30, 25, 5, 2, 0]%)
	MasterTaperAxisKmh []float64 // 0xC06252 (16 pts 0..250 km/h)
	MasterTaperPct     []float64 // 0xC0640A (16 pts 45..25%)
	SlipDeadbandKmh    float64   // 0xC06978 (0.5 km/h low speed, 2.0 km/h high speed)
	SlipCurveDeltaKmh  []float64 // 11 pts (0..20 km/h wheel slip delta)
	SlipCurveTorqueNm  []float64 // 0xC061FA (11 pts 47.5..444.0 Nm)
}

// DiscoverBinaries scans the local storage/bins/{ecu,tcu,awd} folder tree first, followed by sibling directories.
func DiscoverBinaries() ([]BinFileItem, []BinFileItem, []BinFileItem) {
	_ = EnsureStorageTree()
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	type dirScanSpec struct {
		Dir        string
		ForcedMod  string // "ecu", "tcu", "awd", or "" for auto-by-size
	}

	scanSpecs := []dirScanSpec{
		{filepath.Join(cwd, "storage", "bins", "ecu"), "ecu"},
		{filepath.Join(cwd, "storage", "bins", "tcu"), "tcu"},
		{filepath.Join(cwd, "storage", "bins", "awd"), "awd"},
		{`C:\Google Antigravity\BinGo-Master`, ""},
		{`C:\Google Antigravity\Kia SIM2K-260`, ""},
		{`C:\Google Antigravity\Kia SIM2K-260\Claude_Bins`, ""},
		{`C:\Claude Projects\Kia SIM2K-260`, ""},
		{`C:\Claude Projects\Kia SIM2K-260\Claude_Bins`, ""},
		{`C:\Claude Projects\Kefico CPTSH\out`, ""},
		{`C:\Claude Projects\TCU Bins`, ""},
	}

	seenPaths := make(map[string]bool)
	var ecuBins, tcuBins, awdBins []BinFileItem

	for _, spec := range scanSpecs {
		entries, err := os.ReadDir(spec.Dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fname := entry.Name()
			lowerName := strings.ToLower(fname)
			if !strings.HasSuffix(lowerName, ".bin") && !strings.HasSuffix(lowerName, ".rom") && !strings.HasSuffix(lowerName, ".ori") {
				continue
			}
			fullPath := filepath.Clean(filepath.Join(spec.Dir, fname))
			if seenPaths[lowerName] {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			sz := info.Size()

			mod := spec.ForcedMod
			if mod == "" {
				switch sz {
				case 4194304, 2097152:
					mod = "ecu"
				case 785408, 786432:
					mod = "tcu"
				case 49152, 32768:
					mod = "awd"
				}
			}

			switch mod {
			case "ecu":
				seenPaths[lowerName] = true
				name, desc, tag := describeECUBin(fname)
				ecuBins = append(ecuBins, BinFileItem{
					ID:          fname,
					Module:      "ecu",
					Name:        name,
					Filename:    fname,
					Path:        fullPath,
					SizeBytes:   sz,
					BaseAddrHex: "0x80000000 (Cal 0x80800000 @ +0x200000)",
					Description: desc,
					Tag:         tag,
				})
			case "tcu":
				seenPaths[lowerName] = true
				name, desc, tag := describeTCUBin(fname)
				baseHex := "0x1C0000 (768 KB)"
				if sz == 785408 {
					baseHex = "0x1C0200 (767 KB)"
				}
				tcuBins = append(tcuBins, BinFileItem{
					ID:          fname,
					Module:      "tcu",
					Name:        name,
					Filename:    fname,
					Path:        fullPath,
					SizeBytes:   sz,
					BaseAddrHex: baseHex,
					Description: desc,
					Tag:         tag,
				})
			case "awd":
				seenPaths[lowerName] = true
				name, desc, tag := describeAWDBin(fname, sz)
				baseHex := "0xC00000 (48 KB)"
				if sz == 32768 {
					baseHex = "0xC04000 (32 KB)"
				}
				awdBins = append(awdBins, BinFileItem{
					ID:          fname,
					Module:      "awd",
					Name:        name,
					Filename:    fname,
					Path:        fullPath,
					SizeBytes:   sz,
					BaseAddrHex: baseHex,
					Description: desc,
					Tag:         tag,
				})
			}
		}
	}

	// Sort so key baseline/tuned files appear first
	sort.Slice(ecuBins, func(i, j int) bool {
		return ecuSortPriority(ecuBins[i].Filename) < ecuSortPriority(ecuBins[j].Filename)
	})
	sort.Slice(tcuBins, func(i, j int) bool {
		return tcuSortPriority(tcuBins[i].Filename) < tcuSortPriority(tcuBins[j].Filename)
	})
	sort.Slice(awdBins, func(i, j int) bool {
		return awdSortPriority(awdBins[i].Filename) < awdSortPriority(awdBins[j].Filename)
	})

	return ecuBins, tcuBins, awdBins
}

func ecuSortPriority(fname string) int {
	f := strings.ToLower(fname)
	switch {
	case strings.Contains(f, "quiet_good"):
		return 1
	case strings.Contains(f, "v103"):
		return 2
	case strings.Contains(f, "kimjongtuned"):
		return 3
	case strings.Contains(f, "ralv1"):
		return 4
	case strings.Contains(f, "bad_loud"):
		return 5
	case strings.Contains(f, "v58"):
		return 6
	default:
		return 50
	}
}

func tcuSortPriority(fname string) int {
	f := strings.ToLower(fname)
	switch {
	case strings.Contains(f, "stock_read_complete"):
		return 1
	case strings.Contains(f, "ekm422402108_tuned"):
		return 2
	case strings.Contains(f, "write_200056"):
		return 3
	case strings.Contains(f, "brakeslot"):
		return 4
	default:
		return 50
	}
}

func awdSortPriority(fname string) int {
	f := strings.ToLower(fname)
	switch {
	case strings.Contains(f, "c00000_c0bfff"):
		return 1
	case strings.Contains(f, "sportlaunch_30mph"):
		return 2
	case strings.Contains(f, "cal_32k"):
		return 3
	default:
		return 50
	}
}

func describeECUBin(fname string) (string, string, string) {
	f := strings.ToLower(fname)
	switch {
	case strings.Contains(f, "quiet_good"):
		return "SIM2K-260 Base Calibration (Quiet Good / Q)",
			"Stock torque/ignition/boost architecture, TCS active (0x200B4B=0), -3.0 deg shift spark floor (0x207AD9), TCHA cut disabled.",
			"STOCK"
	case strings.Contains(f, "v103"):
		return "IntensePop v103 (Active Street/Strip Tune)",
			"TCS torque cut inhibited (0x200B4B=1), -6.0 deg shift spark floor, TCHA surge disabled, decel pop/burble calibration.",
			"TUNED"
	case strings.Contains(f, "kimjongtuned"):
		return "KimJongTuned BEF (640C7010 / 640C7051)",
			"Aggressive WOT ignition (+3.4 to +5.2 deg advance), +12.0 deg shift spark floor (blocks ECU spark cut on shifts), lowered TQI_BAS.",
			"RACE"
	case strings.Contains(f, "ralv1"):
		return "RALv1 Rolling Antilag Patch (0x808DC000)",
			"Contains custom TriCore code caves at 0x808DC000 for stationary & rolling antilag boost building.",
			"RACE"
	case strings.Contains(f, "bad_loud"):
		return "Sleepy Loud Decat (TCHA Limiter Surge Bug)",
			"Inflated TQI_BAS (+17% to 967.8 Nm) + active TCHA turbo speed cut (0x203EC6=10 cycles -> 200 Nm drop & 17 Nm/cycle sawtooth oscillation).",
			"DIAG"
	case strings.Contains(f, "v58"):
		return "IntensePop v58 (Deep -15 deg Shift Floor)",
			"Shift spark floor lowered to -15.0 deg (0x207AD9=55), Force Sport Mode (0x200CCE=1).",
			"TUNED"
	default:
		return fname, "SIM2K-260 4MB TriCore TC1791 Full Flash Binary", "TUNED"
	}
}

func describeTCUBin(fname string) (string, string, string) {
	f := strings.ToLower(fname)
	switch {
	case strings.Contains(f, "stock_read_complete"):
		return "Kefico CPTSH 8AT Stock (95441-4J571)",
			"Stock 2500 RPM brake-stall limit, 350/400 Nm torque protection caps, -10 shift cut, 850 mA solenoid bleed.",
			"STOCK"
	case strings.Contains(f, "ekm422402108_tuned"):
		return "EKM422402108 Tuned 8AT (3500 Stall / 700 Nm / Fast Shift)",
			"3500 RPM launch stall (0x238B68), 700 Nm torque caps, 90 ms shift timer (0x20FC52), -15 shift torque cut, 490 mA fast VCM solenoid bleed.",
			"TUNED"
	case strings.Contains(f, "write_200056"):
		return "CPTSH Manual Gate & Kickdown Mod (0x200056)",
			"Modified manual-mode auto-upshift gate and kickdown hysteresis thresholds.",
			"TUNED"
	case strings.Contains(f, "brakeslot"):
		return "CPTSH Brake Downshift Slot Test",
			"Experimental braking downshift schedule calibration.",
			"DIAG"
	default:
		return fname, "Kefico CPTSH 8AT SH72549R Calibration Binary", "TUNED"
	}
}

func describeAWDBin(fname string, sz int64) (string, string, string) {
	f := strings.ToLower(fname)
	switch {
	case strings.Contains(f, "sportlaunch_30mph"):
		return "Magna ATC SportLaunch 30mph Mod (Brake-Gate Bypass + 9000 Nm/s)",
			"Zeroes 64.5 psi brake-pressure gate (0xC06080), +50% coupling slew rate (180 Nm/20ms = 9000 Nm/s), 1000 Nm feed-forward held to 50 km/h (31 mph), 50:50 split.",
			"RACE"
	case strings.Contains(f, "c00000_c0bfff"):
		return "Magna ATC AWD Stock Full 48KB (95447-4J105)",
			"Stock 4.45 bar (64.5 psi) brake-launch inhibit gate, 120 Nm/20ms (6000 Nm/s) slew rate, 40% Sport front split tapering above 30 km/h.",
			"STOCK"
	default:
		return fmt.Sprintf("Magna ATC AWD Stock %dKB (%s)", sz/1024, fname),
			"Stock calibration segment from 95447-4J105 AWD controller.",
			"STOCK"
	}
}

// DiscoverDatabases checks the three BinGo SQLite databases (preferring storage/db/{ecu,tcu,awd}) and returns their metadata counts.
func DiscoverDatabases() []DatabaseItem {
	_ = EnsureStorageTree()
	specs := []struct {
		Module string
		Name   string
	}{
		{"ecu", "SIM2K-260 ECU Teardown DB (flash_analysis.db)"},
		{"tcu", "Kefico CPTSH 8AT Teardown DB (tcu_cptsh_95441_4J571.db)"},
		{"awd", "Magna ATC AWD Teardown DB (awd_95447_4J105.db)"},
	}

	var out []DatabaseItem
	for _, s := range specs {
		dbPath := getDBPathForModule(s.Module)
		item := DatabaseItem{
			Module: s.Module,
			Name:   s.Name,
			Path:   dbPath,
		}
		if dbPath != "" {
			if _, err := os.Stat(dbPath); err == nil {
				db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
				if err == nil {
					item.Available = true
					_ = db.QueryRow("SELECT COUNT(*) FROM cal_maps").Scan(&item.MapCount)
					_ = db.QueryRow("SELECT COUNT(*) FROM functions").Scan(&item.FuncCount)
					_ = db.QueryRow("SELECT COUNT(*) FROM memory_interactions").Scan(&item.XrefCount)
					db.Close()
				}
			}
		}
		out = append(out, item)
	}
	return out
}

// Binary helper functions
func readU8(data []byte, off int) uint8 {
	if off < 0 || off >= len(data) {
		return 0
	}
	return data[off]
}

func readU16LE(data []byte, off int) uint16 {
	if off < 0 || off+2 > len(data) {
		return 0
	}
	return binary.LittleEndian.Uint16(data[off : off+2])
}

func readU16BE(data []byte, off int) uint16 {
	if off < 0 || off+2 > len(data) {
		return 0
	}
	return binary.BigEndian.Uint16(data[off : off+2])
}

func readS16BE(data []byte, off int) int16 {
	return int16(readU16BE(data, off))
}

func readArrayU16LE(data []byte, off, count int, scale, offset float64) []float64 {
	res := make([]float64, count)
	for i := 0; i < count; i++ {
		res[i] = float64(readU16LE(data, off+i*2))*scale + offset
	}
	return res
}

func readArrayU8(data []byte, off, count int, scale, offset float64) []float64 {
	res := make([]float64, count)
	for i := 0; i < count; i++ {
		res[i] = float64(readU8(data, off+i))*scale + offset
	}
	return res
}

func readGridU16LE(data []byte, off, rows, cols int, scale, offset float64) ([][]float64, float64) {
	grid := make([][]float64, rows)
	maxVal := -1e9
	for r := 0; r < rows; r++ {
		row := make([]float64, cols)
		for c := 0; c < cols; c++ {
			v := float64(readU16LE(data, off+(r*cols+c)*2))*scale + offset
			row[c] = v
			if v > maxVal {
				maxVal = v
			}
		}
		grid[r] = row
	}
	return grid, maxVal
}

func readGridU8(data []byte, off, rows, cols int, scale, offset float64) ([][]float64, float64) {
	grid := make([][]float64, rows)
	maxVal := -1e9
	for r := 0; r < rows; r++ {
		row := make([]float64, cols)
		for c := 0; c < cols; c++ {
			v := float64(readU8(data, off+(r*cols+c)))*scale + offset
			row[c] = v
			if v > maxVal {
				maxVal = v
			}
		}
		grid[r] = row
	}
	return grid, maxVal
}

// DecodeECUProfile reads a 4MB SIM2K-260 binary and extracts all physical tables and scalars.
func DecodeECUProfile(binPath string) (*ECUProfile, error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ECU bin %s: %w", binPath, err)
	}
	if len(data) < 0x400000 {
		return nil, fmt.Errorf("invalid SIM2K-260 binary size %d (expected 4194304)", len(data))
	}

	p := &ECUProfile{
		BinPath: binPath,
		BinName: filepath.Base(binPath),
	}

	// 1. Drive Mode Override (0x200CCE)
	p.DriveModeForce = int(readU8(data, 0x200CCE))

	// 2. Driver Demand (12 RPM x 14 Pedal)
	// Continental u16 axis has count at +0, data at +2
	p.DrvRPM = readArrayU16LE(data, 0x213A92, 12, 1.0, 0.0)
	p.DrvPedal = readArrayU16LE(data, 0x213B3E, 14, 0.09765625, 0.0)
	p.DrvMapComfort, _ = readGridU16LE(data, 0x214280, 12, 14, 0.03125, 0.0)
	p.DrvMapMode4, _ = readGridU16LE(data, 0x214760, 12, 14, 0.03125, 0.0)
	var sportPeak float64
	p.DrvMapSport, sportPeak = readGridU16LE(data, 0x214E80, 12, 14, 0.03125, 0.0)
	p.DrvPeakNm = sportPeak

	// Gear & Speed factors (Q15 = 1/32768)
	p.GearAxis = readArrayU8(data, 0x201337, 10, 1.0, 0.0)
	p.GearPedalAxis = readArrayU16LE(data, 0x213ACA, 8, 0.09765625, 0.0)
	p.GearFactorMap, _ = readGridU16LE(data, 0x213B70, 10, 8, 1.0/32768.0, 0.0)

	p.SpdAxis = readArrayU8(data, 0x201342, 12, 1.0, 0.0)
	p.SpdPedalAxis = readArrayU16LE(data, 0x213AF2, 8, 0.09765625, 0.0)
	p.SpdFactorMap, _ = readGridU16LE(data, 0x215150, 12, 8, 1.0/32768.0, 0.0)

	// 3. Forward Torque Model TQI_BAS (18 RPM x 20 Airload mg/stk)
	const airScale = 0.042389562829
	p.FwdRPM = readArrayU16LE(data, 0x25546E, 18, 1.0, 0.0)
	p.FwdAir = readArrayU16LE(data, 0x255406, 20, airScale, 0.0)
	p.TQIBasA, p.TQIBasPeak = readGridU16LE(data, 0x255518, 18, 20, 0.03125, 0.0)
	p.TQIBasB, _ = readGridU16LE(data, 0x2557E8, 18, 20, 0.03125, 0.0)

	// 8 Cam-state TQI_MAX tables at 0x253D14 + k*0x2D0 (18x20)
	p.TQIMaxEnv = make([][]float64, 18)
	for r := 0; r < 18; r++ {
		p.TQIMaxEnv[r] = make([]float64, 20)
	}
	maxEnvPeak := 0.0
	for k := 0; k < 8; k++ {
		grid, _ := readGridU16LE(data, 0x253D14+k*0x2D0, 18, 20, 0.03125, 0.0)
		for r := 0; r < 18; r++ {
			for c := 0; c < 20; c++ {
				if grid[r][c] > p.TQIMaxEnv[r][c] {
					p.TQIMaxEnv[r][c] = grid[r][c]
				}
				if grid[r][c] > maxEnvPeak {
					maxEnvPeak = grid[r][c]
				}
			}
		}
	}
	p.TQIMaxPeak = maxEnvPeak

	// 4. Inverse Torque Model MAF_SP (18 RPM x 20 Torque)
	p.InvRPM = readArrayU16LE(data, 0x2554B2, 18, 1.0, 0.0)
	p.InvTq = readArrayU16LE(data, 0x2554EE, 20, 0.03125, 0.0)
	p.MAFSpHigh, _ = readGridU16LE(data, 0x25965C, 18, 20, airScale, 0.0)

	// 5. Basic Lambda Setpoint (0x24CE7C, 16x16 Q14) & ECU Torque Protection Ceiling
	// Note: 0x24CE7C is basic lambda setpoint (X / 16384.0), whereas the ECU torque ceiling is TQIMaxPeak (0x253D14..).
	lambdaGrid, _ := readGridU16LE(data, 0x24CE7C, 16, 16, 1.0/16384.0, 0.0)
	minLambda := 1.0
	for r := 0; r < 16; r++ {
		for c := 0; c < 16; c++ {
			if lambdaGrid[r][c] > 0.5 && lambdaGrid[r][c] < minLambda {
				minLambda = lambdaGrid[r][c]
			}
		}
	}
	p.LambdaSetptMin = minLambda
	p.ClutchLimitNm = p.TQIMaxPeak

	// 6. SIM2K-260 Airload Ceiling & Boost Setpoint Chain (0x8016AC20 -> 0x80136F84 -> 0x800F3CB8 -> 0x800F5DA4)
	//Note: 0x21EC00 is a Q15 unity weighting factor (32768 = 1.0), NOT a 2717 hPa boost target (see xdf_audit.md Item 15).
	const hpaScale = 0.082917525
	// 6a. Maximum Cylinder Air Charge vs RPM: 0x23D810 (M_AIR_CYL_MAX, 12 pts, axis at 0x23D5DE)
	p.MaxCylAirRPM = readArrayU16LE(data, 0x23D5DE, 12, 1.0, 0.0)
	p.MaxCylAirMg = readArrayU16LE(data, 0x23D810, 12, airScale, 0.0)

	// 6b. Pressure Upstream Throttle Setpoint vs RPM: 0x21DA7C (6 RPM x 4 Pressure, axis at 0x203988)
	p.PutSpRPM = readArrayU16LE(data, 0x203988, 6, 1.0, 0.0)
	putGrid, putPeak := readGridU16LE(data, 0x21DA7C, 6, 4, hpaScale, 0.0)
	p.PutSpCurveHpa = make([]float64, 6)
	for r := 0; r < 6; r++ {
		p.PutSpCurveHpa[r] = putGrid[r][3] // Top column (col 3) is active WOT setpoint
	}
	p.PutSpTablePeakHpa = putPeak

	// 6c. Maximum Allowed Compressor Pressure Quotient: 0x21EB38 (IP_PQ_CHA_MAX, 8 IAT x 8 RPM, Q12 = 1/4096, axis at 0x21EAC6)
	p.CompPQRPM = readArrayU16LE(data, 0x21EAC6, 8, 1.0, 0.0)
	p.CompPQGrid, p.CompPQMax = readGridU16LE(data, 0x21EB38, 8, 8, 1.0/4096.0, 0.0)
	p.CompPQWarmCurve = make([]float64, 8)
	for c := 0; c < 8; c++ {
		p.CompPQWarmCurve[c] = p.CompPQGrid[5][c] // Row 5 = 30C IAT warm operating row
	}

	// 6d. Overboost Diagnosis Ceiling CAP_H: 0x21F478 (6x6, hpaScale)
	_, p.BoostLimitAHpa = readGridU16LE(data, 0x21F478, 6, 6, hpaScale, 0.0)

	// Compute peak WOT boost target across 2500..6000 RPM at standard sea-level ambient (1013.25 hPa)
	peakWotBoostHpa := 0.0
	for _, testRPM := range []float64{2500, 3000, 3500, 4000, 4500, 5000, 5500, 6000} {
		bHpa := EvalECUBoostTargetHpa(p, testRPM, 1013.25)
		if bHpa > peakWotBoostHpa {
			peakWotBoostHpa = bHpa
		}
	}
	p.PutSpMaxHpa = peakWotBoostHpa

	p.TCHADebounceCyc = int(readU16LE(data, 0x203EC6))
	p.TCHAMinTorqueNm = float64(readU16LE(data, 0x203ED4)) * 0.03125
	p.TCHARampNmPerCyc = float64(readU16LE(data, 0x203ED8)) * 0.03125
	_, p.TCHAMaxTorqueCap = readGridU16LE(data, 0x203E26, 4, 2, 0.03125, 0.0)
	p.TCHASpeedFactor = float64(readU16LE(data, 0x203ECC)) / 65536.0
	p.TCHAInhibitSwitch = int(readU8(data, 0x204149))
	// In SLEEPY_BAD_Loud, 0x203EC6=10 cycles, 0x203ED4=200 Nm, 0x203ECC=49152 (0.75 factor), and TQI_BAS is inflated to 967.8 Nm
	lowerName := strings.ToLower(p.BinName)
	p.TCHALimitActive = p.TCHADebounceCyc < 60000 && p.TCHAMinTorqueNm < 1000 && strings.Contains(lowerName, "loud")

	// 7. Ignition Timing & Efficiency Model
	p.IgnRPM = readArrayU16LE(data, 0x236C42, 18, 1.0, 0.0)
	p.IgnLoad = readArrayU16LE(data, 0x236BD6, 20, airScale, 0.0)
	p.IgnBaseLow, _ = readGridU8(data, 0x236D50, 18, 20, 0.375, -35.625)
	p.IgnBaseHigh, _ = readGridU8(data, 0x238090, 18, 20, 0.375, -35.625)
	p.IgnOptHigh, _ = readGridU8(data, 0x238874, 18, 20, 0.375, -35.625)

	p.IGAMinCoolant = readArrayU8(data, 0x239825, 16, 0.75, -48.0)
	p.IGAMinLoad = readArrayU16LE(data, 0x239802, 16, airScale, 0.0)
	p.IGAMinHigh, _ = readGridU8(data, 0x239AF4, 16, 16, 0.375, -35.625)
	// Warm row 14 (81C), high load col 15
	p.IGAMinWarmWOTDeg = p.IGAMinHigh[14][15]
	p.ShiftSparkFloor = float64(readU8(data, 0x207AD9))*0.375 - 35.625

	p.EffToRetardAxis = readArrayU16LE(data, 0x25743E, 16, 1.0/32768.0, 0.0)
	p.EffToRetardDeg = readArrayU8(data, 0x25262C, 16, 0.375, 0.0)
	p.RetardToEffAxis = readArrayU8(data, 0x2553AA, 16, 0.375, 0.0)
	p.RetardToEffVal = readArrayU16LE(data, 0x2524EC, 16, 1.0/32768.0, 0.0)

	// 8. TCS & RAL Antilag Patch
	p.TCSInhibit = int(readU8(data, 0x200B4B))
	p.TCSMinRPM = float64(readU16LE(data, 0x203098))

	// Check 0x3DC000..0x3DC100 (EA 0x808DC000) for non-0xFF/non-0x00 RAL code cave
	nonEmpty := 0
	for i := 0; i < 256; i++ {
		if data[0x3DC000+i] != 0xFF && data[0x3DC000+i] != 0x00 {
			nonEmpty++
		}
	}
	p.RALPatchDetected = nonEmpty > 32 || strings.Contains(lowerName, "ral")

	return p, nil
}

// EvalECUBoostTargetHpa evaluates the SIM2K-260 WOT manifold/charge-pipe pressure setpoint (hPa) at a given RPM.
// Traces the exact TriCore firmware chain:
//  1. 0x8016AC20 -> 0x23D810 (M_AIR_CYL_MAX vs RPM)
//  2. 0x80136F84 -> Ideal-gas cylinder filling conversion (0.6264 mg/stk per hPa for 557 cc/cyl at 20C)
//  3. 0x800F3CB8 -> 0x21DA7C (PUT_SP vs RPM) and 0x21EB38 (IP_PQ_CHA_MAX compressor pressure quotient ceiling)
func EvalECUBoostTargetHpa(ecu *ECUProfile, rpm, ambientHpa float64) float64 {
	if ambientHpa <= 500.0 {
		ambientHpa = 1013.25
	}
	mAirCapMg := Interp1D(ecu.MaxCylAirRPM, ecu.MaxCylAirMg, rpm)
	if mAirCapMg <= 200.0 {
		mAirCapMg = 1400.0
	}
	putDa7cHpa := Interp1D(ecu.PutSpRPM, ecu.PutSpCurveHpa, rpm)
	if putDa7cHpa <= 800.0 {
		putDa7cHpa = 2155.0
	}
	pqMax := Interp1D(ecu.CompPQRPM, ecu.CompPQWarmCurve, rpm)
	if pqMax <= 1.2 {
		pqMax = 3.05
	}

	// Twin-MGT1549DSL natural volumetric / turbine backpressure taper above 4500 RPM (verified in PandaCapture logs)
	highRpmTaper := 1.0
	if rpm > 4500.0 {
		highRpmTaper = 1.0 - 0.055*math.Min(1.35, (rpm-4500.0)/1500.0)
	}

	// If 0x21DA7C is tapered at high RPM (e.g. v101/v103/RALv1: 2400 -> 2350 -> 2240 -> 2155 hPa),
	// blend the high-RPM PUT_SP shape into the air-based pressure target
	da7cShape := 1.0
	if rpm > 4000.0 && ecu.PutSpTablePeakHpa > 2160.0 {
		da7cShape = math.Min(1.0, putDa7cHpa/ecu.PutSpTablePeakHpa)
	}

	// Ideal-gas cylinder filling on 3.342L V6 (557.0 cc/cyl): 0.6264 mg/stk per hPa (1400 mg/stk = 2235.0 hPa = 17.7-18.2 psi)
	pFromAirHpa := (mAirCapMg / 0.6264) * highRpmTaper * (0.35 + 0.65*da7cShape)
	// Also honor direct 0x21DA7C PUT_SP table (0.923 * 2155 hPa = 1989 hPa = 14.2 psi stock 3.3T target; 2300..2619 hPa in KJT standalone)
	pFromDa7cHpa := putDa7cHpa * 0.923 * highRpmTaper
	pTargetHpa := math.Max(pFromAirHpa, pFromDa7cHpa)

	// Clamp by compressor pressure quotient 0x21EB38 (accounting for air cleaner + intercooler + throttle pressure drop)
	pCompCapHpa := ambientHpa * (pqMax * 0.78)
	pMapHpa := math.Min(pTargetHpa, pCompCapHpa)

	// Clamp by 0x21F478 (CAP_H overboost threshold) if active
	if ecu.BoostLimitAHpa > 1200.0 && ecu.BoostLimitAHpa < 3500.0 {
		pMapHpa = math.Min(pMapHpa, ecu.BoostLimitAHpa)
	}
	return math.Max(ambientHpa, pMapHpa)
}


// DecodeTCUProfile reads a Kefico CPTSH 8AT binary (Big-Endian) and extracts shift, stall, and torque tables.
func DecodeTCUProfile(binPath string) (*TCUProfile, error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TCU bin %s: %w", binPath, err)
	}
	var swBase int
	switch len(data) {
	case 785408:
		swBase = 0x1C0200
	case 786432:
		swBase = 0x1C0000
	default:
		return nil, fmt.Errorf("invalid CPTSH TCU binary size %d", len(data))
	}

	swOff := func(sw int) int {
		return sw - swBase
	}

	p := &TCUProfile{
		BinPath: binPath,
		BinName: filepath.Base(binPath),
	}

	// Standard 20-point pedal axis for 14x20 shift schedule maps (0..100%)
	p.PedalAxis20 = []float64{
		0, 5, 10, 15, 20, 25, 30, 35, 40, 45,
		50, 55, 60, 65, 70, 75, 80, 85, 90, 100,
	}

	readShiftSlot := func(slotIdx int) [][]float64 {
		descSW := 0x1C0000 + 0x44350 + slotIdx*0x13C
		gridOff := swOff(descSW + 36)
		grid := make([][]float64, 20)
		for r := 0; r < 20; r++ {
			row := make([]float64, 14)
			for c := 0; c < 14; c++ {
				raw := readU8(data, gridOff+r*14+c)
				// CPTSH A8LR1 output-shaft speed scaling to physical road speed (km/h)
				row[c] = round2(float64(raw) * 1.328)
			}
			grid[r] = row
		}
		return grid
	}

	p.ComfortGrid = readShiftSlot(3) // Group 1 Comfort
	p.EcoGrid = readShiftSlot(9)     // Group 2 Eco
	p.SmartGrid = readShiftSlot(17)  // Group 3 Smart
	p.SportGrid = readShiftSlot(23)  // Group 4 Sport

	p.ComfortWOTUpKmh = append([]float64(nil), p.ComfortGrid[19][0:7]...)
	p.SportWOTUpKmh = append([]float64(nil), p.SportGrid[19][0:7]...)

	// Launch Control / Brake-Stall RPM Limit (7x1 u16 BE at sw 0x238B68)
	maxStall := 0.0
	for i := 0; i < 7; i++ {
		v := float64(readU16BE(data, swOff(0x238B68)+i*2))
		if v > maxStall {
			maxStall = v
		}
	}
	p.LaunchStallRPM = maxStall

	// Primary Torque Protection Limit (6x2 u16 BE at sw 0x1FD198)
	p.PrimaryTqCapNm = float64(readU16BE(data, swOff(0x1FD198)))

	// Per-Gear Torque Ceiling (8x1 u16 BE at sw 0x1FD1B0)
	p.PerGearCapNm = make([]float64, 8)
	for g := 0; g < 8; g++ {
		p.PerGearCapNm[g] = float64(readU16BE(data, swOff(0x1FD1B0)+g*2))
	}

	// Global Torque Cap (8x4 u16 BE at sw 0x2096E4)
	p.GlobalTqCapNm = float64(readU16BE(data, swOff(0x2096E4)))

	// 6x 8x8 Clutch Torque Limit maps (sw 0x1FBC5C, stride 0xA4, grid at +36)
	maxClutch := 0.0
	for m := 0; m < 6; m++ {
		off := swOff(0x1FBC5C + m*0xA4 + 36)
		for i := 0; i < 64; i++ {
			v := float64(readU16BE(data, off+i*2))
			if v > maxClutch && v < 10000 {
				maxClutch = v
			}
		}
	}
	p.ClutchLimitPeakNm = maxClutch

	// Shift Torque Intervention (14x 4x3 s16 BE at sw 0x233D3C, grid at +18)
	p.ShiftCutRaw = int(readS16BE(data, swOff(0x233D3C+18)))

	// Shift Timer scalar (sw 0x20FC52 u16 BE ms)
	p.ShiftTimerMs = int(readU16BE(data, swOff(0x20FC52)))

	// Solenoid VCM Bleed Curve (28x1 u16 BE at sw 0x24BB22, min floor in tail)
	minMa := 9999
	for i := 0; i < 28; i++ {
		v := int(readU16BE(data, swOff(0x24BB22)+i*2))
		if v > 100 && v < minMa {
			minMa = v
		}
	}
	p.VCMSolenoidFloor = minMa

	return p, nil
}

// DecodeAWDProfile reads a Magna ATC AWD binary (Little-Endian) and extracts all coupling tables and gates.
func DecodeAWDProfile(binPath string) (*AWDProfile, error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read AWD bin %s: %w", binPath, err)
	}
	var eaBase int
	switch len(data) {
	case 49152:
		eaBase = 0xC00000
	case 32768:
		eaBase = 0xC04000
	default:
		return nil, fmt.Errorf("invalid Magna ATC AWD binary size %d", len(data))
	}

	eaOff := func(ea int) int {
		return ea - eaBase
	}

	p := &AWDProfile{
		BinPath: binPath,
		BinName: filepath.Base(binPath),
	}

	const q4Scale = 0.0625 // 1/16

	// 1. Nominal Tire Radius R_TIRE (0xC04214: 0x7800 -> high byte 0x78 = 120, + 200 = 320 mm)
	rawTire := readU16LE(data, eaOff(0xC04214))
	p.TireRadiusMm = float64(rawTire>>8) + 200.0
	if p.TireRadiusMm < 280 || p.TireRadiusMm > 380 {
		p.TireRadiusMm = 320.0
	}

	// 2. Base Coupling Preload Clamp (0xC04334, 11x11 u16 Q4 Nm)
	_, p.BaseClampNm = readGridU16LE(data, eaOff(0xC04334), 11, 11, q4Scale, 0.0)

	// 3. Per-Gear Launch Preload Maps (16 gears x 5 pedal [10,20,30,50,80]% in Q4 Nm)
	p.LaunchPedalAxis = []float64{10, 20, 30, 50, 80}
	p.LaunchPreloadEco, _ = readGridU16LE(data, eaOff(0xC046D8), 16, 5, q4Scale, 0.0)
	p.LaunchPreloadSport, _ = readGridU16LE(data, eaOff(0xC04778), 16, 5, q4Scale, 0.0)

	// 4. Off-Idle Base Preload (16x4 Q4 Nm: row 1 is 1st gear at +8 bytes)
	p.OffIdleNormalNm = readArrayU16LE(data, eaOff(0xC04A3C+8), 4, q4Scale, 0.0)
	p.OffIdleEcoNm = readArrayU16LE(data, eaOff(0xC04ABC+8), 4, q4Scale, 0.0)

	// 5. Speed x Pedal Feed-Forward Coupling Torque (10 speed x 5 pedal Q4 Nm)
	p.FFComfortSpeedKmh = readArrayU16LE(data, eaOff(0xC0667E), 10, q4Scale, 0.0)
	p.FFSportSpeedKmh = readArrayU16LE(data, eaOff(0xC06692), 10, q4Scale, 0.0)
	p.FFPedalAxis = []float64{0, 20, 40, 60, 100}
	p.FFComfortMapNm, _ = readGridU16LE(data, eaOff(0xC04B3C), 10, 5, q4Scale, 0.0)
	p.FFSportMapNm, _ = readGridU16LE(data, eaOff(0xC04BA0), 10, 5, q4Scale, 0.0)

	// 6. Front Torque Split % Matrices (5 speed x 5 pedal integer %)
	p.SplitSpeedAxisKmh = []float64{0, 30, 60, 100, 160}
	p.SplitPedalAxis = []float64{0, 25, 50, 75, 100}
	p.SplitComfortPct, _ = readGridU16LE(data, eaOff(0xC0585A), 5, 5, 1.0, 0.0)
	p.SplitMod38Pct, _ = readGridU16LE(data, eaOff(0xC058F0), 5, 5, 1.0, 0.0)
	p.SplitSportPct, _ = readGridU16LE(data, eaOff(0xC05922), 5, 5, 1.0, 0.0)

	// 7. Max Torque Ceilings (0xC059EA Comfort, 0xC05A1C Sport, integer Nm)
	_, p.CeilingComfortNm = readGridU16LE(data, eaOff(0xC059EA), 5, 5, 1.0, 0.0)
	_, p.CeilingSportNm = readGridU16LE(data, eaOff(0xC05A1C), 5, 5, 1.0, 0.0)

	// 8. Brake Cylinder Pressure Gate
	p.BrakeGateBar = float64(readU16LE(data, eaOff(0xC07498))) * 0.1
	p.BrakeGateQ88Bar = float64(readU16LE(data, eaOff(0xC07606))) / 256.0
	gate6080 := readU16LE(data, eaOff(0xC06080))
	gate60B0 := readU16LE(data, eaOff(0xC060B0))
	p.BrakeGateBypassed = (gate6080 == 0 && gate60B0 >= 10000)

	// 9. Positive Slew Rate Limiter (0xC07650: Q4 Nm per 20ms cycle)
	p.SlewRateNmPer20ms = float64(readU16LE(data, eaOff(0xC07650))) * q4Scale
	p.SlewRateNmPerSec = p.SlewRateNmPer20ms * 50.0 // 50 cycles/sec (20ms loop)

	// 10. High-Speed Gates & Slip Transfer
	p.HighSpdGate1Kmh = float64(readU16LE(data, eaOff(0xC066F6))) * q4Scale
	p.HighSpdGate2Axis = readArrayU16LE(data, eaOff(0xC06F96), 5, q4Scale, 0.0)
	gate2Grid, _ := readGridU16LE(data, eaOff(0xC056CA), 5, 5, 1.0, 0.0)
	p.HighSpdGate2Split = make([]float64, 5)
	for r := 0; r < 5; r++ {
		p.HighSpdGate2Split[r] = gate2Grid[r][4]
	}
	p.MasterTaperAxisKmh = readArrayU16LE(data, eaOff(0xC06252), 16, q4Scale, 0.0)
	p.MasterTaperPct = readArrayU16LE(data, eaOff(0xC0640A), 16, q4Scale, 0.0)
	p.SlipDeadbandKmh = float64(readU16LE(data, eaOff(0xC06978))) * q4Scale
	p.SlipCurveDeltaKmh = []float64{0, 1, 2, 3, 4, 6, 8, 10, 13, 16, 20}
	p.SlipCurveTorqueNm = readArrayU16LE(data, eaOff(0xC061FA), 11, q4Scale, 0.0)

	return p, nil
}

// BuildExtractedCalSummary creates a serializable summary of all decoded ECU, TCU, and AWD parameters.
func BuildExtractedCalSummary(ecu *ECUProfile, tcu *TCUProfile, awd *AWDProfile, driveMode string) ExtractedCalSummary {
	wotSpark := make([]float64, len(ecu.IgnRPM))
	wotTorque := make([]float64, len(ecu.FwdRPM))
	for r := 0; r < len(ecu.IgnRPM); r++ {
		wotSpark[r] = round2(ecu.IgnBaseHigh[r][18]) // col 18 (~1300 mg/stk WOT)
	}
	for r := 0; r < len(ecu.FwdRPM); r++ {
		tBas := ecu.TQIBasA[r][18]
		tMax := ecu.TQIMaxEnv[r][18]
		wotTorque[r] = round2(math.Min(tBas, tMax))
	}

	activeUp := tcu.SportWOTUpKmh
	if driveMode == "comfort" || driveMode == "eco" {
		activeUp = tcu.ComfortWOTUpKmh
	}

	ffWot := make([]float64, 10)
	ffAxis := awd.FFSportSpeedKmh
	for r := 0; r < 10; r++ {
		if driveMode == "comfort" || driveMode == "eco" {
			ffWot[r] = awd.FFComfortMapNm[r][4]
			ffAxis = awd.FFComfortSpeedKmh
		} else {
			ffWot[r] = awd.FFSportMapNm[r][4]
		}
	}

	splitWot := make([]float64, 5)
	for r := 0; r < 5; r++ {
		if driveMode == "comfort" || driveMode == "eco" {
			splitWot[r] = awd.SplitComfortPct[r][4]
		} else if awd.BrakeGateBypassed && awd.SplitMod38Pct[0][4] > awd.SplitSportPct[0][4] {
			splitWot[r] = awd.SplitMod38Pct[r][4]
		} else {
			splitWot[r] = awd.SplitSportPct[r][4]
		}
	}

	launch1st := awd.LaunchPreloadSport[1]
	if driveMode == "eco" {
		launch1st = awd.LaunchPreloadEco[1]
	}

	return ExtractedCalSummary{
		ECUBinName:            ecu.BinName,
		ECUDriveModeForce:     ecu.DriveModeForce,
		ECUDriverDemandPeakNm: round2(ecu.DrvPeakNm),
		ECUTQIBasPeakNm:       round2(ecu.TQIBasPeak),
		ECUTQIMaxPeakNm:       round2(ecu.TQIMaxPeak),
		ECUClutchLimitNm:      round2(ecu.ClutchLimitNm),
		ECUBoostTargetHpa:     round2(ecu.PutSpMaxHpa),
		ECUBoostLimitAHpa:     round2(ecu.BoostLimitAHpa),
		ECUTCHALimitActive:    ecu.TCHALimitActive,
		ECUTCHADebounceCyc:    ecu.TCHADebounceCyc,
		ECUTCHAMinTorqueNm:    round2(ecu.TCHAMinTorqueNm),
		ECUTCHARampNmPerCyc:   round2(ecu.TCHARampNmPerCyc),
		ECUShiftSparkFloorDeg: round2(ecu.ShiftSparkFloor),
		ECUIGAMinWarmWOTDeg:   round2(ecu.IGAMinWarmWOTDeg),
		ECUTCSInhibit:         ecu.TCSInhibit,
		ECURALPatchDetected:   ecu.RALPatchDetected,
		ECUWOTSparkCurveDeg:   wotSpark,
		ECUWOTTorqueCurveNm:   wotTorque,
		ECURPMAxis:            ecu.FwdRPM,

		TCUBinName:            tcu.BinName,
		TCULaunchStallRPM:     tcu.LaunchStallRPM,
		TCUPrimaryTorqueCapNm: tcu.PrimaryTqCapNm,
		TCUPerGearCapNm:       tcu.PerGearCapNm,
		TCUGlobalTorqueCapNm:  tcu.GlobalTqCapNm,
		TCUClutchLimitPeakNm:  tcu.ClutchLimitPeakNm,
		TCUWOTUpshiftKmh:      activeUp,
		TCUComfortUpshiftKmh:  tcu.ComfortWOTUpKmh,
		TCUSportUpshiftKmh:    tcu.SportWOTUpKmh,
		TCUShiftCutRaw:        tcu.ShiftCutRaw,
		TCUShiftTimerMs:       tcu.ShiftTimerMs,
		TCUVCMSolenoidFloorMa: tcu.VCMSolenoidFloor,

		AWDBinName:            awd.BinName,
		AWDTireRadiusMm:       awd.TireRadiusMm,
		AWDBaseClampNm:        awd.BaseClampNm,
		AWDLaunchPreload1stNm: launch1st,
		AWDOffIdlePreloadNm:   awd.OffIdleNormalNm,
		AWDFeedForwardWOTNm:   ffWot,
		AWDFeedForwardAxisKmh: ffAxis,
		AWDFrontSplitWOTPct:   splitWot,
		AWDMaxCeilingNm:       awd.CeilingSportNm,
		AWDBrakeGateBar:       round2(awd.BrakeGateQ88Bar),
		AWDBrakeGateBypassed:  awd.BrakeGateBypassed,
		AWDSlewRateNmPer20ms:  awd.SlewRateNmPer20ms,
		AWDSlewRateNmPerSec:   awd.SlewRateNmPerSec,
	}
}

// Interpolation Utilities

// Interp1D performs clamped linear interpolation on a 1D table (supports ascending or descending X).
func Interp1D(xAxis, yVals []float64, x float64) float64 {
	n := len(xAxis)
	if n == 0 || len(yVals) == 0 {
		return 0
	}
	if n == 1 {
		return yVals[0]
	}
	// Ascending axis
	if xAxis[n-1] >= xAxis[0] {
		if x <= xAxis[0] {
			return yVals[0]
		}
		if x >= xAxis[n-1] {
			return yVals[n-1]
		}
		for i := 0; i < n-1; i++ {
			if x >= xAxis[i] && x <= xAxis[i+1] {
				dx := xAxis[i+1] - xAxis[i]
				if math.Abs(dx) < 1e-9 {
					return yVals[i]
				}
				t := (x - xAxis[i]) / dx
				return yVals[i] + t*(yVals[i+1]-yVals[i])
			}
		}
	} else {
		// Descending axis
		if x >= xAxis[0] {
			return yVals[0]
		}
		if x <= xAxis[n-1] {
			return yVals[n-1]
		}
		for i := 0; i < n-1; i++ {
			if x <= xAxis[i] && x >= xAxis[i+1] {
				dx := xAxis[i] - xAxis[i+1]
				if math.Abs(dx) < 1e-9 {
					return yVals[i]
				}
				t := (xAxis[i] - x) / dx
				return yVals[i] + t*(yVals[i+1]-yVals[i])
			}
		}
	}
	return yVals[n-1]
}

// Interp2D performs clamped bilinear interpolation on a 2D grid[row][col] where rowAxis has len(grid) and colAxis has len(grid[0]).
func Interp2D(rowAxis, colAxis []float64, grid [][]float64, rVal, cVal float64) float64 {
	rows := len(grid)
	if rows == 0 || len(rowAxis) < rows {
		return 0
	}
	cols := len(grid[0])
	if cols == 0 || len(colAxis) < cols {
		return 0
	}

	rIdx, rFrac := findBracket(rowAxis[:rows], rVal)
	cIdx, cFrac := findBracket(colAxis[:cols], cVal)

	v00 := grid[rIdx][cIdx]
	v01 := grid[rIdx][cIdx+1]
	v10 := grid[rIdx+1][cIdx]
	v11 := grid[rIdx+1][cIdx+1]

	top := v00 + cFrac*(v01-v00)
	bot := v10 + cFrac*(v11-v10)
	return top + rFrac*(bot-top)
}

func findBracket(axis []float64, val float64) (int, float64) {
	n := len(axis)
	if n < 2 {
		return 0, 0
	}
	if val <= axis[0] {
		return 0, 0
	}
	if val >= axis[n-1] {
		return n - 2, 1.0
	}
	for i := 0; i < n-1; i++ {
		if val >= axis[i] && val <= axis[i+1] {
			span := axis[i+1] - axis[i]
			if math.Abs(span) < 1e-9 {
				return i, 0
			}
			return i, (val - axis[i]) / span
		}
	}
	return n - 2, 1.0
}

// QueryDatabaseMaps returns cal_maps from the requested module's SQLite database.
func QueryDatabaseMaps(module, search, category string, limit int) ([]CalMapRecord, error) {
	dbPath := getDBPathForModule(module)
	if dbPath == "" {
		return nil, fmt.Errorf("unknown module %s", module)
	}
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if limit <= 0 || limit > 1000 {
		limit = 300
	}

	query := `
		SELECT name, ea, COALESCE(rows, 1), COALESCE(columns, 1), COALESCE(data_org, 'uint16'),
		       COALESCE(axis_x_ea, 0), COALESCE(axis_y_ea, 0),
		       COALESCE(unit, ''), COALESCE(factor, 1.0), COALESCE(offset, 0.0),
		       COALESCE(folder, ''), COALESCE(comment, ''), COALESCE(applicability, '')
		FROM cal_maps
		WHERE 1=1
	`
	var args []interface{}
	if search != "" {
		query += ` AND (name LIKE ? OR printf('0x%X', ea) LIKE ? OR comment LIKE ? OR folder LIKE ?)`
		pat := "%" + search + "%"
		args = append(args, pat, pat, pat, pat)
	}
	if category != "" && category != "all" {
		query += ` AND folder = ?`
		args = append(args, category)
	}
	query += ` ORDER BY ea ASC LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []CalMapRecord
	for rows.Next() {
		var rec CalMapRecord
		rec.Module = module
		var eaInt, axX, axY int64
		if err := rows.Scan(
			&rec.Name, &eaInt, &rec.Rows, &rec.Cols, &rec.ElementType,
			&axX, &axY, &rec.Units, &rec.ScaleFactor, &rec.OffsetVal,
			&rec.Category, &rec.Description, &rec.Confidence,
		); err != nil {
			continue
		}
		rec.AddressInt = uint64(eaInt)
		rec.AddressHex = fmt.Sprintf("0x%X", rec.AddressInt)
		if axX > 0 {
			rec.XAxisAddrHex = fmt.Sprintf("0x%X", axX)
		}
		if axY > 0 {
			rec.YAxisAddrHex = fmt.Sprintf("0x%X", axY)
		}
		list = append(list, rec)
	}
	return list, nil
}

func getDBPathForModule(module string) string {
	mod := strings.ToLower(module)
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	var defaultName, fallbackPath string
	switch mod {
	case "ecu":
		defaultName = "flash_analysis.db"
		fallbackPath = `C:\Google Antigravity\BinGo-Master\flash_analysis.db`
	case "tcu":
		defaultName = "tcu_cptsh_95441_4J571.db"
		fallbackPath = `C:\Google Antigravity\BinGo-Master\tcu_cptsh_95441_4J571.db`
	case "awd":
		defaultName = "awd_95447_4J105.db"
		fallbackPath = `C:\Google Antigravity\BinGo-Master\awd_95447_4J105.db`
	default:
		return ""
	}

	localDir := filepath.Join(cwd, "storage", "db", mod)
	canonicalLocal := filepath.Join(localDir, defaultName)
	if info, err := os.Stat(canonicalLocal); err == nil && !info.IsDir() {
		return canonicalLocal
	}

	// Check if user imported any other .db/.sqlite/.sqlite3 file into storage/db/<module>/
	if entries, err := os.ReadDir(localDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ln := strings.ToLower(e.Name())
			if strings.HasSuffix(ln, ".db") || strings.HasSuffix(ln, ".sqlite") || strings.HasSuffix(ln, ".sqlite3") {
				return filepath.Clean(filepath.Join(localDir, e.Name()))
			}
		}
	}

	return fallbackPath
}

// CompareMapAcrossBins reads a specific map from the module's SQLite DB and decodes its 2D values from Bin A and Bin B.
func CompareMapAcrossBins(module, addrHex, binAPath, binBPath string) (*MapCompareResponse, error) {
	dbPath := getDBPathForModule(module)
	if dbPath == "" {
		return nil, fmt.Errorf("unknown module %s", module)
	}
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	parsedAddr, parseErr := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(addrHex), "0x"), 16, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("invalid hex address %s: %w", addrHex, parseErr)
	}

	var rec CalMapRecord
	rec.Module = module
	var eaInt, axX, axY int64
	err = db.QueryRow(`
		SELECT name, ea, COALESCE(rows, 1), COALESCE(columns, 1), COALESCE(data_org, 'uint16'),
		       COALESCE(axis_x_ea, 0), COALESCE(axis_y_ea, 0),
		       COALESCE(unit, ''), COALESCE(factor, 1.0), COALESCE(offset, 0.0),
		       COALESCE(folder, ''), COALESCE(comment, ''), COALESCE(applicability, '')
		FROM cal_maps
		WHERE ea = ?
		LIMIT 1
	`, int64(parsedAddr)).Scan(
		&rec.Name, &eaInt, &rec.Rows, &rec.Cols, &rec.ElementType,
		&axX, &axY, &rec.Units, &rec.ScaleFactor, &rec.OffsetVal,
		&rec.Category, &rec.Description, &rec.Confidence,
	)
	if err != nil {
		rec = CalMapRecord{
			Module:      module,
			Name:        "Custom_" + addrHex,
			AddressHex:  addrHex,
			AddressInt:  parsedAddr,
			Rows:        8,
			Cols:        8,
			ElementType: "uint16",
			ScaleFactor: 1.0,
		}
	} else {
		rec.AddressInt = uint64(eaInt)
		rec.AddressHex = fmt.Sprintf("0x%X", rec.AddressInt)
		if axX > 0 {
			rec.XAxisAddrHex = fmt.Sprintf("0x%X", axX)
		}
		if axY > 0 {
			rec.YAxisAddrHex = fmt.Sprintf("0x%X", axY)
		}
	}

	resp := &MapCompareResponse{
		Map: rec,
	}

	if binAPath != "" {
		if gA, err := decodeMapFromBin(module, rec, binAPath); err == nil {
			resp.GridA = gA
		}
	}
	if binBPath != "" {
		if gB, err := decodeMapFromBin(module, rec, binBPath); err == nil {
			resp.GridB = gB
		}
	}

	if resp.GridA != nil && resp.GridB != nil {
		rows := len(resp.GridA.Values)
		cols := 0
		if rows > 0 {
			cols = len(resp.GridA.Values[0])
		}
		deltas := make([][]float64, rows)
		maxAbs := 0.0
		changed := 0
		for r := 0; r < rows; r++ {
			deltas[r] = make([]float64, cols)
			for c := 0; c < cols; c++ {
				d := round4(resp.GridB.Values[r][c] - resp.GridA.Values[r][c])
				deltas[r][c] = d
				if math.Abs(d) > 1e-5 {
					changed++
				}
				if math.Abs(d) > maxAbs {
					maxAbs = math.Abs(d)
				}
			}
		}
		resp.DeltaValues = deltas
		resp.MaxAbsDelta = round4(maxAbs)
		resp.ChangedCells = changed
	}

	return resp, nil
}

func decodeMapFromBin(module string, rec CalMapRecord, binPath string) (*DecodedMapGrid, error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return nil, err
	}

	fileOff := mapAddrToFileOffset(module, rec.AddressInt, len(data))
	if fileOff < 0 || fileOff >= len(data) {
		return nil, fmt.Errorf("address 0x%X out of range for %s", rec.AddressInt, filepath.Base(binPath))
	}

	rows := rec.Rows
	cols := rec.Cols
	if rows <= 0 {
		rows = 1
	}
	if cols <= 0 {
		cols = 1
	}
	if rows > 64 {
		rows = 64
	}
	if cols > 64 {
		cols = 64
	}

	scale := rec.ScaleFactor
	if scale == 0 {
		scale = 1.0
	}
	offset := rec.OffsetVal

	isBE := strings.ToLower(module) == "tcu"
	elemType := strings.ToLower(rec.ElementType)

	// For TCU descriptor-based tables (e.g. 14x20 shift maps or 8x8 clutch maps), advance past descriptor header if needed
	dataOff := fileOff
	if isBE && rows == 14 && cols == 20 && dataOff+36+280 <= len(data) {
		// Check if descriptor header starts with 0x000E 0x0014
		w0 := readU16BE(data, dataOff)
		w1 := readU16BE(data, dataOff+2)
		if (w0 == 14 && w1 == 20) || (w0 == 20 && w1 == 14) {
			dataOff += 36
		}
	}

	values := make([][]float64, rows)
	minVal := 1e18
	maxVal := -1e18

	for r := 0; r < rows; r++ {
		values[r] = make([]float64, cols)
		for c := 0; c < cols; c++ {
			idx := r*cols + c
			var raw float64
			switch {
			case strings.Contains(elemType, "int8") && !strings.Contains(elemType, "u"):
				raw = float64(int8(readU8(data, dataOff+idx)))
			case strings.Contains(elemType, "8"):
				raw = float64(readU8(data, dataOff+idx))
			case strings.Contains(elemType, "int16") && !strings.Contains(elemType, "u"):
				if isBE {
					raw = float64(int16(readU16BE(data, dataOff+idx*2)))
				} else {
					raw = float64(int16(readU16LE(data, dataOff+idx*2)))
				}
			default: // uint16
				if isBE {
					raw = float64(readU16BE(data, dataOff+idx*2))
				} else {
					raw = float64(readU16LE(data, dataOff+idx*2))
				}
			}
			val := round4(raw*scale + offset)
			values[r][c] = val
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}
	}

	xAxis := make([]float64, cols)
	for c := 0; c < cols; c++ {
		xAxis[c] = float64(c)
	}
	yAxis := make([]float64, rows)
	for r := 0; r < rows; r++ {
		yAxis[r] = float64(r)
	}

	return &DecodedMapGrid{
		BinName:    filepath.Base(binPath),
		BinPath:    binPath,
		FileOffset: dataOff,
		XAxis:      xAxis,
		YAxis:      yAxis,
		Values:     values,
		MinVal:     minVal,
		MaxVal:     maxVal,
	}, nil
}

func mapAddrToFileOffset(module string, addr uint64, fileLen int) int {
	switch strings.ToLower(module) {
	case "ecu":
		if addr >= 0x80800000 && addr < 0x80A00000 {
			return int(addr - 0x80800000 + 0x200000)
		}
		if addr >= 0x80000000 && addr < 0x80400000 {
			return int(addr - 0x80000000)
		}
		if int(addr) < fileLen {
			return int(addr)
		}
	case "tcu":
		base := uint64(0x1C0000)
		if fileLen == 785408 {
			base = 0x1C0200
		}
		if addr >= base && int(addr-base) < fileLen {
			return int(addr - base)
		}
		if int(addr) < fileLen {
			return int(addr)
		}
	case "awd":
		base := uint64(0xC00000)
		if fileLen == 32768 {
			base = 0xC04000
		}
		if addr >= base && int(addr-base) < fileLen {
			return int(addr - base)
		}
		if int(addr) < fileLen {
			return int(addr)
		}
	}
	return -1
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
