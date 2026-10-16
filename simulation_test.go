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

