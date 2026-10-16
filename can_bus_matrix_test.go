package main

import (
	"strings"
	"testing"
)

func TestCCANMatrixCatalog(t *testing.T) {
	specs := GetCCANMatrixCatalog()
	if len(specs) != 9 {
		t.Fatalf("Expected 9 C-CAN frame specs, got %d", len(specs))
	}

	expectedIDs := []uint32{0x080, 0x153, 0x162, 0x164, 0x220, 0x316, 0x329, 0x386, 0x4A0}
	for i, exp := range expectedIDs {
		if specs[i].ArbitrationID != exp {
			t.Errorf("Index %d: expected ArbID 0x%03X, got 0x%03X", i, exp, specs[i].ArbitrationID)
		}
		if len(specs[i].Signals) == 0 {
			t.Errorf("Frame %s has 0 signals defined", specs[i].Name)
		}
		if specs[i].DLC != 8 {
			t.Errorf("Frame %s expected DLC 8, got %d", specs[i].Name, specs[i].DLC)
		}
		t.Logf("Frame [0x%03X] %-10s (%2d ms) | %-20s -> %v | %d signals",
			specs[i].ArbitrationID, specs[i].Name, specs[i].PeriodMs,
			specs[i].Producer, specs[i].Receivers, len(specs[i].Signals))
	}
}

func TestEncodeTelemetryToCCANFrames(t *testing.T) {
	sample := TelemetrySample{
		TimeSec:          0.050,
		DistanceM:        0.45,
		DistanceFt:       1.48,
		SpeedKmh:         18.5,
		SpeedMph:         11.5,
		AccelG:           1.12,
		Gear:             1,
		ShiftActive:      false,
		EngineRPM:        2650.0,
		TurbineRPM:       1420.0,
		PedalPct:         100.0,
		BrakePsi:         0.0,
		BoostHpa:         2150.0,
		BoostPsi:         16.5,
		AirloadMgStk:     1120.0,
		DriverDemandNm:   750.0,
		UncappedTorqueNm: 710.0,
		TCUTorqueLimitNm: 700.0,
		ActualEngineNm:   710.0,
		SparkAdvanceDeg:  14.5,
		SparkFloorDeg:    -6.0,
		SparkEfficiency:  1.0,
		AWDTargetNm:      1000.0,
		AWDActualNm:      850.0,
		AWDFrontSplitPct: 40.0,
		AWDBrakeGated:    false,
		FrontWheelKmh:    18.6,
		RearWheelKmh:     19.2,
		FrontSlipPct:     3.5,
		RearSlipPct:      7.2,
		TCSActive:        false,
		TCHACutActive:    false,
	}

	cal := ExtractedCalSummary{
		ECUBinName:            "IntensePop_v103_Pops-10.bin",
		ECUShiftSparkFloorDeg: -6.0,
		ECUTCSInhibit:         0,
		ECUTCHALimitActive:    false,
		TCUBinName:            "tcu_cptsh_EKM422402108_tuned.bin",
		TCULaunchStallRPM:     3500.0,
		TCUPrimaryTorqueCapNm: 700.0,
		TCUVCMSolenoidFloorMa: 490,
		AWDBinName:            "awd_95447_4J105_SportLaunch_30mph.bin",
		AWDTireRadiusMm:       320.0,
		AWDBrakeGateBar:       4.45,
		AWDBrakeGateBypassed:  true,
		AWDSlewRateNmPer20ms:  180.0,
	}

	frames := EncodeTelemetryToCCANFrames(sample, cal, "sport")
	if len(frames) != 9 {
		t.Fatalf("Expected 9 encoded CAN frames, got %d", len(frames))
	}

	for _, f := range frames {
		if len(f.DataHex) != 16 {
			t.Errorf("Frame %s: invalid hex payload length %d (%s)", f.FrameName, len(f.DataHex), f.DataHex)
		}
		if !strings.HasPrefix(f.CandumpLine, "(") || !strings.Contains(f.CandumpLine, "can0") {
			t.Errorf("Frame %s: invalid candump format: %s", f.FrameName, f.CandumpLine)
		}
		t.Logf("Candump: %s  | %s", f.CandumpLine, f.FrameName)
	}

	// Verify AWD coupling torque values in 0x220
	awdFrame := frames[4] // 0x220
	if awdFrame.ArbitrationID != 0x220 {
		t.Fatalf("Expected index 4 to be 0x220, got 0x%03X", awdFrame.ArbitrationID)
	}
	if awdFrame.DecodedSignals["TQ_AWD_CMD_NM"] != 1000.0 {
		t.Errorf("Expected TQ_AWD_CMD_NM = 1000.0, got %v", awdFrame.DecodedSignals["TQ_AWD_CMD_NM"])
	}

	// Verify Shift Spark Floor Clamp detection in 0x329 during upshift
	sampleShift := sample
	sampleShift.ShiftActive = true
	sampleShift.Gear = 2
	sampleShift.SparkFloorDeg = 12.0
	sampleShift.SparkAdvanceDeg = 12.0 // clamped!
	sampleShift.TCUTorqueLimitNm = 250.0

	framesShift := EncodeTelemetryToCCANFrames(sampleShift, cal, "sport")
	ems12 := framesShift[6] // 0x329
	if ems12.ArbitrationID != 0x329 {
		t.Fatalf("Expected index 6 to be 0x329, got 0x%03X", ems12.ArbitrationID)
	}
	if ems12.DecodedSignals["SPARK_FLOOR_CLAMPED"] != true {
		t.Errorf("Expected SPARK_FLOOR_CLAMPED to be true during clamped shift")
	}
}
