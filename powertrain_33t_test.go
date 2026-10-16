package main

import (
	"math"
	"testing"
)

func TestPowertrain33TSpec(t *testing.T) {
	spec := GetStingerG70PlatformSpec()

	if spec.Engine.DisplacementCc != 3342.0 {
		t.Errorf("Expected 3342.0 cc, got %f", spec.Engine.DisplacementCc)
	}
	if spec.Engine.BoreMm != 92.0 || spec.Engine.StrokeMm != 83.8 {
		t.Errorf("Bore/Stroke mismatch: %f x %f", spec.Engine.BoreMm, spec.Engine.StrokeMm)
	}
	if spec.Engine.AirChargeLSB != 0.04238956 {
		t.Errorf("Expected 0.04238956 mg/stk/LSB, got %f", spec.Engine.AirChargeLSB)
	}
	if spec.Engine.PutSpMaxHpa != 2717.0 {
		t.Errorf("Expected PUT_SP 2717.0 hPa, got %f", spec.Engine.PutSpMaxHpa)
	}

	// Verify A8LR1 gear ratios and reflected inertias
	expectedRatios := [8]float64{3.964, 2.468, 1.610, 1.176, 1.000, 0.832, 0.652, 0.565}
	for i, r := range expectedRatios {
		if math.Abs(spec.Transmission.GearRatios[i]-r) > 1e-4 {
			t.Errorf("Gear %d ratio mismatch: expected %f, got %f", i+1, r, spec.Transmission.GearRatios[i])
		}
		if spec.Transmission.ReflectedInertias[i] <= 0 {
			t.Errorf("Gear %d reflected inertia invalid: %f", i+1, spec.Transmission.ReflectedInertias[i])
		}
	}
	t.Logf("A8LR1 1st gear reflected inertia at wheel axle: %.2f kg*m^2", spec.Transmission.ReflectedInertias[0])
	t.Logf("A8LR1 8th gear reflected inertia at wheel axle: %.2f kg*m^2", spec.Transmission.ReflectedInertias[7])

	// Verify Magna ATC specifications
	if spec.AWD.NominalTireRadiusMm != 320.0 {
		t.Errorf("Expected AWD nominal tire radius 320.0 mm, got %f", spec.AWD.NominalTireRadiusMm)
	}
	if spec.AWD.BrakeGatePressureBar != 4.45 {
		t.Errorf("Expected BrakeGatePressureBar 4.45 bar, got %f", spec.AWD.BrakeGatePressureBar)
	}
	if spec.AWD.StockSlewRateNmPerSec != 6000.0 || spec.AWD.TunedSlewRateNmPerSec != 9000.0 {
		t.Errorf("Slew rate mismatch: stock=%f, tuned=%f", spec.AWD.StockSlewRateNmPerSec, spec.AWD.TunedSlewRateNmPerSec)
	}

	// Verify Torque Converter evaluation
	nuStall, trStall, kStall := EvalTorqueConverter(2500.0, 0.0)
	if nuStall != 0.0 {
		t.Errorf("Expected nu=0 at stall, got %f", nuStall)
	}
	if math.Abs(trStall-1.88) > 0.05 {
		t.Errorf("Expected stall TR ~ 1.88, got %f", trStall)
	}
	if math.Abs(kStall-110.7) > 1.0 {
		t.Errorf("Expected stall K ~ 110.7, got %f", kStall)
	}
	t.Logf("TC at stall: nu=%.2f, TR=%.3f, K=%.1f", nuStall, trStall, kStall)

	nuCoup, trCoup, kCoup := EvalTorqueConverter(3000.0, 2550.0) // 2550/3000 = 0.85
	if math.Abs(nuCoup-0.85) > 1e-3 {
		t.Errorf("Expected nu=0.85 at coupling, got %f", nuCoup)
	}
	if math.Abs(trCoup-1.00) > 0.02 {
		t.Errorf("Expected coupling TR = 1.00, got %f", trCoup)
	}
	t.Logf("TC at coupling (nu=0.85): nu=%.2f, TR=%.3f, K=%.1f", nuCoup, trCoup, kCoup)

	// Verify Friction Loss Model
	fric85C := EvalG6DPFrictionLossNm(3000.0, 85.0)
	// Base at 3000: 34 + 0.0045*3000 + 1e-6*9e6 = 34 + 13.5 + 9.0 = 56.5 Nm
	if math.Abs(fric85C-56.5) > 0.5 {
		t.Errorf("Expected 56.5 Nm at 3000 RPM / 85C, got %f", fric85C)
	}
	fric20C := EvalG6DPFrictionLossNm(3000.0, 20.0)
	if fric20C <= fric85C {
		t.Errorf("Expected cold friction (20C) > warm friction (85C): 20C=%f, 85C=%f", fric20C, fric85C)
	}
	t.Logf("G6DP Friction at 3000 RPM: 85C = %.2f Nm, 20C = %.2f Nm (Ratio = %.2f)", fric85C, fric20C, fric20C/fric85C)
}
