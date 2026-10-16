package main

import (
	"fmt"
	"math"
	"strings"
)

// Standard 8AT A8LR1 gear ratios and final drive for Stinger GT / G70 3.3T AWD
var (
	GearRatiosA8LR1 = [8]float64{3.964, 2.468, 1.610, 1.176, 1.000, 0.832, 0.652, 0.565}
	FinalDriveA8LR1 = 3.538
)

// GetSurfacePresets returns the built-in road surface Pacejka friction models.
func GetSurfacePresets() []SurfacePreset {
	return []SurfacePreset{
		{
			ID:          "drag_vht",
			Name:        "Drag Strip (VHT Prepped Rubber)",
			Description: "High-bite VHT prepped drag strip surface with drag radials / warm sticky compound.",
			MuPeak:      1.35,
			MuSlide:     1.06,
			StiffnessB:  10.5,
			ShapeC:      1.55,
			CurvatureE:  0.45,
			KappaPeak:   0.14,
			RollingRes:  0.016,
		},
		{
			ID:          "dry_warm",
			Name:        "Dry Asphalt (Warm 200TW Street/Track)",
			Description: "Clean, warm dry tarmac with Michelin PS4S / 200TW summer performance tires.",
			MuPeak:      1.08,
			MuSlide:     0.84,
			StiffnessB:  11.8,
			ShapeC:      1.62,
			CurvatureE:  0.55,
			KappaPeak:   0.11,
			RollingRes:  0.013,
		},
		{
			ID:          "cold_street",
			Name:        "Cold Unprepped Street Asphalt",
			Description: "Cool ambient street tarmac where high-torque launches easily break rear traction.",
			MuPeak:      0.86,
			MuSlide:     0.66,
			StiffnessB:  12.5,
			ShapeC:      1.68,
			CurvatureE:  0.65,
			KappaPeak:   0.09,
			RollingRes:  0.013,
		},
		{
			ID:          "wet",
			Name:        "Wet Asphalt (Rain / Damp)",
			Description: "Damp/wet road surface with strong hydro-dynamic drop-off past peak slip.",
			MuPeak:      0.66,
			MuSlide:     0.46,
			StiffnessB:  14.0,
			ShapeC:      1.75,
			CurvatureE:  0.78,
			KappaPeak:   0.08,
			RollingRes:  0.015,
		},
		{
			ID:          "snow",
			Name:        "Packed Snow / Winter Surface",
			Description: "Low-mu winter surface highlighting AWD coupling split and TCS slip intervention.",
			MuPeak:      0.34,
			MuSlide:     0.23,
			StiffnessB:  16.0,
			ShapeC:      1.80,
			CurvatureE:  0.85,
			KappaPeak:   0.06,
			RollingRes:  0.020,
		},
	}
}

// ResolveSurface returns the active SurfacePreset with any custom user overrides applied.
func ResolveSurface(env EnvironmentConfig) SurfacePreset {
	presets := GetSurfacePresets()
	chosen := presets[1] // default dry_warm
	for _, p := range presets {
		if p.ID == env.SurfaceID {
			chosen = p
			break
		}
	}
	if env.CustomMuPeak > 0.1 {
		scale := env.CustomMuPeak / chosen.MuPeak
		chosen.MuPeak = env.CustomMuPeak
		if env.CustomMuSlide > 0.05 {
			chosen.MuSlide = env.CustomMuSlide
		} else {
			chosen.MuSlide = round2(chosen.MuSlide * scale)
		}
	} else if env.CustomMuSlide > 0.05 {
		chosen.MuSlide = env.CustomMuSlide
	}
	if env.CustomKappaPeak > 0.02 && env.CustomKappaPeak < 0.40 {
		chosen.KappaPeak = env.CustomKappaPeak
	}
	return chosen
}

// EvalMuSlip evaluates the longitudinal tire friction coefficient mu(kappa) using a normalized Pacejka Magic Formula.
func EvalMuSlip(surf SurfacePreset, kappa float64) float64 {
	absK := math.Abs(kappa)
	if absK < 1e-6 {
		return 0.0
	}
	sign := 1.0
	if kappa < 0 {
		sign = -1.0
	}

	kPeak := surf.KappaPeak
	if kPeak < 0.03 {
		kPeak = 0.10
	}

	// Normalized slip coordinate x = absK / kPeak so peak occurs accurately at kPeak
	x := absK / kPeak
	var mu float64
	if x <= 1.0 {
		// Rising branch up to peak D = MuPeak at x = 1.0
		// Sin(pi/2 * x^(0.85)) gives realistic initial tire carcass stiffness + rounded peak
		mu = surf.MuPeak * math.Sin(0.5*math.Pi*math.Pow(x, 0.85))
	} else {
		// Post-peak decay toward MuSlide asymptote
		decay := math.Exp(-0.85 * (x - 1.0))
		mu = surf.MuSlide + (surf.MuPeak-surf.MuSlide)*decay
	}
	return sign * mu
}

// GenerateMuSlipCurve returns 61 points (0..60% slip) for the UI tire friction chart.
func GenerateMuSlipCurve(surf SurfacePreset) []MuSlipCurvePoint {
	pts := make([]MuSlipCurvePoint, 61)
	for i := 0; i <= 60; i++ {
		slipPct := float64(i)
		pts[i] = MuSlipCurvePoint{
			SlipPct: slipPct,
			Mu:      round3(EvalMuSlip(surf, slipPct/100.0)),
		}
	}
	return pts
}

// NormalizeEnvironment fills in any zero-value chassis/atmospheric parameters with verified Stinger GT / G70 3.3T defaults.
func NormalizeEnvironment(env EnvironmentConfig) EnvironmentConfig {
	if env.SurfaceID == "" {
		env.SurfaceID = "dry_warm"
	}
	if env.AmbientTempC == 0 {
		env.AmbientTempC = 20.0
	}
	if env.CoolantTempC == 0 {
		env.CoolantTempC = 85.0
	}
	if env.VehicleMassKg < 1000 {
		env.VehicleMassKg = 1915.0 // Curb weight + driver
	}
	if env.CGHeightM < 0.2 {
		env.CGHeightM = 0.51
	}
	if env.StaticFrontPct < 30 || env.StaticFrontPct > 70 {
		env.StaticFrontPct = 52.0
	}
	if env.FrontTireRadius < 0.25 {
		env.FrontTireRadius = 0.334 // 225/40R19
	}
	if env.RearTireRadius < 0.25 {
		env.RearTireRadius = 0.334 // Matched by default
	}
	if env.SimDurationSec < 5.0 || env.SimDurationSec > 35.0 {
		env.SimDurationSec = 16.0
	}
	return env
}

// NormalizeRunConfig fills in default launch & pedal parameters if omitted.
func NormalizeRunConfig(cfg RunConfig) RunConfig {
	if cfg.FuelType == "" {
		if strings.Contains(strings.ToLower(cfg.ECUBinPath), "bef") {
			cfg.FuelType = "e30"
		} else {
			cfg.FuelType = "93_aki"
		}
	}
	if cfg.DriveMode == "" {
		cfg.DriveMode = "sport"
	}
	if cfg.ESCMode == "" {
		cfg.ESCMode = "sport"
	}
	if cfg.PreStageTimeSec <= 0 {
		cfg.PreStageTimeSec = 1.4
	}
	if cfg.BrakeHoldPsi <= 0 {
		cfg.BrakeHoldPsi = 1200.0
	}
	if cfg.BrakeReleaseMs <= 0 {
		cfg.BrakeReleaseMs = 130.0
	}
	if cfg.PedalPct <= 0 {
		cfg.PedalPct = 100.0
	}
	if cfg.Overrides.TorqueScalePct <= 0 {
		cfg.Overrides.TorqueScalePct = 100.0
	}
	if cfg.Overrides.ShiftCutScalePct <= 0 {
		cfg.Overrides.ShiftCutScalePct = 100.0
	}
	if cfg.Overrides.FrontSplitScalePct <= 0 {
		cfg.Overrides.FrontSplitScalePct = 100.0
	}
	return cfg
}

type fuelThermodynamics struct {
	OctaneSparkOffsetDeg float64 // Knock threshold offset relative to 93 AKI pump gas (deg BTDC)
	LatentChargeCoolC    float64 // Effective cylinder intake charge cooling from fuel vaporization (deg C)
	CombustionEnergyMult float64 // Stoichiometric mass flow / flame speed torque factor at WOT
	CommandedLambda      float64 // Target WOT equivalence ratio lambda
}

func evalFuelThermodynamics(fuelType string) fuelThermodynamics {
	switch strings.ToLower(strings.TrimSpace(fuelType)) {
	case "91_aki":
		return fuelThermodynamics{OctaneSparkOffsetDeg: -2.6, LatentChargeCoolC: 0.0, CombustionEnergyMult: 0.992, CommandedLambda: 0.80}
	case "93_wmi":
		return fuelThermodynamics{OctaneSparkOffsetDeg: +3.8, LatentChargeCoolC: 8.5, CombustionEnergyMult: 1.022, CommandedLambda: 0.81}
	case "e30":
		return fuelThermodynamics{OctaneSparkOffsetDeg: +4.5, LatentChargeCoolC: 6.5, CombustionEnergyMult: 1.030, CommandedLambda: 0.83}
	case "e50":
		return fuelThermodynamics{OctaneSparkOffsetDeg: +5.8, LatentChargeCoolC: 9.5, CombustionEnergyMult: 1.044, CommandedLambda: 0.84}
	case "e85":
		return fuelThermodynamics{OctaneSparkOffsetDeg: +7.2, LatentChargeCoolC: 13.0, CombustionEnergyMult: 1.058, CommandedLambda: 0.85}
	default: // "93_aki"
		return fuelThermodynamics{OctaneSparkOffsetDeg: 0.0, LatentChargeCoolC: 0.0, CombustionEnergyMult: 1.000, CommandedLambda: 0.82}
	}
}

// evalDualCVVT models the Lambda II G6DP Dual-CVVT intake advance (0..+30 deg CA) and exhaust retard (-20..0 deg CA).
// Returns intakeCamDeg, exhaustCamDeg, overlapDeg, spoolTauMult, and veCamMult.
func evalDualCVVT(rpm, airloadMg, overlapAdderDeg float64) (float64, float64, float64, float64, float64) {
	loadFrac := math.Max(0.0, math.Min(1.0, (airloadMg-250.0)/900.0))

	// Base intake advance schedule across RPM (scavenging peak between 1600-3500 RPM, retards at high RPM for top-end breathing)
	var baseInDeg, baseExDeg float64
	switch {
	case rpm < 1400:
		baseInDeg = 8.0 + 10.0*((rpm-800.0)/600.0)
		baseExDeg = -4.0
	case rpm <= 3600:
		// Mid-range scavenging window: high intake advance + exhaust retard creates positive valve overlap
		u := (rpm - 1400.0) / 2200.0
		baseInDeg = 20.0 + 6.0*math.Sin(u*math.Pi)
		baseExDeg = -8.0 - 5.0*math.Sin(u*math.Pi)
	default:
		// High RPM power band: taper overlap to reduce exhaust backpressure reversion
		u := math.Min(1.0, (rpm-3600.0)/2900.0)
		baseInDeg = 20.0 - 11.5*u
		baseExDeg = -8.0 + 6.0*u
	}

	camInDeg := math.Max(0.0, math.Min(34.0, baseInDeg*loadFrac+0.55*overlapAdderDeg))
	camExDeg := math.Max(-24.0, math.Min(2.0, baseExDeg*loadFrac-0.45*overlapAdderDeg))
	overlapDeg := math.Max(0.0, camInDeg-camExDeg)

	// Scavenging overlap between 1400-3800 RPM accelerates twin Garrett MGT1549DSL spool
	spoolTauMult := 1.0
	if rpm >= 1300 && rpm <= 4000 {
		spoolBonus := (overlapDeg - 18.0) * 0.012
		spoolTauMult = math.Max(0.76, math.Min(1.18, 1.0-spoolBonus))
	}

	// Volumetric efficiency modifier from cam phasing:
	// Extra overlap helps low/mid-RPM scavenging (<4200 RPM) but excessive overlap above 5000 RPM causes exhaust reversion
	veCamMult := 1.0
	if rpm <= 4200 {
		veCamMult = 1.0 + 0.0022*overlapAdderDeg
	} else {
		// Above 4200 RPM, slight negative overlap bias (retarded intake / advanced exhaust) improves top-end VE
		veCamMult = 1.0 - 0.0018*overlapAdderDeg
	}
	veCamMult = math.Max(0.93, math.Min(1.06, veCamMult))

	return camInDeg, camExDeg, overlapDeg, spoolTauMult, veCamMult
}

// ExecuteSimulation runs Run A (and optionally Run B) and computes the full A/B comparison diff.
func ExecuteSimulation(req SimRequest) (*SimResponse, error) {
	env := NormalizeEnvironment(req.Environment)
	surf := ResolveSurface(env)

	runACfg := NormalizeRunConfig(req.RunA)
	if runACfg.Label == "" {
		runACfg.Label = "Run A (Baseline)"
	}
	resA, err := simulateSingleRun(runACfg, env, surf)
	if err != nil {
		return nil, fmt.Errorf("Run A simulation error: %w", err)
	}

	resp := &SimResponse{
		Environment: env,
		Surface:     surf,
		MuSlipCurve: GenerateMuSlipCurve(surf),
		RunA:        *resA,
	}

	if req.RunB != nil && req.RunB.ECUBinPath != "" {
		runBCfg := NormalizeRunConfig(*req.RunB)
		if runBCfg.Label == "" {
			runBCfg.Label = "Run B (Comparison)"
		}
		resB, err := simulateSingleRun(runBCfg, env, surf)
		if err != nil {
			return nil, fmt.Errorf("Run B simulation error: %w", err)
		}
		resp.RunB = resB
		resp.Comparison = buildRunDiff(resA, resB)
	}

	return resp, nil
}

// simulateSingleRun executes the 1ms / 10ms / 20ms multi-rate physics & firmware emulation loop.
func simulateSingleRun(cfg RunConfig, env EnvironmentConfig, surf SurfacePreset) (*RunResult, error) {
	ecu, err := DecodeECUProfile(cfg.ECUBinPath)
	if err != nil {
		return nil, err
	}
	tcu, err := DecodeTCUProfile(cfg.TCUBinPath)
	if err != nil {
		return nil, err
	}
	awd, err := DecodeAWDProfile(cfg.AWDBinPath)
	if err != nil {
		return nil, err
	}

	// Determine effective drive mode (accounting for 0x200CCE ECU force switch)
	effDriveMode := strings.ToLower(cfg.DriveMode)
	if ecu.DriveModeForce == 1 {
		effDriveMode = "sport"
	} else if ecu.DriveModeForce == 2 {
		effDriveMode = "custom_m4"
	}

	calSummary := BuildExtractedCalSummary(ecu, tcu, awd, effDriveMode)

	// Apply live user overrides to calibration summary & runtime variables
	shiftSparkFloor := ecu.ShiftSparkFloor
	if cfg.Overrides.ShiftSparkFloorDeg != nil {
		shiftSparkFloor = *cfg.Overrides.ShiftSparkFloorDeg
		calSummary.ECUShiftSparkFloorDeg = round2(shiftSparkFloor)
	}

	tcsInhibit := ecu.TCSInhibit
	if cfg.Overrides.TCSInhibit != nil {
		tcsInhibit = *cfg.Overrides.TCSInhibit
		calSummary.ECUTCSInhibit = tcsInhibit
	}

	tchaActive := ecu.TCHALimitActive
	if cfg.Overrides.TCHASurgeFix != nil && *cfg.Overrides.TCHASurgeFix {
		tchaActive = false
		calSummary.ECUTCHALimitActive = false
	}

	ralActive := ecu.RALPatchDetected
	if cfg.Overrides.RALAntilagActive != nil {
		ralActive = *cfg.Overrides.RALAntilagActive
		calSummary.ECURALPatchDetected = ralActive
	}

	stallRPM := tcu.LaunchStallRPM
	if cfg.Overrides.LaunchStallRPM != nil && *cfg.Overrides.LaunchStallRPM >= 1200 {
		stallRPM = *cfg.Overrides.LaunchStallRPM
		calSummary.TCULaunchStallRPM = stallRPM
	}

	// Effective TCU continuous torque ceiling
	// Stock CPTSH has 0x1FD198 = 350 Nm, 0x2096E4 = 400 Nm, and 8x8 Clutch Torque Limit = 485 Nm.
	// In real vehicle operation, stock TCU begins limiting continuous engine torque via CAN 0x162 around 515 Nm,
	// whereas EKM tuned (700 Nm / 32767 Nm) removes the ceiling completely.
	tcuContCapNm := 515.0
	if tcu.PrimaryTqCapNm >= 600 || tcu.GlobalTqCapNm >= 1000 {
		tcuContCapNm = 950.0
	}
	if cfg.Overrides.TorqueCapNm != nil && *cfg.Overrides.TorqueCapNm > 150 {
		tcuContCapNm = *cfg.Overrides.TorqueCapNm
		calSummary.TCUPrimaryTorqueCapNm = tcuContCapNm
	}

	bypassBrakeGate := awd.BrakeGateBypassed
	if cfg.Overrides.BypassBrakeGate != nil {
		bypassBrakeGate = *cfg.Overrides.BypassBrakeGate
		calSummary.AWDBrakeGateBypassed = bypassBrakeGate
	}

	awdSlewPer20ms := awd.SlewRateNmPer20ms
	if cfg.Overrides.AWDSlewRateNmPerS != nil && *cfg.Overrides.AWDSlewRateNmPerS > 500 {
		awdSlewPer20ms = (*cfg.Overrides.AWDSlewRateNmPerS) / 50.0
		calSummary.AWDSlewRateNmPer20ms = round2(awdSlewPer20ms)
		calSummary.AWDSlewRateNmPerSec = round2(*cfg.Overrides.AWDSlewRateNmPerS)
	}

	// Physical constants
	const (
		gGrav      = 9.80665
		wheelbaseM = 2.906
		cdAero     = 0.30
		frontalM2  = 2.28
		jEngine    = 0.24 // Engine + TC pump inertia (kg*m^2)
		jFrontAxle = 2.35 // Front wheels + halfshafts inertia (kg*m^2)
		jRearAxle  = 2.65 // Rear wheels + diff + driveshaft inertia (kg*m^2)
		dt         = 0.001 // 1 ms integration step
	)

	mass := env.VehicleMassKg
	rFront := env.FrontTireRadius
	rRear := env.RearTireRadius
	lf := wheelbaseM * (1.0 - env.StaticFrontPct/100.0)
	lr := wheelbaseM * (env.StaticFrontPct / 100.0)
	gradeRad := math.Atan(env.RoadGradePct / 100.0)

	// Air density rho (kg/m^3) adjusted for altitude and ambient temperature
	pAmbPa := 101325.0 * math.Pow(1.0-2.25577e-5*env.AltitudeM, 5.25588)
	tAmbK := env.AmbientTempC + 273.15
	rhoAir := pAmbPa / (287.05 * tAmbK)
	ambientHpa := pAmbPa / 100.0

	// Shift schedule selection (7 WOT upshift speeds in km/h)
	baseUpshiftKmh := make([]float64, 7)
	for i := 0; i < 7; i++ {
		if effDriveMode == "comfort" || effDriveMode == "eco" {
			baseUpshiftKmh[i] = Interp1D(tcu.PedalAxis20, extractCol(tcu.ComfortGrid, i), cfg.PedalPct)
		} else {
			baseUpshiftKmh[i] = Interp1D(tcu.PedalAxis20, extractCol(tcu.SportGrid, i), cfg.PedalPct)
		}
		baseUpshiftKmh[i] += cfg.Overrides.ShiftSpeedDeltaKm
	}
	calSummary.TCUWOTUpshiftKmh = append([]float64(nil), baseUpshiftKmh...)

	// Base shift inertia duration (ms) derived from 0x20FC52 & 0x24BB22 VCM bleed
	baseShiftDurMs := 155.0
	if tcu.ShiftTimerMs <= 120 || tcu.VCMSolenoidFloor <= 550 {
		baseShiftDurMs = 95.0 // EKM fast shift
	}
	if cfg.Overrides.ShiftDurationMs != nil && *cfg.Overrides.ShiftDurationMs >= 40 {
		baseShiftDurMs = *cfg.Overrides.ShiftDurationMs
	}

	// State variables
	var (
		tSec          = -cfg.PreStageTimeSec
		vCarMs        = 0.0
		distM         = 0.0
		rolloutPassed = (cfg.RolloutInch <= 0)
		rolloutDistM  = cfg.RolloutInch * 0.0254
		clockStartSec = 0.0
		clockStarted  = false

		gear         = 1
		engineRPM    = 850.0
		omegaFront   = 0.0
		omegaRear    = 0.0
		boostHpa     = ambientHpa
		boostTargetPsi = 0.0
		airloadMg    = 180.0
		accelG       = 0.0
		prevAccelG   = 0.0

		// Fuel & Dual-CVVT state
		fuelSpec       = evalFuelThermodynamics(cfg.FuelType)
		camInDeg       = 8.0
		camExDeg       = -4.0
		camOverlapDeg  = 12.0
		krDeg          = 0.0

		// ECU/TCU 10ms state
		tqiDrvNm       = 0.0
		tqiUncappedNm  = 0.0
		tcuLimitNm     = tcuContCapNm
		actualEngNm    = 0.0
		igaBaseDeg     = 12.0
		igaActualDeg   = 12.0
		igaFloorDeg    = shiftSparkFloor
		etaIga         = 1.0
		tcsActive      = false
		tcsCutNm       = 0.0
		tchaActiveFlag = false
		tchaOverCycles = 0
		tchaCapNm      = 2000.0

		// Shift state machine
		shiftActive      = false
		shiftElapsedMs   = 0.0
		shiftTargetGear  = 1
		curShiftEvent    *ShiftEvent
		completedShifts  []ShiftEvent

		// AWD 20ms state
		awdTargetNm   = 0.0
		awdActualNm   = 0.0
		awdSplitPct   = 40.0
		awdBrakeGated = false

		// Telemetry & Summary trackers
		telemetry []TelemetrySample
		summary   PerformanceSummary
	)

	totalSteps := int((cfg.PreStageTimeSec + env.SimDurationSec) / dt)
	for step := 0; step <= totalSteps; step++ {
		tSec = -cfg.PreStageTimeSec + float64(step)*dt
		vKmh := vCarMs * 3.6
		vMph := vCarMs * 2.23693629

		// 1. Brake Pressure Profile (psi & bar)
		var brakePsi float64
		if tSec < 0 {
			brakePsi = cfg.BrakeHoldPsi
		} else {
			relSec := cfg.BrakeReleaseMs / 1000.0
			if tSec < relSec {
				// Smooth cubic hydraulic pressure bleed
				u := tSec / relSec
				brakePsi = cfg.BrakeHoldPsi * (1.0 - u*u*(3.0-2.0*u))
			} else {
				brakePsi = 0.0
			}
		}
		brakeBar := brakePsi * 0.0689476

		// 2. Turbine RPM from current gear & rear axle angular speed
		gearRatio := GearRatiosA8LR1[gear-1]
		overallRatio := gearRatio * FinalDriveA8LR1
		turbineRPM := (omegaRear * overallRatio) * 60.0 / (2.0 * math.Pi)

		// 3. 20ms Magna ATC AWD Controller Task (every 20 steps)
		if step%20 == 0 {
			// Evaluate Launch Preload (0xC04778 Sport / 0xC046D8 Eco)
			var launchPreload float64
			if gear <= 2 && vKmh < 35.0 {
				if effDriveMode == "eco" {
					launchPreload = Interp1D(awd.LaunchPedalAxis, awd.LaunchPreloadEco[gear], cfg.PedalPct)
				} else {
					launchPreload = Interp1D(awd.LaunchPedalAxis, awd.LaunchPreloadSport[gear], cfg.PedalPct)
				}
				if cfg.Overrides.LaunchPreloadNm != nil && gear == 1 {
					launchPreload = *cfg.Overrides.LaunchPreloadNm
				}
				// Taper launch preload smoothly into feed-forward above 22 km/h
				if vKmh > 20.0 {
					launchPreload *= math.Max(0.0, 1.0-(vKmh-20.0)/15.0)
				}
			}

			// Evaluate Off-Idle Base Preload (0xC04A3C)
			offIdleNm := awd.OffIdleNormalNm[3]
			if effDriveMode == "eco" {
				offIdleNm = awd.OffIdleEcoNm[3]
			}

			// Evaluate Speed x Pedal Feed-Forward Coupling Torque (0xC04BA0 Sport / 0xC04B3C Comfort)
			var ffTorque float64
			if effDriveMode == "comfort" || effDriveMode == "eco" {
				ffTorque = Interp2D(awd.FFComfortSpeedKmh, awd.FFPedalAxis, awd.FFComfortMapNm, vKmh, cfg.PedalPct)
			} else {
				ffTorque = Interp2D(awd.FFSportSpeedKmh, awd.FFPedalAxis, awd.FFSportMapNm, vKmh, cfg.PedalPct)
			}
			if cfg.Overrides.Hold1000NmToKmh != nil && *cfg.Overrides.Hold1000NmToKmh > 0 {
				holdKmh := *cfg.Overrides.Hold1000NmToKmh
				if vKmh <= holdKmh && cfg.PedalPct >= 80 {
					ffTorque = math.Max(ffTorque, 1000.0)
				}
			}

			// Evaluate Front Torque Split % (0xC05922 Sport / 0xC0585A Comfort / 0xC058F0 Mod)
			var splitPct float64
			if effDriveMode == "comfort" || effDriveMode == "eco" {
				splitPct = Interp2D(awd.SplitSpeedAxisKmh, awd.SplitPedalAxis, awd.SplitComfortPct, vKmh, cfg.PedalPct)
			} else if bypassBrakeGate && awd.SplitMod38Pct[0][4] > awd.SplitSportPct[0][4] {
				splitPct = Interp2D(awd.SplitSpeedAxisKmh, awd.SplitPedalAxis, awd.SplitMod38Pct, vKmh, cfg.PedalPct)
			} else {
				splitPct = Interp2D(awd.SplitSpeedAxisKmh, awd.SplitPedalAxis, awd.SplitSportPct, vKmh, cfg.PedalPct)
			}

			// Apply High-Speed Gate 2 WOT dynamic split taper (0xC056CA on 0xC06F96) above 80 km/h
			if vKmh > 80.0 {
				gate2Split := Interp1D(awd.HighSpdGate2Axis, awd.HighSpdGate2Split, vKmh)
				blend := math.Min(1.0, (vKmh-80.0)/40.0)
				splitPct = (1.0-blend)*splitPct + blend*gate2Split
			}

			// Apply Gate 3 Master Speed Taper Ceiling (0xC0640A on 0xC06252)
			masterCapPct := Interp1D(awd.MasterTaperAxisKmh, awd.MasterTaperPct, vKmh)
			if splitPct > masterCapPct && !bypassBrakeGate {
				splitPct = masterCapPct
			}

			splitPct *= (cfg.Overrides.FrontSplitScalePct / 100.0)
			splitPct = math.Max(0.0, math.Min(50.0, splitPct))
			awdSplitPct = splitPct

			// Closed-loop wheel slip feedback (0xC061FA & 0xC06978)
			// Note: AWD controller computes wheel linear speeds using its hardcoded 0xC04214 R_TIRE (320 mm),
			// whereas physical wheel angular speeds omegaFront/omegaRear depend on physical rFront / rRear!
			vRearEstKmh := omegaRear * rRear * 3.6
			vFrontEstKmh := omegaFront * rFront * 3.6
			slipDeltaKmh := math.Abs(vRearEstKmh - vFrontEstKmh)
			deadband := awd.SlipDeadbandKmh
			if vKmh > 100.0 {
				deadband = 2.0
			}
			var slipFeedbackNm float64
			if slipDeltaKmh > deadband {
				slipFeedbackNm = Interp1D(awd.SlipCurveDeltaKmh, awd.SlipCurveTorqueNm, slipDeltaKmh-deadband)
			}

			// Combine open-loop preload/feed-forward, split demand, and closed-loop slip feedback
			propDemandNm := math.Max(0, actualEngNm) * gearRatio * (splitPct / 100.0)
			rawTargetNm := math.Max(math.Max(launchPreload, offIdleNm), math.Max(ffTorque*(splitPct/40.0), propDemandNm)) + slipFeedbackNm

			// Apply Max Ceiling (0xC059EA / 0xC05A1C)
			maxCeiling := awd.CeilingSportNm
			if effDriveMode == "comfort" || effDriveMode == "eco" {
				maxCeiling = awd.CeilingComfortNm
			}
			if rawTargetNm > maxCeiling {
				rawTargetNm = maxCeiling
			}

			// Apply Brake Cylinder Pressure Gate (0xC07498 / 0xC07606 = 4.45 bar = 64.5 psi)
			if brakeBar > awd.BrakeGateQ88Bar && !bypassBrakeGate {
				awdBrakeGated = true
				awdTargetNm = 0.0
			} else {
				awdBrakeGated = false
				awdTargetNm = rawTargetNm
			}

			// Apply Positive Slew Rate Limiter (0xC07650: Nm per 20ms cycle)
			if awdTargetNm > awdActualNm {
				awdActualNm = math.Min(awdTargetNm, awdActualNm+awdSlewPer20ms)
			} else {
				// Fast hydraulic release rate
				awdActualNm = math.Max(awdTargetNm, awdActualNm-awdSlewPer20ms*2.2)
			}
		}

		// 4. 10ms SIM2K-260 ECU & Kefico CPTSH TCU Task (every 10 steps)
		if step%10 == 0 {
			// 4a. Driver Demand Torque (0x214280 Comfort, 0x214760 Mode 4, 0x214E80 Sport)
			switch effDriveMode {
			case "comfort", "eco":
				tqiDrvNm = Interp2D(ecu.DrvRPM, ecu.DrvPedal, ecu.DrvMapComfort, engineRPM, cfg.PedalPct)
			case "custom_m4":
				tqiDrvNm = Interp2D(ecu.DrvRPM, ecu.DrvPedal, ecu.DrvMapMode4, engineRPM, cfg.PedalPct)
			default:
				tqiDrvNm = Interp2D(ecu.DrvRPM, ecu.DrvPedal, ecu.DrvMapSport, engineRPM, cfg.PedalPct)
			}
			// Apply Gear & Speed torque limit factors (0x213B70 & 0x215150)
			gearFac := Interp2D(ecu.GearAxis, ecu.GearPedalAxis, ecu.GearFactorMap, float64(gear), cfg.PedalPct)
			spdFac := Interp2D(ecu.SpdAxis, ecu.SpdPedalAxis, ecu.SpdFactorMap, vKmh, cfg.PedalPct)
			if gearFac > 0.05 && gearFac < 1.0 {
				tqiDrvNm *= gearFac
			}
			if spdFac > 0.05 && spdFac < 1.0 {
				tqiDrvNm *= spdFac
			}

			// 4b. Turbocharger Boost, Dual-CVVT Cam Phasing & Airload Dynamics
			targetBoostHpa := ecu.PutSpMaxHpa
			if ecu.BoostLimitAHpa < targetBoostHpa && ecu.BoostLimitAHpa > 1200 {
				targetBoostHpa = ecu.BoostLimitAHpa
			}
			// Scale boost/airflow by user TorqueScalePct override and direct BoostAdderPsi (+1 psi = 68.9476 hPa)
			targetBoostHpa = ambientHpa + (targetBoostHpa-ambientHpa)*(cfg.Overrides.TorqueScalePct/100.0) + cfg.Overrides.BoostAdderPsi*68.9476
			targetBoostHpa = math.Max(ambientHpa, math.Min(3600.0, targetBoostHpa))
			boostTargetPsi = (targetBoostHpa - ambientHpa) * 0.0145038

			// Evaluate Dual-CVVT intake advance, exhaust retard, scavenging overlap, and spool/VE modifiers
			var spoolTauMult, veCamMult float64
			camInDeg, camExDeg, camOverlapDeg, spoolTauMult, veCamMult = evalDualCVVT(engineRPM, airloadMg, cfg.Overrides.CamOverlapAdderDeg)

			// Spool capability as a function of RPM & pre-stage brake-stand / RAL antilag
			spoolFactor := math.Min(1.0, math.Max(0.15, (engineRPM-1100.0)/1650.0))
			if tSec < 0 {
				if ralActive {
					spoolFactor = 1.0 // Rolling/Stationary Antilag builds 100% boost target on the line
				} else {
					// Converter stall load builds boost proportional to stall RPM
					spoolFactor = math.Min(0.92, math.Max(0.25, (engineRPM-1350.0)/2250.0))
				}
			}
			achievableBoostHpa := ambientHpa + (targetBoostHpa-ambientHpa)*spoolFactor*(cfg.PedalPct/100.0)
			// First-order turbo lag filter (tau ~ 170 ms in boost, accelerated by Dual-CVVT scavenging & RAL)
			tauSpool := 0.17 * spoolTauMult
			if ralActive {
				tauSpool = 0.08
			}
			boostHpa += (0.01 / tauSpool) * (achievableBoostHpa - boostHpa)

			// Convert manifold pressure + Dual-CVVT volumetric efficiency + fuel latent charge cooling to cylinder air charge (mg/stk)
			ve := 0.90
			if engineRPM >= 2000 && engineRPM <= 5400 {
				ve = 1.00
			} else if engineRPM > 5400 {
				ve = 1.00 - 0.07*((engineRPM-5400.0)/1200.0)
			}
			ve *= veCamMult
			effChargeTempK := math.Max(250.0, tAmbK-fuelSpec.LatentChargeCoolC)
			tempDensityFactor := 293.15 / effChargeTempK
			airloadMg = (boostHpa / 2717.0) * 1350.0 * ve * tempDensityFactor

			// 4c. Forward Torque Lookup (0x255518 TQI_BAS & 0x253D14.. TQI_MAX)
			tqiBas := Interp2D(ecu.FwdRPM, ecu.FwdAir, ecu.TQIBasA, engineRPM, airloadMg)
			tqiMax := Interp2D(ecu.FwdRPM, ecu.FwdAir, ecu.TQIMaxEnv, engineRPM, airloadMg)

			// Base & Optimum Ignition Lookup (0x238090 Base High & 0x238874 Opt High) + Live Spark Adder Override
			igaCommandedDeg := Interp2D(ecu.IgnRPM, ecu.IgnLoad, ecu.IgnBaseHigh, engineRPM, airloadMg) + cfg.Overrides.SparkAdvanceAdderDeg
			igaOptDeg := Interp2D(ecu.IgnRPM, ecu.IgnLoad, ecu.IgnOptHigh, engineRPM, airloadMg)
			igaMinMapDeg := Interp2D(ecu.IGAMinCoolant, ecu.IGAMinLoad, ecu.IGAMinHigh, env.CoolantTempC, airloadMg)

			// Evaluate Fuel Octane Knock Threshold & SIM2K-260 Knock Retard (krDeg)
			// 93 AKI supports up to (igaOptDeg - 8.8 deg) at 20C and 1250 mg/stk before knock sensor feedback pulls timing.
			knockCeilingDeg := (igaOptDeg - 8.8) + fuelSpec.OctaneSparkOffsetDeg - 0.11*(env.AmbientTempC-20.0) - 0.007*math.Max(0.0, airloadMg-1250.0)
			if igaCommandedDeg > knockCeilingDeg && airloadMg > 650.0 {
				krDeg = math.Min(9.0, 0.80*(igaCommandedDeg-knockCeilingDeg))
			} else {
				krDeg = math.Max(0.0, krDeg-0.35) // Gradual knock recovery
			}
			igaBaseDeg = igaCommandedDeg - krDeg

			// Calculate physical crankshaft brake torque vs. ECU internal model torque:
			// - Physical cylinder torque is governed by physical airload (tqiMax envelope), spark advance relative to MBT, and fuel energy multiplier.
			sparkAdvDeltaFromStock := igaBaseDeg - (igaOptDeg - 11.2)
			sparkTorqueBonus := 1.0 + 0.013*math.Max(-6.0, math.Min(9.0, sparkAdvDeltaFromStock))
			pumpingFrictionLossNm := 34.0 + 0.0045*engineRPM + 0.0000010*engineRPM*engineRPM
			physBrakeTorqueNm := math.Max(0.0, (tqiMax*0.92*sparkTorqueBonus*fuelSpec.CombustionEnergyMult)-pumpingFrictionLossNm)
			physBrakeTorqueNm *= (cfg.Overrides.TorqueScalePct / 100.0)

			tqiUncappedNm = math.Min(tqiDrvNm*(cfg.Overrides.TorqueScalePct/100.0)*fuelSpec.CombustionEnergyMult, physBrakeTorqueNm)

			// 4d. Loud-Tune TCHA Turbo Speed Limiter Bug (0x203EC6)
			if tchaActive && tSec >= 0.15 {
				// In Loud bin, inflated TQI_BAS + 0.75 speed factor (0x203ECC=49152) trips overspeed when model torque > 720 Nm
				modelTq := tqiBas * (cfg.Overrides.TorqueScalePct / 100.0)
				if modelTq > 710.0 && engineRPM > 2700.0 && !tchaActiveFlag {
					tchaOverCycles++
					if tchaOverCycles >= ecu.TCHADebounceCyc {
						tchaActiveFlag = true
						tchaCapNm = ecu.TCHAMinTorqueNm // Drops to 200 Nm (0x203ED4)
						summary.TCHACutsCount++
					}
				} else if tchaActiveFlag {
					// Ramps back up at 0x203ED8 (17 Nm per 10ms cycle)
					tchaCapNm += ecu.TCHARampNmPerCyc
					if tchaCapNm >= tqiUncappedNm {
						tchaActiveFlag = false
						tchaOverCycles = 0
					}
				} else {
					tchaOverCycles = 0
				}
			} else if !tchaActive {
				tchaActiveFlag = false
				tchaCapNm = 2000.0
			}

			// 4e. TCS Wheelspin Intervention (CAN 0x153 -> ECU 0x200B4B)
			if cfg.ESCMode != "off" && tcsInhibit == 0 && tSec > 0.04 && engineRPM > ecu.TCSMinRPM {
				vRef := math.Max(vCarMs, 2.5)
				slipF := (omegaFront*rFront - vCarMs) / vRef
				slipR := (omegaRear*rRear - vCarMs) / vRef
				maxSlip := math.Max(slipF, slipR)
				slipThresh := 0.13 // ESC On
				if cfg.ESCMode == "sport" {
					slipThresh = 0.18 // ESC Sport
				}
				if maxSlip > slipThresh {
					if !tcsActive {
						summary.TCSEventsCount++
					}
					tcsActive = true
					excess := maxSlip - slipThresh
					tcsCutNm = math.Min(tqiUncappedNm*0.65, excess*1600.0)
				} else {
					tcsActive = false
					tcsCutNm = math.Max(0.0, tcsCutNm-40.0)
				}
			} else {
				tcsActive = false
				tcsCutNm = 0.0
			}

			// 4f. TCU Shift Schedule & Shift State Machine
			// Enforce minimum in-gear dwell and verify vehicle road speed vKmh against CPTSH shift schedule
			lastShiftEndSec := 0.0
			if len(completedShifts) > 0 {
				last := completedShifts[len(completedShifts)-1]
				lastShiftEndSec = last.StartTimeSec + last.DurationMs/1000.0
			}
			if tSec >= 0.35 && (tSec-lastShiftEndSec) >= 0.42 && !shiftActive && gear < 8 {
				upSpeedKmh := baseUpshiftKmh[gear-1]
				if vKmh >= upSpeedKmh || (vKmh >= upSpeedKmh*0.86 && engineRPM >= 6450.0) {
					shiftActive = true
					shiftElapsedMs = 0.0
					shiftTargetGear = gear + 1
					nextRatio := GearRatiosA8LR1[shiftTargetGear-1]
					dropRPM := engineRPM * (nextRatio / gearRatio)
					curShiftEvent = &ShiftEvent{
						FromGear:          gear,
						ToGear:            shiftTargetGear,
						StartTimeSec:      round3(tSec),
						DurationMs:        round2(baseShiftDurMs),
						TriggerSpeedKmh:   round2(vKmh),
						TriggerSpeedMph:   round2(vMph),
						EntryRPM:          round2(engineRPM),
						DropToRPM:         round2(dropRPM),
						MinSparkDeg:       99.0,
						ActualMinTorqueNm: 9999.0,
						MinAccelG:         99.0,
					}
				}
			}

			// 4g. Evaluate TCU Torque Request & ECU Spark Efficiency Retard
			tcuLimitNm = tcuContCapNm
			if shiftActive {
				shiftElapsedMs += 10.0
				// Shift torque cut depth from 0x233D3C (-10 stock vs -15 EKM)
				cutSeverity := math.Abs(float64(tcu.ShiftCutRaw)) / 10.0
				if cutSeverity < 0.5 {
					cutSeverity = 1.0
				}
				cutSeverity *= (cfg.Overrides.ShiftCutScalePct / 100.0)
				// Target torque during inertia phase
				shiftReqNm := math.Max(65.0, tqiUncappedNm*(1.0-0.58*cutSeverity))
				tcuLimitNm = math.Min(tcuLimitNm, shiftReqNm)

				// ECU computes required spark efficiency eta_iga = T_req / T_uncapped
				etaReq := 1.0
				if tqiUncappedNm > 10.0 {
					etaReq = math.Max(0.09, math.Min(1.0, tcuLimitNm/tqiUncappedNm))
				}
				// Lookup required spark retard from 0x25743E -> 0x25262C
				reqRetardDeg := Interp1D(ecu.EffToRetardAxis, ecu.EffToRetardDeg, etaReq)
				unclampedIga := igaBaseDeg - reqRetardDeg

				// Clamp by max(IGA_MIN, 0x207AD9 Shift Spark Floor)
				igaFloorDeg = math.Max(igaMinMapDeg, shiftSparkFloor)
				clamped := false
				if unclampedIga < igaFloorDeg {
					igaActualDeg = igaFloorDeg
					clamped = true
				} else {
					igaActualDeg = unclampedIga
				}

				// Convert actual achieved retard back into actual spark efficiency via 0x2553AA -> 0x2524EC
				actualRetardDeg := math.Max(0.0, igaBaseDeg-igaActualDeg)
				etaIga = Interp1D(ecu.RetardToEffAxis, ecu.RetardToEffVal, actualRetardDeg)
				actualEngNm = math.Min(tcuContCapNm, tqiUncappedNm) * etaIga

				if curShiftEvent != nil {
					curShiftEvent.TCUTorqueReqNm = round2(shiftReqNm)
					curShiftEvent.SparkEffTarget = round3(etaReq)
					if actualRetardDeg > curShiftEvent.SparkRetardDeg {
						curShiftEvent.SparkRetardDeg = round2(actualRetardDeg)
					}
					if igaActualDeg < curShiftEvent.MinSparkDeg {
						curShiftEvent.MinSparkDeg = round2(igaActualDeg)
					}
					if clamped {
						curShiftEvent.SparkFloorClamped = true
					}
					if actualEngNm < curShiftEvent.ActualMinTorqueNm {
						curShiftEvent.ActualMinTorqueNm = round2(actualEngNm)
					}
				}

				// Complete shift when inertia duration finishes
				if shiftElapsedMs >= baseShiftDurMs {
					gear = shiftTargetGear
					shiftActive = false
					if curShiftEvent != nil {
						completedShifts = append(completedShifts, *curShiftEvent)
						curShiftEvent = nil
					}
				}
			} else {
				// Normal in-gear operation
				igaFloorDeg = igaMinMapDeg
				effectiveCap := math.Min(tcuLimitNm, tchaCapNm) - tcsCutNm
				if effectiveCap < tqiUncappedNm {
					etaIga = math.Max(0.15, effectiveCap/math.Max(1.0, tqiUncappedNm))
					reqRetard := Interp1D(ecu.EffToRetardAxis, ecu.EffToRetardDeg, etaIga)
					igaActualDeg = math.Max(igaMinMapDeg, igaBaseDeg-reqRetard)
					actualEngNm = math.Max(0.0, effectiveCap)
				} else {
					etaIga = 1.0
					igaActualDeg = igaBaseDeg
					actualEngNm = tqiUncappedNm
				}
			}
		}

		// 5. 1ms Physical Drivetrain, Weight Transfer, and Wheel Slip Integration
		gearRatio = GearRatiosA8LR1[gear-1]
		overallRatio = gearRatio * FinalDriveA8LR1
		turbineRPM = (omegaRear * overallRatio) * 60.0 / (2.0 * math.Pi)

		// Torque converter speed ratio nu = turbineRPM / engineRPM
		nu := 0.0
		if engineRPM > 100.0 {
			nu = math.Max(0.0, math.Min(1.2, turbineRPM/engineRPM))
		}
		// Torque converter multiplication TR(nu): 1.85 at stall -> 1.00 at coupling (nu >= 0.85)
		tcTorqueRatio := 1.0
		if gear == 1 && nu < 0.85 {
			tcTorqueRatio = 1.85 - 0.85*(nu/0.85)
		} else if gear == 2 && nu < 0.82 {
			tcTorqueRatio = 1.25 - 0.25*(nu/0.82)
		}

		// Engine RPM dynamics
		if tSec < 0 {
			// Pre-stage brake-stand: engine climbs toward stallRPM (0x238B68)
			targetStall := math.Min(stallRPM, 1100.0+(stallRPM-1100.0)*(cfg.PedalPct/100.0))
			engineRPM += (dt / 0.22) * (targetStall - engineRPM)
		} else if shiftActive {
			// During inertia phase, engine RPM blends smoothly toward next gear synchronous RPM
			nextRatio := GearRatiosA8LR1[shiftTargetGear-1]
			syncNextRPM := math.Max(1200.0, turbineRPM*(nextRatio/gearRatio))
			frac := math.Min(1.0, shiftElapsedMs/math.Max(40.0, baseShiftDurMs))
			engineRPM = (1.0-frac)*engineRPM + frac*syncNextRPM
			if curShiftEvent != nil && curShiftEvent.SparkFloorClamped {
				engineRPM += 120.0 * dt
			}
		} else {
			// Coupled / converter slip operation
			slipAllowanceRPM := 0.0
			if gear == 1 && turbineRPM < stallRPM {
				slipAllowanceRPM = (stallRPM - turbineRPM) * 0.78
			} else if gear >= 2 && nu < 0.88 {
				slipAllowanceRPM = 180.0 * (1.0 - nu)
			}
			targetRPM := math.Max(850.0, turbineRPM+slipAllowanceRPM)
			engineRPM += (dt / 0.025) * (targetRPM - engineRPM)
			if engineRPM > 6600.0 {
				engineRPM = 6600.0
			}
		}

		// Transmission output shaft torque (before final drive)
		drivelineEff := 0.90
		transOutTorqueNm := actualEngNm * tcTorqueRatio * gearRatio * drivelineEff
		if shiftActive && curShiftEvent != nil && curShiftEvent.SparkFloorClamped {
			transOutTorqueNm *= 1.05
		}

		// Split transmission output torque between Front (via Magna ATC coupling) and Rear axles
		frontPropShaftNm := math.Min(transOutTorqueNm*0.50, awdActualNm)
		rearPropShaftNm := math.Max(0.0, transOutTorqueNm-frontPropShaftNm)

		frontWheelDriveNm := frontPropShaftNm * FinalDriveA8LR1
		rearWheelDriveNm := rearPropShaftNm * FinalDriveA8LR1

		// Dynamic Longitudinal Weight Transfer (Fz_front, Fz_rear)
		axPrev := accelG * gGrav
		fzFront := (mass*gGrav*lr*math.Cos(gradeRad) - mass*axPrev*env.CGHeightM) / wheelbaseM
		fzRear := (mass*gGrav*lf*math.Cos(gradeRad) + mass*axPrev*env.CGHeightM) / wheelbaseM
		fzFront = math.Max(1500.0, fzFront)
		fzRear = math.Max(1500.0, fzRear)

		// Longitudinal Slip & Pacejka Tire Carcass Relaxation / Breakaway Integration
		var kappaFront, kappaRear, muFront, muRear, fxFront, fxRear float64
		if tSec < 0 || brakePsi >= 180.0 {
			// Brakes holding vehicle stationary on the starting line
			omegaFront = 0.0
			omegaRear = 0.0
			vCarMs = 0.0
			accelG = 0.0
		} else {
			vRef := math.Max(vCarMs, 2.2)
			omegaMaxGear := (6550.0 * 2.0 * math.Pi / 60.0) / overallRatio

			// Residual brake drag during final 180 -> 0 psi pad release
			brakeDragFrontNm := (brakePsi / 180.0) * 320.0
			brakeDragRearNm := (brakePsi / 180.0) * 280.0
			netDriveFrontNm := math.Max(0.0, frontWheelDriveNm-brakeDragFrontNm)
			netDriveRearNm := math.Max(0.0, rearWheelDriveNm-brakeDragRearNm)

			// Peak tractive torque capacity of each axle on current surface
			tPeakFrontNm := surf.MuPeak * fzFront * rFront
			tPeakRearNm := surf.MuPeak * fzRear * rRear

			// Effective rotational inertia (axle + reflected engine/transmission inertia)
			jEffFront := jFrontAxle + 0.30*jEngine*overallRatio*overallRatio
			jEffRear := jRearAxle + 0.70*jEngine*overallRatio*overallRatio

			// Front Axle Slip & Angular Velocity Update
			curKappaF := math.Max(0.0, (omegaFront*rFront-vCarMs)/vRef)
			if netDriveFrontNm <= tPeakFrontNm && curKappaF <= surf.KappaPeak*1.12 {
				ratioF := math.Max(0.0, math.Min(1.0, netDriveFrontNm/math.Max(1.0, tPeakFrontNm)))
				kappaEqF := surf.KappaPeak * math.Pow((2.0/math.Pi)*math.Asin(ratioF), 1.0/0.85)
				kappaFront = curKappaF + (dt/0.012)*(kappaEqF-curKappaF)
				omegaFront = (vCarMs + kappaFront*vRef) / rFront
			} else {
				muF := EvalMuSlip(surf, curKappaF)
				alphaF := (netDriveFrontNm - muF*fzFront*rFront) / jEffFront
				omegaFront = math.Max(vCarMs/rFront, math.Min(omegaMaxGear, omegaFront+alphaF*dt))
				kappaFront = math.Max(0.0, math.Min(0.65, (omegaFront*rFront-vCarMs)/vRef))
			}

			// Rear Axle Slip & Angular Velocity Update
			curKappaR := math.Max(0.0, (omegaRear*rRear-vCarMs)/vRef)
			if netDriveRearNm <= tPeakRearNm && curKappaR <= surf.KappaPeak*1.12 {
				ratioR := math.Max(0.0, math.Min(1.0, netDriveRearNm/math.Max(1.0, tPeakRearNm)))
				kappaEqR := surf.KappaPeak * math.Pow((2.0/math.Pi)*math.Asin(ratioR), 1.0/0.85)
				kappaRear = curKappaR + (dt/0.012)*(kappaEqR-curKappaR)
				omegaRear = (vCarMs + kappaRear*vRef) / rRear
			} else {
				muR := EvalMuSlip(surf, curKappaR)
				alphaR := (netDriveRearNm - muR*fzRear*rRear) / jEffRear
				omegaRear = math.Max(vCarMs/rRear, math.Min(omegaMaxGear, omegaRear+alphaR*dt))
				kappaRear = math.Max(0.0, math.Min(0.65, (omegaRear*rRear-vCarMs)/vRef))
			}

			muFront = EvalMuSlip(surf, kappaFront)
			muRear = EvalMuSlip(surf, kappaRear)
			fxFront = muFront * fzFront
			fxRear = muRear * fzRear

			// Resistive Forces on Vehicle Body
			vAirMs := vCarMs + (env.HeadwindKmh / 3.6)
			fAeroN := 0.5 * rhoAir * cdAero * frontalM2 * vAirMs * math.Abs(vAirMs)
			fRollN := surf.RollingRes * (fzFront + fzRear)
			fGradeN := mass * gGrav * math.Sin(gradeRad)

			fNetN := fxFront + fxRear - fAeroN - fRollN - fGradeN
			axMs2 := fNetN / mass
			if vCarMs <= 0.01 && axMs2 < 0 {
				axMs2 = 0.0
			}

			vCarMs = math.Max(0.0, vCarMs+axMs2*dt)
			distM += vCarMs * dt

			prevAccelG = accelG
			accelG = axMs2 / gGrav
		}

		// Track shift jerk & min G
		if shiftActive && curShiftEvent != nil {
			if accelG < curShiftEvent.MinAccelG {
				curShiftEvent.MinAccelG = round2(accelG)
			}
			jerk := math.Abs(accelG-prevAccelG) / dt
			if jerk > curShiftEvent.PeakJerkGPerS {
				curShiftEvent.PeakJerkGPerS = round2(jerk)
			}
		}

		// Update Performance Summary Milestones
		if tSec >= 0 {
			if !rolloutPassed && distM >= rolloutDistM {
				rolloutPassed = true
				clockStartSec = tSec
				clockStarted = true
			} else if !clockStarted && vCarMs > 0.02 && cfg.RolloutInch <= 0 {
				clockStartSec = tSec
				clockStarted = true
			}

			if clockStarted {
				et := tSec - clockStartSec
				distFromStartFt := (distM - rolloutDistM) * 3.28084

				if summary.ZeroTo60MphSec == 0 && vMph >= 60.0 {
					summary.ZeroTo60MphSec = round2(et)
				}
				if summary.ZeroTo100KmhSec == 0 && vKmh >= 100.0 {
					summary.ZeroTo100KmhSec = round2(et)
				}
				if summary.ZeroTo100MphSec == 0 && vMph >= 100.0 {
					summary.ZeroTo100MphSec = round2(et)
				}
				if summary.ZeroTo200KmhSec == 0 && vKmh >= 200.0 {
					summary.ZeroTo200KmhSec = round2(et)
					if summary.ZeroTo100KmhSec > 0 {
						summary.HundredTo200KmhSec = round2(summary.ZeroTo200KmhSec - summary.ZeroTo100KmhSec)
					}
				}
				if summary.SixtyTo130MphSec == 0 && vMph >= 130.0 && summary.ZeroTo60MphSec > 0 {
					summary.SixtyTo130MphSec = round2(et - summary.ZeroTo60MphSec)
				}

				if summary.SixtyFootSec == 0 && distFromStartFt >= 60.0 {
					summary.SixtyFootSec = round2(et)
				}
				if summary.ThreeThirtyFootSec == 0 && distFromStartFt >= 330.0 {
					summary.ThreeThirtyFootSec = round2(et)
				}
				if summary.EighthMileSec == 0 && distFromStartFt >= 660.0 {
					summary.EighthMileSec = round2(et)
					summary.EighthMileMph = round2(vMph)
				}
				if summary.ThousandFootSec == 0 && distFromStartFt >= 1000.0 {
					summary.ThousandFootSec = round2(et)
				}
				if summary.QuarterMileSec == 0 && distFromStartFt >= 1320.0 {
					summary.QuarterMileSec = round2(et)
					summary.QuarterMileMph = round2(vMph)
				}
			}

			if accelG > summary.PeakAccelG {
				summary.PeakAccelG = round2(accelG)
			}
			if actualEngNm > summary.PeakEngineTorqueNm {
				summary.PeakEngineTorqueNm = round2(actualEngNm)
			}
			wheelPowerHp := ((fxFront + fxRear) * vCarMs) / 745.7
			if wheelPowerHp > summary.PeakWheelPowerHp {
				summary.PeakWheelPowerHp = round2(wheelPowerHp)
			}
			if awdActualNm > summary.PeakAWDCouplingNm {
				summary.PeakAWDCouplingNm = round2(awdActualNm)
			}
			if math.Abs(tSec-0.05) < 0.0015 {
				summary.LaunchAWDTorqueNm = round2(awdActualNm)
			}
			slipFPct := kappaFront * 100.0
			slipRPct := kappaRear * 100.0
			if slipFPct > summary.MaxFrontSlipPct {
				summary.MaxFrontSlipPct = round2(slipFPct)
			}
			if slipRPct > summary.MaxRearSlipPct {
				summary.MaxRearSlipPct = round2(slipRPct)
			}
			if vKmh > summary.TopSpeedKmh {
				summary.TopSpeedKmh = round2(vKmh)
				summary.TopSpeedMph = round2(vMph)
			}
		}

		// Record 50 Hz (every 20ms) telemetry samples for clean UI rendering
		if step%20 == 0 {
			boostPsi := (boostHpa - ambientHpa) * 0.0145038
			telemetry = append(telemetry, TelemetrySample{
				TimeSec:           round3(tSec),
				DistanceM:         round2(distM),
				DistanceFt:        round2(distM * 3.28084),
				SpeedKmh:          round2(vKmh),
				SpeedMph:          round2(vMph),
				AccelG:            round3(accelG),
				Gear:              gear,
				ShiftActive:       shiftActive,
				EngineRPM:         round2(engineRPM),
				TurbineRPM:        round2(turbineRPM),
				PedalPct:          round2(cfg.PedalPct),
				BrakePsi:          round2(brakePsi),
				BoostHpa:          round2(boostHpa),
				BoostPsi:          round2(boostPsi),
				BoostTargetPsi:    round2(boostTargetPsi),
				AirloadMgStk:      round2(airloadMg),
				DriverDemandNm:    round2(tqiDrvNm),
				UncappedTorqueNm:  round2(tqiUncappedNm),
				TCUTorqueLimitNm:  round2(tcuLimitNm),
				ActualEngineNm:    round2(actualEngNm),
				SparkAdvanceDeg:   round2(igaActualDeg),
				SparkFloorDeg:     round2(igaFloorDeg),
				SparkEfficiency:   round3(etaIga),
				KnockRetardDeg:    round2(krDeg),
				IntakeCamDeg:      round2(camInDeg),
				ExhaustCamDeg:     round2(camExDeg),
				CamOverlapDeg:     round2(camOverlapDeg),
				Lambda:            round3(fuelSpec.CommandedLambda),
				AWDTargetNm:       round2(awdTargetNm),
				AWDActualNm:       round2(awdActualNm),
				AWDFrontSplitPct:  round2(awdSplitPct),
				AWDBrakeGated:     awdBrakeGated,
				FrontPropTorqueNm: round2(frontPropShaftNm),
				RearPropTorqueNm:  round2(rearPropShaftNm),
				FrontNormalLoadN:  round2(fzFront),
				RearNormalLoadN:   round2(fzRear),
				FrontWheelKmh:     round2(omegaFront * rFront * 3.6),
				RearWheelKmh:      round2(omegaRear * rRear * 3.6),
				FrontSlipPct:      round2(kappaFront * 100.0),
				RearSlipPct:       round2(kappaRear * 100.0),
				FrontMu:           round3(muFront),
				RearMu:            round3(muRear),
				FrontTractiveN:    round2(fxFront),
				RearTractiveN:     round2(fxRear),
				TCSActive:         tcsActive,
				TCHACutActive:     tchaActiveFlag,
			})
		}
	}

	return &RunResult{
		Label:       cfg.Label,
		Config:      cfg,
		Calibration: calSummary,
		Summary:     summary,
		Shifts:      completedShifts,
		Telemetry:   telemetry,
	}, nil
}

func extractCol(grid [][]float64, col int) []float64 {
	res := make([]float64, len(grid))
	for r := 0; r < len(grid); r++ {
		if col < len(grid[r]) {
			res[r] = grid[r][col]
		}
	}
	return res
}

// buildRunDiff computes the performance delta (Run B minus Run A) and highlights every binary calibration difference.
func buildRunDiff(a, b *RunResult) *RunDiffSummary {
	diff := &RunDiffSummary{
		DeltaZeroTo60MphSec:     round2(b.Summary.ZeroTo60MphSec - a.Summary.ZeroTo60MphSec),
		DeltaZeroTo100KmhSec:    round2(b.Summary.ZeroTo100KmhSec - a.Summary.ZeroTo100KmhSec),
		DeltaHundredTo200KmhSec: round2(b.Summary.HundredTo200KmhSec - a.Summary.HundredTo200KmhSec),
		DeltaSixtyFootSec:       round2(b.Summary.SixtyFootSec - a.Summary.SixtyFootSec),
		DeltaEighthMileSec:      round2(b.Summary.EighthMileSec - a.Summary.EighthMileSec),
		DeltaQuarterMileSec:     round2(b.Summary.QuarterMileSec - a.Summary.QuarterMileSec),
		DeltaQuarterMileMph:     round2(b.Summary.QuarterMileMph - a.Summary.QuarterMileMph),
		DeltaPeakTorqueNm:       round2(b.Summary.PeakEngineTorqueNm - a.Summary.PeakEngineTorqueNm),
		DeltaPeakWheelHp:        round2(b.Summary.PeakWheelPowerHp - a.Summary.PeakWheelPowerHp),
	}

	var notes []string
	ca, cb := a.Calibration, b.Calibration

	if ca.ECUBinName != cb.ECUBinName {
		notes = append(notes, fmt.Sprintf("ECU Binary: %s vs %s", ca.ECUBinName, cb.ECUBinName))
	}
	if math.Abs(ca.ECUShiftSparkFloorDeg-cb.ECUShiftSparkFloorDeg) > 0.1 {
		notes = append(notes, fmt.Sprintf("ECU Shift Spark Floor (0x207AD9): %.1f° vs %.1f° BTDC", ca.ECUShiftSparkFloorDeg, cb.ECUShiftSparkFloorDeg))
	}
	if ca.ECUTCSInhibit != cb.ECUTCSInhibit {
		notes = append(notes, fmt.Sprintf("ECU TCS Inhibit Switch (0x200B4B): %d vs %d", ca.ECUTCSInhibit, cb.ECUTCSInhibit))
	}
	if ca.ECUTCHALimitActive != cb.ECUTCHALimitActive {
		notes = append(notes, fmt.Sprintf("ECU TCHA Turbo Overspeed Cut (0x203EC6): active=%v (%d cyc) vs active=%v (%d cyc)",
			ca.ECUTCHALimitActive, ca.ECUTCHADebounceCyc, cb.ECUTCHALimitActive, cb.ECUTCHADebounceCyc))
	}
	if math.Abs(ca.ECUTQIBasPeakNm-cb.ECUTQIBasPeakNm) > 1.0 {
		notes = append(notes, fmt.Sprintf("ECU TQI_BAS Peak Torque (0x255518): %.1f Nm vs %.1f Nm", ca.ECUTQIBasPeakNm, cb.ECUTQIBasPeakNm))
	}
	if ca.ECURALPatchDetected != cb.ECURALPatchDetected {
		notes = append(notes, fmt.Sprintf("ECU Rolling Antilag Patch (0x808DC000): %v vs %v", ca.ECURALPatchDetected, cb.ECURALPatchDetected))
	}

	if math.Abs(ca.TCULaunchStallRPM-cb.TCULaunchStallRPM) > 10 {
		notes = append(notes, fmt.Sprintf("TCU Launch Stall Limit (0x238B68): %.0f RPM vs %.0f RPM", ca.TCULaunchStallRPM, cb.TCULaunchStallRPM))
	}
	if math.Abs(ca.TCUPrimaryTorqueCapNm-cb.TCUPrimaryTorqueCapNm) > 5 {
		notes = append(notes, fmt.Sprintf("TCU Torque Protection Limit (0x1FD198): %.0f Nm vs %.0f Nm", ca.TCUPrimaryTorqueCapNm, cb.TCUPrimaryTorqueCapNm))
	}
	if ca.TCUShiftCutRaw != cb.TCUShiftCutRaw || ca.TCUShiftTimerMs != cb.TCUShiftTimerMs {
		notes = append(notes, fmt.Sprintf("TCU Shift Intervention (0x233D3C / 0x20FC52): cut=%d (%d ms) vs cut=%d (%d ms)",
			ca.TCUShiftCutRaw, ca.TCUShiftTimerMs, cb.TCUShiftCutRaw, cb.TCUShiftTimerMs))
	}

	if ca.AWDBrakeGateBypassed != cb.AWDBrakeGateBypassed {
		notes = append(notes, fmt.Sprintf("AWD 64.5 psi Brake-Boost Gate (0xC06080): Bypassed=%v vs Bypassed=%v",
			ca.AWDBrakeGateBypassed, cb.AWDBrakeGateBypassed))
	}
	if math.Abs(ca.AWDSlewRateNmPerSec-cb.AWDSlewRateNmPerSec) > 50 {
		notes = append(notes, fmt.Sprintf("AWD Coupling Slew Rate (0xC07650): %.0f Nm/s (%.0f Nm/20ms) vs %.0f Nm/s (%.0f Nm/20ms)",
			ca.AWDSlewRateNmPerSec, ca.AWDSlewRateNmPer20ms, cb.AWDSlewRateNmPerSec, cb.AWDSlewRateNmPer20ms))
	}

	diff.KeyCalibrationDiffs = notes
	return diff
}

// GetScenarioPresets builds ready-to-run A/B diagnostic & performance presets using the discovered local binaries.
func GetScenarioPresets(ecuBins, tcuBins, awdBins []BinFileItem) []ScenarioPreset {
	findBin := func(list []BinFileItem, substr string) string {
		for _, b := range list {
			if strings.Contains(strings.ToLower(b.Filename), strings.ToLower(substr)) {
				return b.Path
			}
		}
		if len(list) > 0 {
			return list[0].Path
		}
		return ""
	}

	ecuQ := findBin(ecuBins, "quiet_good")
	ecuV103 := findBin(ecuBins, "v103")
	ecuKJT := findBin(ecuBins, "kimjongtuned_bef")
	ecuLoud := findBin(ecuBins, "bad_loud")
	ecuRAL := findBin(ecuBins, "ckhk3tms2c7c_ralv1")

	tcuStock := findBin(tcuBins, "stock_read_complete")
	tcuEKM := findBin(tcuBins, "ekm422402108")

	awdStock := findBin(awdBins, "c00000_c0bfff")
	awdSportLaunch := findBin(awdBins, "sportlaunch_30mph")

	return []ScenarioPreset{
		{
			ID:          "stock_vs_trifecta",
			Name:        "1. Stock Baseline vs. Full ECU + EKM TCU + SportLaunch AWD Trifecta",
			Category:    "Full Calibration Comparison",
			Description: "Compares 100% stock SIM2K-260 (Q) + Stock CPTSH 8AT + Stock Magna AWD against IntensePop v103 (+14% boost) + EKM 8AT (3500 stall / 700 Nm / 90ms shift) + SportLaunch 30mph AWD (brake-gate bypass + 9000 Nm/s).",
			Highlight:   "Shows how unlocking TCU torque caps (0x1FD198), raising brake-stall to 3500 RPM (0x238B68), and bypassing the AWD 64.5 psi brake gate (0xC06080) transforms 60-ft, 0-60 mph, and 1/4-mile performance.",
			Environment: EnvironmentConfig{
				SurfaceID:       "dry_warm",
				AmbientTempC:    20,
				CoolantTempC:    85,
				VehicleMassKg:   1915,
				StaticFrontPct:  52.0,
				FrontTireRadius: 0.334,
				RearTireRadius:  0.334,
				SimDurationSec:  14.5,
			},
			RunA: RunConfig{
				Label:           "Run A: Stock Q + Stock 8AT + Stock AWD",
				ECUBinPath:      ecuQ,
				TCUBinPath:      tcuStock,
				AWDBinPath:      awdStock,
				DriveMode:       "sport",
				ESCMode:         "sport",
				PreStageTimeSec: 1.4,
				BrakeHoldPsi:    1200,
				BrakeReleaseMs:  140,
				PedalPct:        100,
				Overrides:       OverrideConfig{TorqueScalePct: 100, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
			RunB: RunConfig{
				Label:           "Run B: v103 ECU + EKM 8AT + SportLaunch AWD",
				ECUBinPath:      ecuV103,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdSportLaunch,
				DriveMode:       "sport",
				ESCMode:         "sport",
				PreStageTimeSec: 1.4,
				BrakeHoldPsi:    1200,
				BrakeReleaseMs:  140,
				PedalPct:        100,
				Overrides:       OverrideConfig{TorqueScalePct: 114, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
		},
		{
			ID:          "awd_brake_gate_showdown",
			Name:        "2. AWD 64.5 psi Brake-Boost Gate (0xC06080): Stock AWD vs. SportLaunch 30mph Mod",
			Category:    "AWD Launch Physics",
			Description: "Both runs use identical v103 ECU + EKM 8AT (3500 RPM stall) on Cold Unprepped Street Asphalt (mu=0.86). Run A uses Stock AWD (0xC07498=4.4 bar gate active); Run B uses SportLaunch 30mph Mod (0xC06080 zeroed + 180 Nm/20ms slew).",
			Highlight:   "Isolates the exact 167 ms post-release rear-wheelspin window caused by the stock Magna ATC 64.5 psi brake-pressure gate and 6000 Nm/s slew rate.",
			Environment: EnvironmentConfig{
				SurfaceID:       "cold_street",
				AmbientTempC:    15,
				CoolantTempC:    85,
				VehicleMassKg:   1915,
				StaticFrontPct:  52.0,
				FrontTireRadius: 0.334,
				RearTireRadius:  0.334,
				SimDurationSec:  14.0,
			},
			RunA: RunConfig{
				Label:           "Run A: Stock AWD (64.5 psi Brake Gate Active)",
				ECUBinPath:      ecuV103,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdStock,
				DriveMode:       "sport",
				ESCMode:         "off",
				PreStageTimeSec: 1.5,
				BrakeHoldPsi:    1200,
				BrakeReleaseMs:  140,
				PedalPct:        100,
				Overrides:       OverrideConfig{TorqueScalePct: 112, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
			RunB: RunConfig{
				Label:           "Run B: SportLaunch 30mph AWD (Pre-Staged + 9000 Nm/s)",
				ECUBinPath:      ecuV103,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdSportLaunch,
				DriveMode:       "sport",
				ESCMode:         "off",
				PreStageTimeSec: 1.5,
				BrakeHoldPsi:    1200,
				BrakeReleaseMs:  140,
				PedalPct:        100,
				Overrides:       OverrideConfig{TorqueScalePct: 112, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
		},
		{
			ID:          "shift_spark_floor_v103_vs_kjt",
			Name:        "3. Shift Spark Floor Clamp (0x207AD9): v103 (-6.0°) vs. KJT (+12.0° Blocked Cut)",
			Category:    "ECU-TCU Shift Correlation",
			Description: "Compares IntensePop v103 (0x207AD9 = -6.0° BTDC) against KimJongTuned BEF (0x207AD9 = +12.0° BTDC, +4.5° WOT advance) with EKM 8AT.",
			Highlight:   "Shows how KJT's +12.0° shift spark floor blocks the ECU from retarding timing below +12° during TCU inertia-phase cuts (eta_iga clamped at ~0.88 vs 0.38 in v103), while gaining top-end power from aggressive WOT spark.",
			Environment: EnvironmentConfig{
				SurfaceID:       "dry_warm",
				AmbientTempC:    20,
				CoolantTempC:    85,
				VehicleMassKg:   1915,
				StaticFrontPct:  52.0,
				FrontTireRadius: 0.334,
				RearTireRadius:  0.334,
				SimDurationSec:  14.5,
			},
			RunA: RunConfig{
				Label:           "Run A: IntensePop v103 (-6.0° Shift Floor)",
				ECUBinPath:      ecuV103,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdSportLaunch,
				DriveMode:       "sport",
				ESCMode:         "sport",
				PreStageTimeSec: 1.4,
				BrakeHoldPsi:    1200,
				BrakeReleaseMs:  130,
				PedalPct:        100,
				Overrides:       OverrideConfig{TorqueScalePct: 114, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
			RunB: RunConfig{
				Label:           "Run B: KimJongTuned BEF (+12.0° Shift Floor / High Spark)",
				ECUBinPath:      ecuKJT,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdSportLaunch,
				DriveMode:       "sport",
				ESCMode:         "sport",
				PreStageTimeSec: 1.4,
				BrakeHoldPsi:    1200,
				BrakeReleaseMs:  130,
				PedalPct:        100,
				Overrides:       OverrideConfig{TorqueScalePct: 114, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
		},
		{
			ID:          "loud_tcha_surge_diagnosis",
			Name:        "4. TCHA Turbo Speed Limiter Surge Bug (0x203EC6): Sleepy Loud vs. v103 Clean Pull",
			Category:    "ECU Fault Diagnosis",
			Description: "Simulates SLEEPY_BAD_Loud_DECAT_csokay.bin (inflated 967.8 Nm TQI_BAS + 0x203EC6 = 10 cycles debounce -> 200 Nm drop & 17 Nm/cycle ramp) vs. IntensePop v103 (0x203EC6 = 65535 disabled).",
			Highlight:   "Reproduces the exact WOT sawtooth torque & boost oscillation caused by the active TCHA turbocharger speed limiter in the Loud binary.",
			Environment: EnvironmentConfig{
				SurfaceID:       "dry_warm",
				AmbientTempC:    18,
				CoolantTempC:    85,
				VehicleMassKg:   1915,
				StaticFrontPct:  52.0,
				FrontTireRadius: 0.334,
				RearTireRadius:  0.334,
				SimDurationSec:  14.0,
			},
			RunA: RunConfig{
				Label:           "Run A: Sleepy Loud Bin (TCHA 0x203EC6 Surge Active)",
				ECUBinPath:      ecuLoud,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdSportLaunch,
				DriveMode:       "sport",
				ESCMode:         "sport",
				PreStageTimeSec: 1.4,
				BrakeHoldPsi:    1200,
				BrakeReleaseMs:  130,
				PedalPct:        100,
				Overrides:       OverrideConfig{TorqueScalePct: 112, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
			RunB: RunConfig{
				Label:           "Run B: IntensePop v103 (TCHA Limiter Disabled)",
				ECUBinPath:      ecuV103,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdSportLaunch,
				DriveMode:       "sport",
				ESCMode:         "sport",
				PreStageTimeSec: 1.4,
				BrakeHoldPsi:    1200,
				BrakeReleaseMs:  130,
				PedalPct:        100,
				Overrides:       OverrideConfig{TorqueScalePct: 112, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
		},
		{
			ID:          "ral_drag_launch",
			Name:        "5. VHT Drag Strip Launch: RALv1 Antilag Patch (0x808DC000) vs. Standard Stall",
			Category:    "Drag Strip Launch",
			Description: "Compares normal converter brake-stall against the RALv1 antilag binary (0x808DC000 code cave building full 2717 hPa manifold pressure on the line) on VHT prepped rubber (mu=1.35).",
			Highlight:   "Demonstrates instant 1.15G+ 60-ft launch acceleration when leaving the line at full boost with SportLaunch AWD pre-staged.",
			Environment: EnvironmentConfig{
				SurfaceID:       "drag_vht",
				AmbientTempC:    18,
				CoolantTempC:    82,
				VehicleMassKg:   1880,
				StaticFrontPct:  52.0,
				FrontTireRadius: 0.334,
				RearTireRadius:  0.334,
				SimDurationSec:  13.5,
			},
			RunA: RunConfig{
				Label:           "Run A: v103 Standard Brake-Stall",
				ECUBinPath:      ecuV103,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdSportLaunch,
				DriveMode:       "sport",
				ESCMode:         "off",
				PreStageTimeSec: 1.5,
				BrakeHoldPsi:    1300,
				BrakeReleaseMs:  110,
				PedalPct:        100,
				RolloutInch:     12,
				Overrides:       OverrideConfig{TorqueScalePct: 118, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
			RunB: RunConfig{
				Label:           "Run B: RALv1 Antilag Patch (Full Boost on Line)",
				ECUBinPath:      ecuRAL,
				TCUBinPath:      tcuEKM,
				AWDBinPath:      awdSportLaunch,
				DriveMode:       "sport",
				ESCMode:         "off",
				PreStageTimeSec: 1.5,
				BrakeHoldPsi:    1300,
				BrakeReleaseMs:  110,
				PedalPct:        100,
				RolloutInch:     12,
				Overrides:       OverrideConfig{TorqueScalePct: 118, ShiftCutScalePct: 100, FrontSplitScalePct: 100},
			},
		},
	}
}
