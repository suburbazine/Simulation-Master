package main

// BinFileItem describes a discovered ECU, TCU, or AWD calibration binary on disk.
type BinFileItem struct {
	ID          string `json:"id"`
	Module      string `json:"module"` // "ecu", "tcu", "awd"
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	Path        string `json:"path"`
	SizeBytes   int64  `json:"size_bytes"`
	BaseAddrHex string `json:"base_addr_hex"`
	Description string `json:"description"`
	Tag         string `json:"tag"` // "STOCK", "TUNED", "RACE", "DIAG"
}

// DatabaseItem describes a linked BinGo SQLite teardown database.
type DatabaseItem struct {
	Module    string `json:"module"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	MapCount  int    `json:"map_count"`
	FuncCount int    `json:"func_count"`
	XrefCount int    `json:"xref_count"`
}

// SurfacePreset defines Pacejka Magic Formula tire-ground friction parameters.
type SurfacePreset struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	MuPeak      float64 `json:"mu_peak"`      // Peak friction D
	MuSlide     float64 `json:"mu_slide"`     // High-slip asymptote friction
	StiffnessB  float64 `json:"stiffness_b"`  // Pacejka B stiffness factor
	ShapeC      float64 `json:"shape_c"`      // Pacejka C shape factor
	CurvatureE  float64 `json:"curvature_e"`  // Pacejka E curvature factor
	KappaPeak   float64 `json:"kappa_peak"`   // Slip ratio at peak mu (e.g. 0.11 = 11%)
	RollingRes  float64 `json:"rolling_res"`  // Rolling resistance coefficient Cr
}

// ScenarioPreset provides one-click A/B comparison setups demonstrating real teardown findings.
type ScenarioPreset struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Highlight   string            `json:"highlight"`
	Environment EnvironmentConfig `json:"environment"`
	RunA        RunConfig         `json:"run_a"`
	RunB        RunConfig         `json:"run_b"`
}

// InventoryResponse is returned by GET /api/inventory.
type InventoryResponse struct {
	ECUBins   []BinFileItem    `json:"ecu_bins"`
	TCUBins   []BinFileItem    `json:"tcu_bins"`
	AWDBins   []BinFileItem    `json:"awd_bins"`
	Databases []DatabaseItem   `json:"databases"`
	Surfaces  []SurfacePreset  `json:"surfaces"`
	Presets   []ScenarioPreset `json:"presets"`
}

// EnvironmentConfig controls road surface, atmosphere, cooling airflow restriction, and vehicle chassis parameters.
type EnvironmentConfig struct {
	SurfaceID             string  `json:"surface_id"`
	CustomMuPeak          float64 `json:"custom_mu_peak"`            // Optional override if > 0
	CustomMuSlide         float64 `json:"custom_mu_slide"`           // Optional override if > 0
	CustomKappaPeak       float64 `json:"custom_kappa_peak"`         // Optional override if > 0
	RoadGradePct          float64 `json:"road_grade_pct"`            // Grade in %
	AmbientTempC          float64 `json:"ambient_temp_c"`            // Ambient temperature in C
	CoolantTempC          float64 `json:"coolant_temp_c"`            // Engine coolant temp in C (default 85C)
	AirflowRestrictionPct float64 `json:"airflow_restriction_pct"`   // Cooling/FMIC/Underbody airflow restriction % (0 = open, 60 = heat-soaked/blocked)
	AltitudeM             float64 `json:"altitude_m"`                // Altitude above sea level in meters
	HeadwindKmh           float64 `json:"headwind_kmh"`              // Headwind speed in km/h
	VehicleMassKg         float64 `json:"vehicle_mass_kg"`           // Curb + driver mass (default 1915 kg)
	CGHeightM             float64 `json:"cg_height_m"`               // Center of gravity height (default 0.51 m)
	StaticFrontPct        float64 `json:"static_front_pct"`          // Static front weight distribution % (default 52.0)
	FrontTireRadius       float64 `json:"front_tire_radius"`         // Physical front rolling radius in meters (default 0.334)
	RearTireRadius        float64 `json:"rear_tire_radius"`          // Physical rear rolling radius in meters (default 0.336)
	SimDurationSec        float64 `json:"sim_duration_sec"`          // Max simulation duration after brake release (default 18.0 s)
}

// OverrideConfig allows non-destructive live A/B what-if tuning on top of any loaded binary.
type OverrideConfig struct {
	// ECU overrides
	TorqueScalePct       float64  `json:"torque_scale_pct"`        // 100 = binary native, e.g. 115 = +15% airflow/boost
	SparkAdvanceAdderDeg float64  `json:"spark_advance_adder_deg"` // Direct WOT spark timing adder in deg BTDC (-6..+8)
	BoostAdderPsi        float64  `json:"boost_adder_psi"`         // Direct boost target adder in psi (-6..+8)
	CamOverlapAdderDeg   float64  `json:"cam_overlap_adder_deg"`   // Dual-CVVT valve overlap bias in deg CA (-10..+10)
	ShiftSparkFloorDeg   *float64 `json:"shift_spark_floor_deg"`   // Override 0x207AD9 shift spark floor (deg BTDC)
	TCSInhibit           *int     `json:"tcs_inhibit"`             // Override 0x200B4B (0=Stock TCS, 1=TCS Inhibited)
	TCHASurgeFix         *bool    `json:"tcha_surge_fix"`          // Force disable 0x203EC6 turbo overspeed torque cut
	RALAntilagActive     *bool    `json:"ral_antilag_active"`      // Force enable/disable 0x808DC000 Rolling Antilag brake-boost

	// TCU overrides
	LaunchStallRPM    *float64 `json:"launch_stall_rpm"`     // Override 0x238B68 brake-stall RPM limit
	TorqueCapNm       *float64 `json:"torque_cap_nm"`        // Override 0x1FD198 / 0x2096E4 TCU torque limit
	ShiftSpeedDeltaKm float64  `json:"shift_speed_delta_km"` // Offset WOT upshift speeds (km/h)
	ShiftCutScalePct  float64  `json:"shift_cut_scale_pct"`  // Scale 0x233D3C shift intervention (100 = binary native)
	ShiftDurationMs   *float64 `json:"shift_duration_ms"`    // Override inertia phase duration in ms

	// AWD overrides
	BypassBrakeGate    *bool    `json:"bypass_brake_gate"`     // Bypass 0xC07498 / 0xC06080 64.5 psi brake inhibit
	LaunchPreloadNm    *float64 `json:"launch_preload_nm"`     // Override 1st gear launch preload (0xC04778)
	AWDSlewRateNmPerS  *float64 `json:"awd_slew_rate_nm_s"`    // Override 0xC07650 positive slew rate (Nm/s)
	FrontSplitScalePct float64  `json:"front_split_scale_pct"` // Scale 0xC0585A / 0xC05922 front split % (100 = native)
	Hold1000NmToKmh    *float64 `json:"hold_1000nm_to_kmh"`    // Hold 1000 Nm feed-forward up to speed (km/h)
}

// RunConfig defines a complete ECU + TCU + AWD setup and launch procedure for one simulation slot.
type RunConfig struct {
	Label            string         `json:"label"`
	ECUBinPath       string         `json:"ecu_bin_path"`
	TCUBinPath       string         `json:"tcu_bin_path"`
	AWDBinPath       string         `json:"awd_bin_path"`
	FuelType         string         `json:"fuel_type"`          // "93_aki", "91_aki", "93_wmi", "e30", "e50", "e85"
	TurboPreset      string         `json:"turbo_preset"`       // "stock_mgt1549", "stage1_41_52", "stage2_44_56", "stage3_47_60", "custom"
	CompInducerMm    float64        `json:"comp_inducer_mm"`    // Compressor inducer diameter in mm (stock MGT1549DSL = 37.2 mm)
	CompExducerMm    float64        `json:"comp_exducer_mm"`    // Compressor exducer diameter in mm (stock MGT1549DSL = 49.0 mm)
	TurbInducerMm    float64        `json:"turb_inducer_mm"`    // Turbine inducer diameter in mm (stock MGT1549DSL = 40.0 mm)
	TurbExducerMm    float64        `json:"turb_exducer_mm"`    // Turbine exducer diameter in mm (stock MGT1549DSL = 35.5 mm)
	ExhaustType      string         `json:"exhaust_type"`       // "stock_full_cats", "sec_cat_delete", "high_flow_catted", "catless_stock_cb", "catless_full_race", "open_dump_race"
	DriveMode        string         `json:"drive_mode"`         // "sport", "comfort", "eco", "custom_m4"
	ESCMode          string         `json:"esc_mode"`           // "on", "sport", "off"
	PreStageTimeSec  float64        `json:"pre_stage_time_sec"` // Brake-stand build time before launch (e.g. 1.5 s)
	BrakeHoldPsi     float64        `json:"brake_hold_psi"`     // Brake pressure held during pre-stage (e.g. 1200 psi)
	BrakeReleaseMs   float64        `json:"brake_release_ms"`   // Hydraulic release time from hold to 0 psi (e.g. 140 ms)
	PedalPct         float64        `json:"pedal_pct"`          // Throttle pedal target % (default 100%)
	RolloutInch      float64        `json:"rollout_inch"`       // Drag strip staging beam rollout (0 or 12 inches)
	Overrides        OverrideConfig `json:"overrides"`
}

// SimRequest is posted to POST /api/simulate.
type SimRequest struct {
	Environment EnvironmentConfig `json:"environment"`
	RunA        RunConfig         `json:"run_a"`
	RunB        *RunConfig        `json:"run_b,omitempty"`
}

// ExtractedCalSummary exposes the actual decoded binary calibration parameters used by the physics solver.
type ExtractedCalSummary struct {
	// ECU
	ECUBinName            string      `json:"ecu_bin_name"`
	ECUDriveModeForce     int         `json:"ecu_drive_mode_force"`      // 0x200CCE
	ECUDriverDemandPeakNm float64     `json:"ecu_driver_demand_peak_nm"` // 0x214E80 / 0x214280
	ECUTQIBasPeakNm       float64     `json:"ecu_tqi_bas_peak_nm"`       // 0x255518 peak
	ECUTQIMaxPeakNm       float64     `json:"ecu_tqi_max_peak_nm"`       // 0x253D14.. peak
	ECUClutchLimitNm      float64     `json:"ecu_clutch_limit_nm"`       // 0x24CE7C
	ECUBoostTargetHpa     float64     `json:"ecu_boost_target_hpa"`      // 0x21EC00
	ECUBoostLimitAHpa     float64     `json:"ecu_boost_limit_a_hpa"`     // 0x21F478
	ECUTCHALimitActive    bool        `json:"ecu_tcha_limit_active"`     // 0x203EC6 != 65535
	ECUTCHADebounceCyc    int         `json:"ecu_tcha_debounce_cyc"`     // 0x203EC6
	ECUTCHAMinTorqueNm    float64     `json:"ecu_tcha_min_torque_nm"`    // 0x203ED4
	ECUTCHARampNmPerCyc   float64     `json:"ecu_tcha_ramp_nm_per_cyc"`  // 0x203ED8
	ECUShiftSparkFloorDeg float64     `json:"ecu_shift_spark_floor_deg"` // 0x207AD9
	ECUIGAMinWarmWOTDeg   float64     `json:"ecu_iga_min_warm_wot_deg"`  // 0x239AF4 warm row
	ECUTCSInhibit         int         `json:"ecu_tcs_inhibit"`           // 0x200B4B
	ECURALPatchDetected   bool        `json:"ecu_ral_patch_detected"`    // 0x808DC000
	ECUWOTSparkCurveDeg   []float64   `json:"ecu_wot_spark_curve_deg"`   // 18 RPM points at WOT (0x238090)
	ECUWOTTorqueCurveNm   []float64   `json:"ecu_wot_torque_curve_nm"`   // 18 RPM points at WOT
	ECURPMAxis            []float64   `json:"ecu_rpm_axis"`              // 18 RPM points (550..6600)

	// TCU
	TCUBinName            string    `json:"tcu_bin_name"`
	TCULaunchStallRPM     float64   `json:"tcu_launch_stall_rpm"`      // 0x238B68
	TCUPrimaryTorqueCapNm float64   `json:"tcu_primary_torque_cap_nm"` // 0x1FD198
	TCUPerGearCapNm       []float64 `json:"tcu_per_gear_cap_nm"`       // 0x1FD1B0 (8 gears)
	TCUGlobalTorqueCapNm  float64   `json:"tcu_global_torque_cap_nm"`  // 0x2096E4
	TCUClutchLimitPeakNm  float64   `json:"tcu_clutch_limit_peak_nm"`  // 0x1FBC5C
	TCUWOTUpshiftKmh      []float64 `json:"tcu_wot_upshift_kmh"`       // 7 upshifts (1->2 .. 7->8) for active mode
	TCUComfortUpshiftKmh  []float64 `json:"tcu_comfort_upshift_kmh"`   // Slot 3 WOT upshifts
	TCUSportUpshiftKmh    []float64 `json:"tcu_sport_upshift_kmh"`     // Slot 23 WOT upshifts
	TCUShiftCutRaw        int       `json:"tcu_shift_cut_raw"`         // 0x233D3C (-10 stock vs -15 EKM)
	TCUShiftTimerMs       int       `json:"tcu_shift_timer_ms"`        // 0x20FC52 (65535 stock vs 90 EKM)
	TCUVCMSolenoidFloorMa int       `json:"tcu_vcm_solenoid_floor_ma"` // 0x24BB22 (850 stock vs 490 EKM)

	// AWD
	AWDBinName            string    `json:"awd_bin_name"`
	AWDTireRadiusMm       float64   `json:"awd_tire_radius_mm"`        // 0xC04214 (320 mm)
	AWDBaseClampNm        float64   `json:"awd_base_clamp_nm"`         // 0xC04334 (150 Nm)
	AWDLaunchPreload1stNm []float64 `json:"awd_launch_preload_1st_nm"` // 5 pedal pts [10,20,30,50,80]%
	AWDOffIdlePreloadNm   []float64 `json:"awd_off_idle_preload_nm"`   // 4 pts
	AWDFeedForwardWOTNm   []float64 `json:"awd_feed_forward_wot_nm"`   // 10 speed pts at WOT
	AWDFeedForwardAxisKmh []float64 `json:"awd_feed_forward_axis_kmh"` // 10 speed pts
	AWDFrontSplitWOTPct   []float64 `json:"awd_front_split_wot_pct"`   // 5 speed pts at WOT
	AWDMaxCeilingNm       float64   `json:"awd_max_ceiling_nm"`        // 0xC059EA / 0xC05A1C
	AWDBrakeGateBar       float64   `json:"awd_brake_gate_bar"`        // 0xC07498 (4.4 bar = 63.8 psi)
	AWDBrakeGateBypassed  bool      `json:"awd_brake_gate_bypassed"`   // 0xC06080 zeroed
	AWDSlewRateNmPer20ms  float64   `json:"awd_slew_rate_nm_20ms"`     // 0xC07650 (120 vs 180 Nm/20ms)
	AWDSlewRateNmPerSec   float64   `json:"awd_slew_rate_nm_sec"`      // 6000 vs 9000 Nm/s
}

// PerformanceSummary holds standard drag strip & acceleration metrics for a run.
type PerformanceSummary struct {
	ZeroTo60MphSec         float64 `json:"zero_to_60_mph_sec"`
	ZeroTo100KmhSec        float64 `json:"zero_to_100_kmh_sec"`
	ZeroTo100MphSec        float64 `json:"zero_to_100_mph_sec"`
	ZeroTo200KmhSec        float64 `json:"zero_to_200_kmh_sec"`
	HundredTo200KmhSec     float64 `json:"hundred_to_200_kmh_sec"`
	SixtyTo130MphSec       float64 `json:"sixty_to_130_mph_sec"`
	SixtyFootSec           float64 `json:"sixty_foot_sec"`
	ThreeThirtyFootSec     float64 `json:"three_thirty_foot_sec"`
	EighthMileSec          float64 `json:"eighth_mile_sec"`
	EighthMileMph          float64 `json:"eighth_mile_mph"`
	ThousandFootSec        float64 `json:"thousand_foot_sec"`
	QuarterMileSec         float64 `json:"quarter_mile_sec"`
	QuarterMileMph         float64 `json:"quarter_mile_mph"`
	PeakAccelG             float64 `json:"peak_accel_g"`
	PeakEngineTorqueNm     float64 `json:"peak_engine_torque_nm"`
	PeakWheelPowerHp       float64 `json:"peak_wheel_power_hp"`
	PeakAWDCouplingNm      float64 `json:"peak_awd_coupling_nm"`
	LaunchAWDTorqueNm      float64 `json:"launch_awd_torque_nm"` // AWD torque at t = +0.05s after release
	MaxFrontSlipPct        float64 `json:"max_front_slip_pct"`
	MaxRearSlipPct         float64 `json:"max_rear_slip_pct"`
	TCSEventsCount         int     `json:"tcs_events_count"`
	TCHACutsCount          int     `json:"tcha_cuts_count"`
	PeakEMAPPsi            float64 `json:"peak_emap_psi"`
	PeakExhBackpressurePsi float64 `json:"peak_exh_backpressure_psi"`
	PeakTurboSpeedKRPM     float64 `json:"peak_turbo_krpm"`
	PeakEGTC               float64 `json:"peak_egt_c"`
	PeakTransTempC         float64 `json:"peak_trans_c"`
	PeakAWDTempC           float64 `json:"peak_awd_c"`
	PeakRearTireTempC      float64 `json:"peak_tire_r_c"`
	PeakStressEnginePct    float64 `json:"peak_stress_engine_pct"`
	PeakStressTransPct     float64 `json:"peak_stress_trans_pct"`
	PeakStressAWDPct       float64 `json:"peak_stress_awd_pct"`
	PeakStressRearDrivePct float64 `json:"peak_stress_drive_r_pct"`
	TopSpeedKmh            float64 `json:"top_speed_kmh"`
	TopSpeedMph            float64 `json:"top_speed_mph"`
}

// ShiftEvent captures detailed firmware & physical telemetry across each gear change.
type ShiftEvent struct {
	FromGear          int     `json:"from_gear"`
	ToGear            int     `json:"to_gear"`
	StartTimeSec      float64 `json:"start_time_sec"`
	DurationMs        float64 `json:"duration_ms"`
	TriggerSpeedKmh   float64 `json:"trigger_speed_kmh"`
	TriggerSpeedMph   float64 `json:"trigger_speed_mph"`
	EntryRPM          float64 `json:"entry_rpm"`
	DropToRPM         float64 `json:"drop_to_rpm"`
	TCUTorqueReqNm    float64 `json:"tcu_torque_req_nm"`
	SparkEffTarget    float64 `json:"spark_eff_target"`
	SparkRetardDeg    float64 `json:"spark_retard_deg"`
	MinSparkDeg       float64 `json:"min_spark_deg"`
	SparkFloorClamped bool    `json:"spark_floor_clamped"`
	ActualMinTorqueNm float64 `json:"actual_min_torque_nm"`
	MinAccelG         float64 `json:"min_accel_g"`
	PeakJerkGPerS     float64 `json:"peak_jerk_g_per_s"`
}

// TelemetrySample represents a 100 Hz (10 ms) snapshot of all physical & CAN bus channels.
type TelemetrySample struct {
	TimeSec            float64 `json:"t"`
	DistanceM          float64 `json:"dist_m"`
	DistanceFt         float64 `json:"dist_ft"`
	SpeedKmh           float64 `json:"v_kmh"`
	SpeedMph           float64 `json:"v_mph"`
	AccelG             float64 `json:"accel_g"`
	Gear               int     `json:"gear"`
	ShiftActive        bool    `json:"shift_active"`
	EngineRPM          float64 `json:"rpm"`
	TurbineRPM         float64 `json:"turbine_rpm"`
	PedalPct           float64 `json:"pedal_pct"`
	BrakePsi           float64 `json:"brake_psi"`
	BoostHpa           float64 `json:"boost_hpa"`
	BoostPsi           float64 `json:"boost_psi"`
	BoostTargetPsi     float64 `json:"boost_target_psi"`
	EMAPPsi            float64 `json:"emap_psi"`
	ExhBackpressurePsi float64 `json:"exh_backpressure_psi"`
	DrivePressureRatio float64 `json:"drive_pressure_ratio"`
	TurboSpeedKRPM     float64 `json:"turbo_krpm"`
	CompEfficiency     float64 `json:"eta_comp"`
	AirloadMgStk       float64 `json:"airload_mg"`
	DriverDemandNm     float64 `json:"tqi_drv_nm"`
	UncappedTorqueNm   float64 `json:"tqi_uncapped_nm"`
	TCUTorqueLimitNm   float64 `json:"tcu_lim_nm"`
	ActualEngineNm     float64 `json:"eng_nm"`
	SparkAdvanceDeg    float64 `json:"iga_deg"`
	SparkFloorDeg      float64 `json:"iga_floor_deg"`
	SparkEfficiency    float64 `json:"eta_iga"`
	KnockRetardDeg     float64 `json:"kr_deg"`
	IntakeCamDeg       float64 `json:"cam_in_deg"`
	ExhaustCamDeg      float64 `json:"cam_ex_deg"`
	CamOverlapDeg      float64 `json:"cam_overlap_deg"`
	Lambda             float64 `json:"lambda"`
	AWDTargetNm        float64 `json:"awd_target_nm"`
	AWDActualNm        float64 `json:"awd_actual_nm"`
	AWDFrontSplitPct   float64 `json:"awd_split_pct"`
	AWDBrakeGated      bool    `json:"awd_brake_gated"`
	FrontPropTorqueNm  float64 `json:"t_front_nm"`
	RearPropTorqueNm   float64 `json:"t_rear_nm"`
	FrontNormalLoadN   float64 `json:"fz_front_n"`
	RearNormalLoadN    float64 `json:"fz_rear_n"`
	FrontWheelKmh      float64 `json:"v_wheel_f_kmh"`
	RearWheelKmh       float64 `json:"v_wheel_r_kmh"`
	FrontSlipPct       float64 `json:"slip_f_pct"`
	RearSlipPct        float64 `json:"slip_r_pct"`
	FrontMu            float64 `json:"mu_f"`
	RearMu             float64 `json:"mu_r"`
	FrontTractiveN     float64 `json:"fx_front_n"`
	RearTractiveN      float64 `json:"fx_rear_n"`
	TCSActive          bool    `json:"tcs_active"`
	TCHACutActive      bool    `json:"tcha_active"`

	// Component Temperatures (deg C) & Slip Dissipation (kW)
	EngineBlockTempC     float64 `json:"temp_engine_c"`
	ExhaustManifoldTempC float64 `json:"temp_egt_c"`
	IntakeAirTempC       float64 `json:"temp_iat_c"`
	TransFluidTempC      float64 `json:"temp_trans_c"`
	AWDClutchTempC       float64 `json:"temp_awd_c"`
	RearDiffTempC        float64 `json:"temp_rear_diff_c"`
	FrontDiffTempC       float64 `json:"temp_front_diff_c"`
	RearTireTempC        float64 `json:"temp_tire_r_c"`
	FrontTireTempC       float64 `json:"temp_tire_f_c"`
	TCSlipPowerKW        float64 `json:"tc_slip_kw"`
	AWDSlipPowerKW       float64 `json:"awd_slip_kw"`

	// Component Mechanical & Thermal Stress Indices (0..100%)
	StressEnginePct     float64 `json:"stress_engine_pct"`
	StressTransPct      float64 `json:"stress_trans_pct"`
	StressAWDPct        float64 `json:"stress_awd_pct"`
	StressRearDrivePct  float64 `json:"stress_drive_r_pct"`
	StressFrontDrivePct float64 `json:"stress_drive_f_pct"`
	StressRearTirePct   float64 `json:"stress_tire_r_pct"`
	StressFrontTirePct  float64 `json:"stress_tire_f_pct"`
}

//MuSlipCurvePoint represents a point on the active surface's tire friction curve.
type MuSlipCurvePoint struct {
	SlipPct float64 `json:"slip_pct"`
	Mu      float64 `json:"mu"`
}

// RunResult encapsulates everything produced for a single simulation slot (Run A or Run B).
type RunResult struct {
	Label       string              `json:"label"`
	Config      RunConfig           `json:"config"`
	Calibration ExtractedCalSummary `json:"calibration"`
	Summary     PerformanceSummary  `json:"summary"`
	Shifts      []ShiftEvent        `json:"shifts"`
	Telemetry   []TelemetrySample   `json:"telemetry"`
}

// RunDiffSummary highlights the A vs B performance and calibration differences.
type RunDiffSummary struct {
	DeltaZeroTo60MphSec     float64  `json:"delta_zero_to_60_mph_sec"`
	DeltaZeroTo100KmhSec    float64  `json:"delta_zero_to_100_kmh_sec"`
	DeltaHundredTo200KmhSec float64  `json:"delta_hundred_to_200_kmh_sec"`
	DeltaSixtyFootSec       float64  `json:"delta_sixty_foot_sec"`
	DeltaEighthMileSec      float64  `json:"delta_eighth_mile_sec"`
	DeltaQuarterMileSec     float64  `json:"delta_quarter_mile_sec"`
	DeltaQuarterMileMph     float64  `json:"delta_quarter_mile_mph"`
	DeltaPeakTorqueNm       float64  `json:"delta_peak_torque_nm"`
	DeltaPeakWheelHp        float64  `json:"delta_peak_wheel_hp"`
	KeyCalibrationDiffs     []string `json:"key_calibration_diffs"`
}

// SimResponse is returned by POST /api/simulate.
type SimResponse struct {
	Environment EnvironmentConfig  `json:"environment"`
	Surface     SurfacePreset      `json:"surface"`
	MuSlipCurve []MuSlipCurvePoint `json:"mu_slip_curve"`
	RunA        RunResult          `json:"run_a"`
	RunB        *RunResult         `json:"run_b,omitempty"`
	Comparison  *RunDiffSummary    `json:"comparison,omitempty"`
}

// CalMapRecord represents a map entry from one of the three BinGo SQLite databases.
type CalMapRecord struct {
	Module       string  `json:"module"`
	Name         string  `json:"name"`
	AddressHex   string  `json:"address_hex"`
	AddressInt   uint64  `json:"address_int"`
	Rows         int     `json:"rows"`
	Cols         int     `json:"cols"`
	ElementType  string  `json:"element_type"`
	XAxisAddrHex string  `json:"x_axis_addr_hex"`
	YAxisAddrHex string  `json:"y_axis_addr_hex"`
	Units        string  `json:"units"`
	ScaleFactor  float64 `json:"scale_factor"`
	OffsetVal    float64 `json:"offset_val"`
	Category     string  `json:"category"`
	Description  string  `json:"description"`
	Confidence   string  `json:"confidence"`
}

// DecodedMapGrid holds the decoded 2D physical values for a calibration map from a specific binary.
type DecodedMapGrid struct {
	BinName    string      `json:"bin_name"`
	BinPath    string      `json:"bin_path"`
	FileOffset int         `json:"file_offset"`
	XAxis      []float64   `json:"x_axis"`
	YAxis      []float64   `json:"y_axis"`
	Values     [][]float64 `json:"values"`
	MinVal     float64     `json:"min_val"`
	MaxVal     float64     `json:"max_val"`
}

// MapCompareResponse is returned by GET /api/maps/compare.
type MapCompareResponse struct {
	Map         CalMapRecord    `json:"map"`
	GridA       *DecodedMapGrid `json:"grid_a,omitempty"`
	GridB       *DecodedMapGrid `json:"grid_b,omitempty"`
	DeltaValues [][]float64     `json:"delta_values,omitempty"`
	MaxAbsDelta float64         `json:"max_abs_delta"`
	ChangedCells int            `json:"changed_cells"`
}
