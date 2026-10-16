package main

import (
	"testing"
)

func TestBinaryDiscoveryAndPresets(t *testing.T) {
	ecuBins, tcuBins, awdBins := DiscoverBinaries()
	if len(ecuBins) == 0 {
		t.Fatalf("Expected to discover SIM2K-260 ECU binaries, found 0")
	}
	if len(tcuBins) == 0 {
		t.Fatalf("Expected to discover Kefico CPTSH TCU binaries, found 0")
	}
	if len(awdBins) == 0 {
		t.Fatalf("Expected to discover Magna ATC AWD binaries, found 0")
	}
	t.Logf("Discovered %d ECU bins, %d TCU bins, %d AWD bins", len(ecuBins), len(tcuBins), len(awdBins))

	dbs := DiscoverDatabases()
	for _, db := range dbs {
		if !db.Available {
			t.Errorf("Expected database %s (%s) to be available", db.Module, db.Path)
		}
		t.Logf("DB [%s]: %d maps, %d funcs, %d xrefs", db.Module, db.MapCount, db.FuncCount, db.XrefCount)
	}

	presets := GetScenarioPresets(ecuBins, tcuBins, awdBins)
	for _, p := range presets {
		runB := p.RunB
		resp, err := ExecuteSimulation(SimRequest{
			Environment: p.Environment,
			RunA:        p.RunA,
			RunB:        &runB,
		})
		if err != nil {
			t.Fatalf("Preset %s failed simulation: %v", p.ID, err)
		}
		sa := resp.RunA.Summary
		sb := resp.RunB.Summary
		t.Logf("=== PRESET: %s ===", p.Name)
		t.Logf("  Run A (%s): 60ft=%.2fs | 0-60mph=%.2fs | 0-100kmh=%.2fs | 1/4mi=%.2fs @ %.1f mph | LaunchAWD=%.0f Nm | Peak=%.0f Nm / %.0f whp | TCHA_Cuts=%d",
			resp.RunA.Label, sa.SixtyFootSec, sa.ZeroTo60MphSec, sa.ZeroTo100KmhSec, sa.QuarterMileSec, sa.QuarterMileMph,
			sa.LaunchAWDTorqueNm, sa.PeakEngineTorqueNm, sa.PeakWheelPowerHp, sa.TCHACutsCount)
		t.Logf("  Run B (%s): 60ft=%.2fs | 0-60mph=%.2fs | 0-100kmh=%.2fs | 1/4mi=%.2fs @ %.1f mph | LaunchAWD=%.0f Nm | Peak=%.0f Nm / %.0f whp | TCHA_Cuts=%d",
			resp.RunB.Label, sb.SixtyFootSec, sb.ZeroTo60MphSec, sb.ZeroTo100KmhSec, sb.QuarterMileSec, sb.QuarterMileMph,
			sb.LaunchAWDTorqueNm, sb.PeakEngineTorqueNm, sb.PeakWheelPowerHp, sb.TCHACutsCount)
	}
}

func TestMapCompareAcrossBins(t *testing.T) {
	ecuBins, _, awdBins := DiscoverBinaries()
	if len(ecuBins) >= 2 {
		cmp, err := CompareMapAcrossBins("ecu", "0x80855518", ecuBins[0].Path, ecuBins[1].Path)
		if err != nil {
			t.Fatalf("CompareMapAcrossBins ECU failed: %v", err)
		}
		t.Logf("ECU Map %s (%s): %dx%d, MaxAbsDelta=%.2f, ChangedCells=%d",
			cmp.Map.Name, cmp.Map.AddressHex, cmp.Map.Rows, cmp.Map.Cols, cmp.MaxAbsDelta, cmp.ChangedCells)
	}
	if len(awdBins) >= 2 {
		awd0, err := DecodeAWDProfile(awdBins[0].Path)
		if err != nil {
			t.Fatalf("DecodeAWDProfile 0 failed: %v", err)
		}
		awd1, err := DecodeAWDProfile(awdBins[1].Path)
		if err != nil {
			t.Fatalf("DecodeAWDProfile 1 failed: %v", err)
		}
		t.Logf("AWD0 (%s): GateBypass=%v, Slew=%.1f Nm/20ms, Launch1st=%v, FFSportWOT=%v, SplitSportWOT=%v",
			awd0.BinName, awd0.BrakeGateBypassed, awd0.SlewRateNmPer20ms, awd0.LaunchPreloadSport[1], awd0.FFSportMapNm[0], awd0.SplitSportPct[0])
		t.Logf("AWD1 (%s): GateBypass=%v, Slew=%.1f Nm/20ms, Launch1st=%v, FFSportWOT=%v, SplitMod38=%v",
			awd1.BinName, awd1.BrakeGateBypassed, awd1.SlewRateNmPer20ms, awd1.LaunchPreloadSport[1], awd1.FFSportMapNm[0], awd1.SplitMod38Pct[0])
	}
	_, tcuBins, _ := DiscoverBinaries()
	for i := 0; i < len(tcuBins) && i < 3; i++ {
		tp, _ := DecodeTCUProfile(tcuBins[i].Path)
		t.Logf("TCU[%d] (%s): Stall=%.0f, PrimaryCap=%.0f, GlobalCap=%.0f, PerGear=%v, ShiftCut=%d, ShiftTimer=%d, VCM=%d",
			i, tp.BinName, tp.LaunchStallRPM, tp.PrimaryTqCapNm, tp.GlobalTqCapNm, tp.PerGearCapNm, tp.ShiftCutRaw, tp.ShiftTimerMs, tp.VCMSolenoidFloor)
	}
	for i := 0; i < len(ecuBins) && i < 6; i++ {
		ep, _ := DecodeECUProfile(ecuBins[i].Path)
		t.Logf("ECU[%d] (%s): TCHA_Active=%v, Debounce=%d, Inhibit=%d, MinTq=%.1f, Ramp=%.1f, TQIBasPeak=%.1f, SparkFloor=%.1f, TCS=%d, RAL=%v",
			i, ep.BinName, ep.TCHALimitActive, ep.TCHADebounceCyc, ep.TCHAInhibitSwitch, ep.TCHAMinTorqueNm, ep.TCHARampNmPerCyc, ep.TQIBasPeak, ep.ShiftSparkFloor, ep.TCSInhibit, ep.RALPatchDetected)
	}
}

func TestStorageTreeInitialization(t *testing.T) {
	if err := EnsureStorageTree(); err != nil {
		t.Fatalf("EnsureStorageTree failed: %v", err)
	}
	status := GetStorageTreeStatus()
	if len(status.Directories) != 6 {
		t.Fatalf("Expected 6 organized storage directories (db/{ecu,tcu,awd} and bins/{ecu,tcu,awd}), got %d", len(status.Directories))
	}
	for _, d := range status.Directories {
		t.Logf("Storage folder [%s] (%s/%s): %d files", d.RelPath, d.Category, d.Module, len(d.Files))
	}
}

func TestFuelTypeCamSparkAndBoostModeling(t *testing.T) {
	ecuBins, tcuBins, awdBins := DiscoverBinaries()
	if len(ecuBins) < 3 || len(tcuBins) < 2 || len(awdBins) < 2 {
		t.Skip("insufficient bins for fuel/cam/spark/boost test")
	}

	env := EnvironmentConfig{
		SurfaceID:       "dry_asphalt_street",
		VehicleMassKg:   1915,
		AmbientTempC:    22,
		CoolantTempC:    85,
		FrontTireRadius: 0.334,
		RearTireRadius:  0.334,
		SimDurationSec:  13.5,
	}

	// Compare KimJongTuned BEF (+4.5 deg aggressive spark) on 91 AKI vs E85
	baseCfg := RunConfig{
		Label:           "KJT BEF 91 AKI",
		ECUBinPath:      ecuBins[2].Path, // KIMJONGTUNED_BEF
		TCUBinPath:      tcuBins[1].Path, // EKM Tuned
		AWDBinPath:      awdBins[1].Path, // SportLaunch
		DriveMode:       "sport",
		ESCMode:         "sport",
		FuelType:        "91_aki",
		PreStageTimeSec: 1.4,
		BrakeHoldPsi:    1200,
		BrakeReleaseMs:  140,
		PedalPct:        100,
		Overrides:       OverrideConfig{TorqueScalePct: 100},
	}

	e85Cfg := baseCfg
	e85Cfg.Label = "KJT BEF E85 + 2.5 psi Boost + 6 deg Cam Overlap"
	e85Cfg.FuelType = "e85"
	e85Cfg.Overrides.BoostAdderPsi = 2.5
	e85Cfg.Overrides.CamOverlapAdderDeg = 6.0

	resp, err := ExecuteSimulation(SimRequest{
		Environment: env,
		RunA:        baseCfg,
		RunB:        &e85Cfg,
	})
	if err != nil {
		t.Fatalf("ExecuteSimulation failed: %v", err)
	}
	res91 := resp.RunA
	resE85 := *resp.RunB

	var maxKR91, maxKRE85, maxOverlap91, maxOverlapE85, maxBoost91, maxBoostE85 float64
	for _, s := range res91.Telemetry {
		if s.KnockRetardDeg > maxKR91 {
			maxKR91 = s.KnockRetardDeg
		}
		if s.CamOverlapDeg > maxOverlap91 {
			maxOverlap91 = s.CamOverlapDeg
		}
		if s.BoostPsi > maxBoost91 {
			maxBoost91 = s.BoostPsi
		}
	}
	for _, s := range resE85.Telemetry {
		if s.KnockRetardDeg > maxKRE85 {
			maxKRE85 = s.KnockRetardDeg
		}
		if s.CamOverlapDeg > maxOverlapE85 {
			maxOverlapE85 = s.CamOverlapDeg
		}
		if s.BoostPsi > maxBoostE85 {
			maxBoostE85 = s.BoostPsi
		}
	}

	t.Logf("KJT BEF on 91 AKI: 0-60=%.2fs | 1/4mi=%.2fs @ %.1f mph | Peak=%.0f Nm / %.0f whp | MaxBoost=%.1f psi | MaxKR=%.1f deg | MaxOverlap=%.1f degCA",
		res91.Summary.ZeroTo60MphSec, res91.Summary.QuarterMileSec, res91.Summary.QuarterMileMph, res91.Summary.PeakEngineTorqueNm, res91.Summary.PeakWheelPowerHp, maxBoost91, maxKR91, maxOverlap91)
	t.Logf("KJT BEF on E85 (+2.5psi, +6deg Overlap): 0-60=%.2fs | 1/4mi=%.2fs @ %.1f mph | Peak=%.0f Nm / %.0f whp | MaxBoost=%.1f psi | MaxKR=%.1f deg | MaxOverlap=%.1f degCA",
		resE85.Summary.ZeroTo60MphSec, resE85.Summary.QuarterMileSec, resE85.Summary.QuarterMileMph, resE85.Summary.PeakEngineTorqueNm, resE85.Summary.PeakWheelPowerHp, maxBoostE85, maxKRE85, maxOverlapE85)

	if maxKR91 <= maxKRE85 {
		t.Fatalf("Expected higher knock retard on 91 AKI (%.2f deg) than E85 (%.2f deg)", maxKR91, maxKRE85)
	}
	if maxOverlapE85 <= maxOverlap91 {
		t.Fatalf("Expected higher cam overlap with +6 deg adder (%.2f vs %.2f)", maxOverlapE85, maxOverlap91)
	}
	if resE85.Summary.PeakWheelPowerHp <= res91.Summary.PeakWheelPowerHp {
		t.Fatalf("Expected higher peak wheel HP on E85 + boost (%.1f vs %.1f)", resE85.Summary.PeakWheelPowerHp, res91.Summary.PeakWheelPowerHp)
	}
}


