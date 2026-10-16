package main

import (
	"math"
)

// G6DPEngineSpec defines the complete physical, thermodynamic, and calibration specifications
// of the Hyundai/Genesis Lambda II 3.3L Twin-Turbo V6 (G6DP) controlled by Continental SIM2K-260.
type G6DPEngineSpec struct {
	// Geometric & Mechanical Parameters
	EngineCode        string  `json:"engine_code"`         // "Lambda II G6DP"
	Architecture      string  `json:"architecture"`        // "60-degree V6, Aluminum block and heads"
	DisplacementCc    float64 `json:"displacement_cc"`     // 3342.0 cc
	BoreMm            float64 `json:"bore_mm"`             // 92.0 mm
	StrokeMm          float64 `json:"stroke_mm"`           // 83.8 mm
	BoreToStrokeRatio float64 `json:"bore_to_stroke_ratio"`// 1.098 (Over-square)
	CompressionRatio  float64 `json:"compression_ratio"`   // 10.0:1
	ConnectingRodMm   float64 `json:"connecting_rod_mm"`   // 152.5 mm
	FiringOrder       string  `json:"firing_order"`        // "1-2-3-4-5-6"
	Valvetrain        string  `json:"valvetrain"`          // "DOHC 24V, Dual-CVVT (Dual Continuous Variable Valve Timing)"
	FuelSystem        string  `json:"fuel_system"`         // "Direct Injection (GDI) up to 200 bar"
	EngineInertiaKgM2 float64 `json:"engine_inertia_kg_m2"`// 0.24 kg*m^2 (crankshaft + flywheel + TC impeller)
	IdleRPMSpec       float64 `json:"idle_rpm_spec"`       // 850.0 RPM
	RedlineRPMSpec     float64 `json:"redline_rpm_spec"`    // 6600.0 RPM

	// Turbocharger Hardware Specifications
	TurbochargerType string  `json:"turbocharger_type"` // "Twin Garrett MGT1549DSL wastegated mono-scroll"
	CompressorInd    float64 `json:"compressor_ind_mm"` // 36.4 mm
	CompressorExd    float64 `json:"compressor_exd_mm"` // 49.0 mm
	TurbineInd       float64 `json:"turbine_ind_mm"`    // 45.0 mm
	TurbineExd       float64 `json:"turbine_exd_mm"`    // 40.2 mm
	WastegateType    string  `json:"wastegate_type"`    // "Electronic PWM actuator with position feedback"
	IntercoolerType  string  `json:"intercooler_type"`  // "Dual air-to-air charge air coolers"
	MaxPressureRatio float64 `json:"max_pressure_ratio"`// 2.70

	// Continental SIM2K-260 Air-Path & Airload Quantization
	AirChargeLSB      float64 `json:"air_charge_lsb"`       // 0.04238956 mg/stroke per LSB
	ManifoldVolLiters float64 `json:"manifold_vol_liters"` // 4.85 L
	NominalVEPeak     float64 `json:"nominal_ve_peak"`     // 1.00 at 2000-5400 RPM
	PutSpMaxHpa       float64 `json:"put_sp_max_hpa"`      // 2717.0 hPa (0x21EC00 ceiling)
	BoostLimitAHpa    float64 `json:"boost_limit_a_hpa"`   // 2717.0 hPa (0x21F478)

	// Friction Polynomial Coefficients (T_loss = C0 + C1*N + C2*N^2)
	FricPolyC0 float64 `json:"fric_poly_c0"` // 34.0 Nm base boundary friction
	FricPolyC1 float64 `json:"fric_poly_c1"` // 0.0045 Nm/RPM hydrodynamic shearing
	FricPolyC2 float64 `json:"fric_poly_c2"` // 1.0e-6 Nm/RPM^2 windage & pumping

	// ROM Calibration Addresses (Continental SIM2K-260)
	MapAddrs struct {
		DriverDemandComfort string `json:"driver_demand_comfort"` // 0x214280 (12x14)
		DriverDemandSport   string `json:"driver_demand_sport"`   // 0x214E80 (12x14)
		DriverDemandMode4   string `json:"driver_demand_mode4"`   // 0x214760 (12x14)
		GearTorqueFactor    string `json:"gear_torque_factor"`    // 0x213B70 (10x8)
		SpeedTorqueFactor   string `json:"speed_torque_factor"`   // 0x215150 (12x8)
		BoostTargetPUT      string `json:"boost_target_put"`      // 0x21EC00 (1x1 scalar: 2717 hPa)
		BoostLimitA         string `json:"boost_limit_a"`         // 0x21F478 (1x1 scalar: 2717 hPa)
		TCHATurboLimiter    string `json:"tcha_turbo_limiter"`    // 0x203EC6 (debounce cycles)
		TCHAMinTorque       string `json:"tcha_min_torque"`       // 0x203ED4 (200 Nm clamp)
		TCHARampPerCyc      string `json:"tcha_ramp_per_cyc"`     // 0x203ED8 (17 Nm/cycle ramp)
		TQIBasTableA        string `json:"tqi_bas_table_a"`       // 0x255518 (18x20 Indicated Torque)
		TQIBasTableB        string `json:"tqi_bas_table_b"`       // 0x2557E8 (18x20 Indicated Torque)
		TQIMaxCamEnvelope   string `json:"tqi_max_cam_envelope"`  // 0x253D14.. (8 Cam-State Envelope)
		IgnBaseHighWOT      string `json:"ign_base_high_wot"`     // 0x238090 (18x20 Base Spark Advance)
		IgnOptHighMBT       string `json:"ign_opt_high_mbt"`      // 0x238874 (18x20 Optimum MBT Spark)
		IGAMinMapHigh       string `json:"iga_min_map_high"`      // 0x239AF4 (16x16 Minimum Spark Advance)
		ShiftSparkFloor     string `json:"shift_spark_floor"`     // 0x207AD9 (Scalar: -3.0 Stock / +12.0 KJT)
		SparkEffToRetard    string `json:"spark_eff_to_retard"`   // 0x25262C (16 pts: eta -> deg retard)
		SparkRetardToEff    string `json:"spark_retard_to_eff"`   // 0x2524EC (16 pts: deg retard -> eta)
		TCSInhibitSwitch    string `json:"tcs_inhibit_switch"`    // 0x200B4B
		RALAntilagCave      string `json:"ral_antilag_cave"`      // 0x808DC000 (0x3DC000 ROM offset)
	} `json:"map_addrs"`
}

// TorqueConverterPoint defines a hydrodynamic operating point for the torque converter.
type TorqueConverterPoint struct {
	SpeedRatio     float64 `json:"speed_ratio"`     // nu = Nt / Np (0.00 to 1.00+)
	TorqueRatio    float64 `json:"torque_ratio"`    // TR = Tt / Tp (1.88 at stall -> 1.00 at coupling)
	KFactor        float64 `json:"k_factor"`        // Np / sqrt(Tp) [RPM / sqrt(Nm)]
	CapacityFactor float64 `json:"capacity_factor"` // Tp / (Np/1000)^2 [Nm / (1000 RPM)^2]
}

// A8LR1TransSpec defines the physical, kinematic, and calibration parameters of the
// Hyundai/Kefico A8LR1 8-Speed Longitudinal Automatic Transmission (CPTSH).
type A8LR1TransSpec struct {
	ModelCode           string  `json:"model_code"`             // "A8LR1"
	Manufacturer        string  `json:"manufacturer"`           // "Hyundai Powertech / Kefico"
	GearCount           int     `json:"gear_count"`             // 8 Forward + 1 Reverse
	Architecture        string  `json:"architecture"`           // "Lepelletier-type 4-planetary-gearset, 4 clutches (C1-C4), 2 brakes (B1-B2)"
	GearRatios          [8]float64 `json:"gear_ratios"`         // [3.964, 2.468, 1.610, 1.176, 1.000, 0.832, 0.652, 0.565]
	ReverseRatio        float64 `json:"reverse_ratio"`          // 2.273
	FinalDriveRatio     float64 `json:"final_drive_ratio"`      // 3.538
	OverallRatios       [8]float64 `json:"overall_ratios"`      // GearRatios[i] * FinalDriveRatio
	MechanicalEfficiency float64 `json:"mechanical_efficiency"` // 0.90 (loss = 10% in planetary gears + pump)

	// Reflected Rotational Inertias at Wheel Axle (kg*m^2)
	// J_eff(g) = J_axle + J_engine * (Ratio_g * FinalDrive)^2
	ReflectedInertias [8]float64 `json:"reflected_inertias"`

	// Torque Converter Hydrodynamics
	StallTorqueRatioStock float64 `json:"stall_torque_ratio_stock"` // 1.88 (nominal 1.85..1.92)
	CouplingSpeedRatio    float64 `json:"coupling_speed_ratio"`    // 0.85
	StallKFactor          float64 `json:"stall_k_factor"`          // 110.7 RPM / sqrt(Nm)
	TorqueConverterCurve  []TorqueConverterPoint `json:"torque_converter_curve"`

	// CPTSH Firmware Calibration Parameters
	StallRPMStock         float64   `json:"stall_rpm_stock"`           // 2500.0 RPM (0x238B68)
	StallRPMEKM           float64   `json:"stall_rpm_ekm"`             // 3500.0 RPM
	PrimaryTorqueCapStock float64   `json:"primary_torque_cap_stock"`   // 350.0 Nm (0x1FD198)
	PrimaryTorqueCapEKM   float64   `json:"primary_torque_cap_ekm"`     // 700.0 Nm
	GlobalTorqueCapStock  float64   `json:"global_torque_cap_stock"`    // 400.0 Nm (0x2096E4)
	GlobalTorqueCapEKM    float64   `json:"global_torque_cap_ekm"`      // 32767.0 Nm
	PerGearCapStockNm     [8]float64 `json:"per_gear_cap_stock_nm"`     // [350, 350, 350, 350, 350, 350, 350, 350]
	PerGearCapEKMNm       [8]float64 `json:"per_gear_cap_ekm_nm"`       // [700, 700, 700, 700, 700, 700, 700, 700]
	ShiftCutStock         int       `json:"shift_cut_stock"`           // -10 (0x233D3C)
	ShiftCutEKM           int       `json:"shift_cut_ekm"`             // -15
	ShiftTimerMsStock     int       `json:"shift_timer_ms_stock"`       // 65535 ms (0x20FC52)
	ShiftTimerMsEKM       int       `json:"shift_timer_ms_ekm"`         // 90 ms
	VCMSolenoidFloorStock int       `json:"vcm_solenoid_floor_stock"`   // 850 mA (0x24BB22)
	VCMSolenoidFloorEKM   int       `json:"vcm_solenoid_floor_ekm"`     // 490 mA

	// ROM Calibration Addresses (Kefico CPTSH 95441-4J571)
	MapAddrs struct {
		ShiftScheduleComfort string `json:"shift_schedule_comfort"` // 0x2047E4 (Slot 3, 20x14)
		ShiftScheduleSport   string `json:"shift_schedule_sport"`   // 0x205E64 (Slot 23, 20x14)
		BrakeStallRPMLimit   string `json:"brake_stall_rpm_limit"`   // 0x238B68 (Scalar: 2500 stock vs 3500 EKM)
		PrimaryTorqueCap     string `json:"primary_torque_cap"`     // 0x1FD198 (Scalar: 350 stock vs 700 EKM)
		PerGearTorqueCap     string `json:"per_gear_torque_cap"`     // 0x1FD1B0 (8x1 table)
		GlobalTorqueCap      string `json:"global_torque_cap"`      // 0x2096E4 (Scalar: 400 stock vs 32767 EKM)
		ClutchTorqueLimit    string `json:"clutch_torque_limit"`    // 0x1FBC5C (8x8 clutch envelope)
		ShiftIntervention    string `json:"shift_intervention"`    // 0x233D3C (-10 stock vs -15 EKM)
		ShiftInertiaTimer    string `json:"shift_inertia_timer"`    // 0x20FC52 (65535 stock vs 90 ms EKM)
		VCMBleedFloor        string `json:"vcm_bleed_floor"`        // 0x24BB22 (850 mA stock vs 490 mA EKM)
	} `json:"map_addrs"`
}

// MagnaATCAWDSpec defines the physical electro-hydraulic transfer case parameters and calibration
// of the Magna Active Torque Control (ATC) module (95447-4J105).
type MagnaATCAWDSpec struct {
	ModelCode               string  `json:"model_code"`                 // "Magna ATC 95447-4J105"
	ClutchType              string  `json:"clutch_type"`                // "Wet multi-plate clutch with ball-ramp axial actuator"
	ChainDriveRatio         float64 `json:"chain_drive_ratio"`          // 1.00:1 (silent chain to front output flange)
	NominalTireRadiusMm     float64 `json:"nominal_tire_radius_mm"`     // 320.0 mm (0xC04214 calibration constant)
	BaseClampTorqueNm       float64 `json:"base_clamp_torque_nm"`       // 150.0 Nm (0xC04334)
	MaxClutchTorqueSportNm  float64 `json:"max_clutch_torque_sport_nm"` // 1000.0 Nm (0xC05A1C)
	MaxClutchTorqueComfortNm float64 `json:"max_clutch_torque_comfort_nm"`// 850.0 Nm (0xC059EA)
	BrakeGatePressureBar    float64 `json:"brake_gate_pressure_bar"`    // 4.45 bar / 64.5 psi (0xC07498 / 0xC07606)
	StockSlewRateNmPer20ms  float64 `json:"stock_slew_rate_nm_20ms"`    // 120.0 Nm/20ms
	StockSlewRateNmPerSec   float64 `json:"stock_slew_rate_nm_sec"`     // 6000.0 Nm/s
	TunedSlewRateNmPer20ms  float64 `json:"tuned_slew_rate_nm_20ms"`    // 180.0 Nm/20ms
	TunedSlewRateNmPerSec   float64 `json:"tuned_slew_rate_nm_sec"`     // 9000.0 Nm/s
	FrontAxleInertiaKgM2    float64 `json:"front_axle_inertia_kg_m2"`   // 2.35 kg*m^2 (wheels + hubs + halfshafts)
	RearAxleInertiaKgM2     float64 `json:"rear_axle_inertia_kg_m2"`    // 2.65 kg*m^2 (wheels + hubs + diff + prop shaft)

	// ROM Calibration Addresses (Magna ATC 95447-4J105)
	MapAddrs struct {
		NominalRadiusConst  string `json:"nominal_radius_const"`  // 0xC04214 (320 mm)
		BaseClampTorque     string `json:"base_clamp_torque"`     // 0xC04334 (150 Nm)
		PWMLinearization    string `json:"pwm_linearization"`     // 0xC04238 (PWM mA -> Clamp Force)
		ViscosityComp       string `json:"viscosity_comp"`        // 0xC05AE0 (Oil Temp -> Clamp Scale)
		LaunchPreloadSport  string `json:"launch_preload_sport"`  // 0xC04778 (16 gears x 5 pedal pts)
		LaunchPreloadEco    string `json:"launch_preload_eco"`    // 0xC046D8 (16 gears x 5 pedal pts)
		OffIdleBasePreload  string `json:"off_idle_base_preload"` // 0xC04A3C (4 pts off-idle preload)
		FeedForwardSport    string `json:"feed_forward_sport"`    // 0xC04BA0 (10 speed x 5 pedal Nm)
		FeedForwardComfort  string `json:"feed_forward_comfort"`  // 0xC04B3C (10 speed x 5 pedal Nm)
		SplitMatrixSport    string `json:"split_matrix_sport"`    // 0xC05922 (5x5 speed x pedal % front)
		SplitMatrixComfort  string `json:"split_matrix_comfort"`  // 0xC0585A (5x5 speed x pedal % front)
		HighSpeedGate1      string `json:"high_speed_gate1"`      // 0xC066F6 (75 km/h cruise cutoff)
		HighSpeedGate2      string `json:"high_speed_gate2"`      // 0xC056CA on 0xC06F96 (140 km/h WOT taper)
		MasterSpeedTaper    string `json:"master_speed_taper"`    // 0xC0640A on 0xC06252 (0..250 km/h ceiling)
		WheelSlipDeadband   string `json:"wheel_slip_deadband"`   // 0xC06978 (0.5 km/h low / 2.0 km/h high)
		WheelSlipCurve      string `json:"wheel_slip_curve"`      // 0xC061FA (11 pts: delta km/h -> coupling Nm)
		BrakePressureGate   string `json:"brake_pressure_gate"`   // 0xC07498 / 0xC07606 (4.45 bar)
		BrakeGateInhibitSw  string `json:"brake_gate_inhibit_sw"` // 0xC06080 / 0xC060B0
		PositiveSlewLimiter string `json:"positive_slew_limiter"` // 0xC07650 (120 vs 180 Nm / 20ms)
	} `json:"map_addrs"`
}

// Platform33TSpec encapsulates the complete vehicle platform, engine, transmission,
// and AWD sub-models for a Stinger GT / Genesis G70 3.3T AWD.
type Platform33TSpec struct {
	PlatformName      string  `json:"platform_name"`       // "Kia Stinger GT / Genesis G70 3.3T AWD"
	CurbMassKg        float64 `json:"curb_mass_kg"`        // 1835 kg curb + 80 kg driver = 1915 kg
	GrossMassKg       float64 `json:"gross_mass_kg"`       // 2320 kg
	WheelbaseM        float64 `json:"wheelbase_m"`         // 2.906 m (Stinger) / 2.835 m (G70)
	StaticFrontPct    float64 `json:"static_front_pct"`    // 52.0%
	StaticRearPct     float64 `json:"static_rear_pct"`     // 48.0%
	CGHeightM         float64 `json:"cg_height_m"`         // 0.51 m
	DragCoefficientCd float64 `json:"drag_coefficient_cd"` // 0.30
	FrontalAreaM2     float64 `json:"frontal_area_m2"`     // 2.28 m^2
	FrontTireSize     string  `json:"front_tire_size"`     // "225/40R19 89Y"
	RearTireSize      string  `json:"rear_tire_size"`      // "255/35R19 92Y"
	FrontTireRadiusM  float64 `json:"front_tire_radius_m"` // 0.334 m
	RearTireRadiusM   float64 `json:"rear_tire_radius_m"`  // 0.336 m (staggered delta = +2 mm)

	// Integrated Powertrain Sub-Models
	Engine       G6DPEngineSpec  `json:"engine"`
	Transmission A8LR1TransSpec  `json:"transmission"`
	AWD          MagnaATCAWDSpec `json:"awd"`
}

// GetStingerG70PlatformSpec returns the complete, factory-verified and teardown-correlated
// specification for the Kia Stinger GT / Genesis G70 3.3T AWD platform.
func GetStingerG70PlatformSpec() Platform33TSpec {
	spec := Platform33TSpec{
		PlatformName:      "Kia Stinger GT / Genesis G70 3.3T AWD",
		CurbMassKg:        1915.0,
		GrossMassKg:       2320.0,
		WheelbaseM:        2.906,
		StaticFrontPct:    52.0,
		StaticRearPct:     48.0,
		CGHeightM:         0.51,
		DragCoefficientCd: 0.30,
		FrontalAreaM2:     2.28,
		FrontTireSize:     "225/40R19 89Y",
		RearTireSize:      "255/35R19 92Y",
		FrontTireRadiusM:  0.334,
		RearTireRadiusM:   0.336,
	}

	// 1. Engine Sub-Model Spec (G6DP - SIM2K-260)
	spec.Engine = G6DPEngineSpec{
		EngineCode:        "Lambda II G6DP",
		Architecture:      "60-degree V6, Aluminum block and heads, integrated exhaust manifolds",
		DisplacementCc:    3342.0,
		BoreMm:            92.0,
		StrokeMm:          83.8,
		BoreToStrokeRatio: 92.0 / 83.8,
		CompressionRatio:  10.0,
		ConnectingRodMm:   152.5,
		FiringOrder:       "1-2-3-4-5-6",
		Valvetrain:        "DOHC 24V, Dual-CVVT (Dual Continuous Variable Valve Timing)",
		FuelSystem:        "Direct Injection (GDI) up to 200 bar",
		EngineInertiaKgM2: 0.24,
		IdleRPMSpec:       850.0,
		RedlineRPMSpec:     6600.0,

		TurbochargerType: "Twin Garrett MGT1549DSL wastegated mono-scroll",
		CompressorInd:    36.4,
		CompressorExd:    49.0,
		TurbineInd:       45.0,
		TurbineExd:       40.2,
		WastegateType:    "Electronic PWM actuator with position feedback",
		IntercoolerType:  "Dual air-to-air charge air coolers",
		MaxPressureRatio: 2.70,

		AirChargeLSB:      0.04238956,
		ManifoldVolLiters: 4.85,
		NominalVEPeak:     1.00,
		PutSpMaxHpa:       2235.0,
		BoostLimitAHpa:    2500.0,

		FricPolyC0: 34.0,
		FricPolyC1: 0.0045,
		FricPolyC2: 1.0e-6,
	}
	spec.Engine.MapAddrs.DriverDemandComfort = "0x214280"
	spec.Engine.MapAddrs.DriverDemandSport = "0x214E80"
	spec.Engine.MapAddrs.DriverDemandMode4 = "0x214760"
	spec.Engine.MapAddrs.GearTorqueFactor = "0x213B70"
	spec.Engine.MapAddrs.SpeedTorqueFactor = "0x215150"
	spec.Engine.MapAddrs.BoostTargetPUT = "0x23D810 / 0x21DA7C / 0x21EB38"
	spec.Engine.MapAddrs.BoostLimitA = "0x21F478"
	spec.Engine.MapAddrs.TCHATurboLimiter = "0x203EC6"
	spec.Engine.MapAddrs.TCHAMinTorque = "0x203ED4"
	spec.Engine.MapAddrs.TCHARampPerCyc = "0x203ED8"
	spec.Engine.MapAddrs.TQIBasTableA = "0x255518"
	spec.Engine.MapAddrs.TQIBasTableB = "0x2557E8"
	spec.Engine.MapAddrs.TQIMaxCamEnvelope = "0x253D14"
	spec.Engine.MapAddrs.IgnBaseHighWOT = "0x238090"
	spec.Engine.MapAddrs.IgnOptHighMBT = "0x238874"
	spec.Engine.MapAddrs.IGAMinMapHigh = "0x239AF4"
	spec.Engine.MapAddrs.ShiftSparkFloor = "0x207AD9"
	spec.Engine.MapAddrs.SparkEffToRetard = "0x25262C"
	spec.Engine.MapAddrs.SparkRetardToEff = "0x2524EC"
	spec.Engine.MapAddrs.TCSInhibitSwitch = "0x200B4B"
	spec.Engine.MapAddrs.RALAntilagCave = "0x808DC000"

	// 2. Transmission Sub-Model Spec (A8LR1 - CPTSH)
	ratios := [8]float64{3.964, 2.468, 1.610, 1.176, 1.000, 0.832, 0.652, 0.565}
	finalDrive := 3.538
	spec.Transmission = A8LR1TransSpec{
		ModelCode:             "A8LR1",
		Manufacturer:          "Hyundai Powertech / Kefico",
		GearCount:             8,
		Architecture:          "Lepelletier-type 4-planetary-gearset, 4 clutches (C1-C4), 2 brakes (B1-B2)",
		GearRatios:            ratios,
		ReverseRatio:          2.273,
		FinalDriveRatio:       finalDrive,
		MechanicalEfficiency:  0.90,
		StallTorqueRatioStock: 1.88,
		CouplingSpeedRatio:    0.85,
		StallKFactor:          110.7,
		StallRPMStock:         2500.0,
		StallRPMEKM:           3500.0,
		PrimaryTorqueCapStock: 350.0,
		PrimaryTorqueCapEKM:   700.0,
		GlobalTorqueCapStock:  400.0,
		GlobalTorqueCapEKM:    32767.0,
		PerGearCapStockNm:     [8]float64{350, 350, 350, 350, 350, 350, 350, 350},
		PerGearCapEKMNm:       [8]float64{700, 700, 700, 700, 700, 700, 700, 700},
		ShiftCutStock:         -10,
		ShiftCutEKM:           -15,
		ShiftTimerMsStock:     65535,
		ShiftTimerMsEKM:       90,
		VCMSolenoidFloorStock: 850,
		VCMSolenoidFloorEKM:   490,
	}

	for i := 0; i < 8; i++ {
		totRatio := ratios[i] * finalDrive
		spec.Transmission.OverallRatios[i] = totRatio
		// Reflected inertia at rear wheel axle: J_wheel + J_engine * totRatio^2
		spec.Transmission.ReflectedInertias[i] = 2.65 + 0.24*totRatio*totRatio
	}

	spec.Transmission.TorqueConverterCurve = []TorqueConverterPoint{
		{SpeedRatio: 0.00, TorqueRatio: 1.88, KFactor: 110.7, CapacityFactor: 0.0816},
		{SpeedRatio: 0.10, TorqueRatio: 1.77, KFactor: 111.9, CapacityFactor: 0.0799},
		{SpeedRatio: 0.20, TorqueRatio: 1.66, KFactor: 114.2, CapacityFactor: 0.0767},
		{SpeedRatio: 0.30, TorqueRatio: 1.55, KFactor: 117.8, CapacityFactor: 0.0721},
		{SpeedRatio: 0.40, TorqueRatio: 1.44, KFactor: 122.5, CapacityFactor: 0.0666},
		{SpeedRatio: 0.50, TorqueRatio: 1.33, KFactor: 129.1, CapacityFactor: 0.0600},
		{SpeedRatio: 0.60, TorqueRatio: 1.22, KFactor: 139.8, CapacityFactor: 0.0512},
		{SpeedRatio: 0.70, TorqueRatio: 1.12, KFactor: 156.4, CapacityFactor: 0.0409},
		{SpeedRatio: 0.80, TorqueRatio: 1.03, KFactor: 185.0, CapacityFactor: 0.0292},
		{SpeedRatio: 0.85, TorqueRatio: 1.00, KFactor: 215.0, CapacityFactor: 0.0216},
		{SpeedRatio: 0.90, TorqueRatio: 1.00, KFactor: 265.0, CapacityFactor: 0.0142},
		{SpeedRatio: 0.95, TorqueRatio: 1.00, KFactor: 380.0, CapacityFactor: 0.0069},
		{SpeedRatio: 1.00, TorqueRatio: 1.00, KFactor: 999.9, CapacityFactor: 0.0000},
	}

	spec.Transmission.MapAddrs.ShiftScheduleComfort = "0x2047E4"
	spec.Transmission.MapAddrs.ShiftScheduleSport = "0x205E64"
	spec.Transmission.MapAddrs.BrakeStallRPMLimit = "0x238B68"
	spec.Transmission.MapAddrs.PrimaryTorqueCap = "0x1FD198"
	spec.Transmission.MapAddrs.PerGearTorqueCap = "0x1FD1B0"
	spec.Transmission.MapAddrs.GlobalTorqueCap = "0x2096E4"
	spec.Transmission.MapAddrs.ClutchTorqueLimit = "0x1FBC5C"
	spec.Transmission.MapAddrs.ShiftIntervention = "0x233D3C"
	spec.Transmission.MapAddrs.ShiftInertiaTimer = "0x20FC52"
	spec.Transmission.MapAddrs.VCMBleedFloor = "0x24BB22"

	// 3. AWD Sub-Model Spec (Magna ATC 95447-4J105)
	spec.AWD = MagnaATCAWDSpec{
		ModelCode:                "Magna ATC 95447-4J105",
		ClutchType:               "Wet multi-plate clutch with ball-ramp axial actuator",
		ChainDriveRatio:          1.00,
		NominalTireRadiusMm:      320.0,
		BaseClampTorqueNm:        150.0,
		MaxClutchTorqueSportNm:   1000.0,
		MaxClutchTorqueComfortNm: 850.0,
		BrakeGatePressureBar:     4.45,
		StockSlewRateNmPer20ms:   120.0,
		StockSlewRateNmPerSec:    6000.0,
		TunedSlewRateNmPer20ms:   180.0,
		TunedSlewRateNmPerSec:    9000.0,
		FrontAxleInertiaKgM2:     2.35,
		RearAxleInertiaKgM2:      2.65,
	}
	spec.AWD.MapAddrs.NominalRadiusConst = "0xC04214"
	spec.AWD.MapAddrs.BaseClampTorque = "0xC04334"
	spec.AWD.MapAddrs.PWMLinearization = "0xC04238"
	spec.AWD.MapAddrs.ViscosityComp = "0xC05AE0"
	spec.AWD.MapAddrs.LaunchPreloadSport = "0xC04778"
	spec.AWD.MapAddrs.LaunchPreloadEco = "0xC046D8"
	spec.AWD.MapAddrs.OffIdleBasePreload = "0xC04A3C"
	spec.AWD.MapAddrs.FeedForwardSport = "0xC04BA0"
	spec.AWD.MapAddrs.FeedForwardComfort = "0xC04B3C"
	spec.AWD.MapAddrs.SplitMatrixSport = "0xC05922"
	spec.AWD.MapAddrs.SplitMatrixComfort = "0xC0585A"
	spec.AWD.MapAddrs.HighSpeedGate1 = "0xC066F6"
	spec.AWD.MapAddrs.HighSpeedGate2 = "0xC056CA"
	spec.AWD.MapAddrs.MasterSpeedTaper = "0xC0640A"
	spec.AWD.MapAddrs.WheelSlipDeadband = "0xC06978"
	spec.AWD.MapAddrs.WheelSlipCurve = "0xC061FA"
	spec.AWD.MapAddrs.BrakePressureGate = "0xC07498"
	spec.AWD.MapAddrs.BrakeGateInhibitSw = "0xC06080"
	spec.AWD.MapAddrs.PositiveSlewLimiter = "0xC07650"

	return spec
}

// EvalTorqueConverter calculates the hydrodynamic operating state of the A8LR1 torque converter
// given pump (engine) angular speed and turbine (input shaft) angular speed.
// Returns:
//   nu: speed ratio Nt / Np
//   torqueRatio: torque multiplication ratio Tt / Tp
//   kFactor: capacity factor Np / sqrt(Tp) in [RPM / sqrt(Nm)]
func EvalTorqueConverter(pumpRPM, turbineRPM float64) (nu, torqueRatio, kFactor float64) {
	if pumpRPM <= 50.0 {
		return 0.0, 1.88, 110.7
	}

	nu = turbineRPM / pumpRPM
	if nu < 0.0 {
		nu = 0.0
	} else if nu > 1.20 {
		nu = 1.20
	}

	const nuCoupling = 0.85
	const trStall = 1.88

	if nu <= nuCoupling {
		// Linear-to-parabolic decline in torque multiplication from stall to coupling point
		normNu := nu / nuCoupling
		torqueRatio = trStall - (trStall-1.00)*(0.90*normNu + 0.10*normNu*normNu)
	} else if nu <= 1.00 {
		// Coupling range: fluid coupling operation with 1:1 torque ratio
		torqueRatio = 1.00
	} else {
		// Overrun / engine-braking (turbine driving pump): inverse fluid shear
		torqueRatio = math.Max(0.80, 1.00-0.20*(nu-1.00))
	}

	// K-factor evaluation: K = Np / sqrt(Tp)
	// Derived from empirical hydrodynamic test matrix for the A8LR1 245mm converter:
	// K(0) = 110.7 RPM / sqrt(Nm); rises asymptotically toward lockup/synchronism
	if nu <= 0.80 {
		kFactor = 110.7 + 74.3*math.Pow(nu/0.80, 2.2)
	} else if nu < 1.00 {
		// Steep rise approaching synchronism
		frac := (nu - 0.80) / 0.20
		kFactor = 185.0 + 350.0*math.Pow(frac, 1.8)
	} else {
		kFactor = 999.9
	}

	return nu, torqueRatio, kFactor
}

// EvalG6DPFrictionLossNm evaluates the total mechanical friction and pumping losses (FMEP + PMEP)
// for the 3.3L Twin-Turbo V6 (G6DP) as a function of engine speed (RPM) and engine coolant/oil temperature (C).
// Formula: T_loss = (C0 + C1*N + C2*N^2) * f(T_coolant)
// Correlated to SIM2K-260 internal loss model at 85C nominal operating temperature.
func EvalG6DPFrictionLossNm(rpm, coolantC float64) float64 {
	clampedRPM := math.Max(0.0, math.Min(7200.0, rpm))

	// Nominal base friction polynomial at 85 deg C
	baseNm := 34.0 + 0.0045*clampedRPM + 1.0e-6*clampedRPM*clampedRPM

	// Coolant & oil temperature viscosity correction factor:
	// Normalized so f(85.0) == 1.000 exactly.
	// Low temperature increases kinematic oil viscosity and hydrodynamic drag.
	tempCorrection := 1.00
	if coolantC < 85.0 {
		// Sub-85C viscous penalty: ~35% higher friction at 20C ambient cold start
		tempDelta := 85.0 - coolantC
		tempCorrection = 1.0 + 0.0062*tempDelta + 0.000045*tempDelta*tempDelta
	} else if coolantC > 85.0 {
		// Hot oil viscosity reduction clamped to realistic hydrodynamic minimum
		tempDelta := coolantC - 85.0
		tempCorrection = math.Max(0.88, 1.0-0.0035*tempDelta)
	}

	return baseNm * tempCorrection
}
