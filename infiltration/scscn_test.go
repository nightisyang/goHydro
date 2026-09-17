package infiltration_test

import (
	"math"
	"testing"

	"github.com/maseology/goHydro/infiltration"
)

// CN2=80 gives S2=0.0635 m. Independently evaluated with decimal
// arithmetic: S1=2.281*S2, S3=0.427*S2, and F=p*S/(p+S).
// These are post-abstraction retention depths, not total storm losses.
func TestScscnAntecedentMoistureStormRetention(t *testing.T) {
	for _, tt := range []struct {
		name string
		amc  int
		want float64
	}{
		{"dry", 1, 0.037169189631678758},
		{"normal", 2, 0.027973568281938326},
		{"wet", 3, 0.017580675489045510},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := infiltration.Scscn(0.05, 80, tt.amc)
			if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("50 mm available rain, CN80, AMC%d: retention %.17g m, want %.17g m", tt.amc, got, tt.want)
			}
		})
	}
}

// At CN immediately below 100, rounding can give zero retention capacity;
// zero rainfall must still return zero rather than evaluating 0/0.
func TestScscnZeroRain(t *testing.T) {
	for _, amc := range []int{1, 2, 3} {
		for _, cn := range []float64{0, 50, 80, 99.999999, math.Nextafter(100, 0), 100} {
			if got := infiltration.Scscn(0, cn, amc); got != 0 {
				t.Errorf("zero rain, CN=%.17g, AMC%d: retention %g m, want 0", cn, amc, got)
			}
		}
	}
}

func TestScscnCurveNumberEndpoints(t *testing.T) {
	for _, amc := range []int{1, 2, 3} {
		if got := infiltration.Scscn(0.05, 0, amc); got != 0.05 {
			t.Errorf("CN0, AMC%d: retention %g m, want 0.05", amc, got)
		}
		if got := infiltration.Scscn(0.05, 100, amc); got != 0 {
			t.Errorf("CN100, AMC%d: retention %g m, want 0", amc, got)
		}
	}
}

func TestScscnSampledRetention(t *testing.T) {
	const tolerance = 1e-12 // metres; permits cancellation roundoff near CN=100
	// Scale retention directly instead of converting CN as production does.
	// The CN conversions are published in Hong et al. (2007), equations 4-5:
	// https://gpm.nasa.gov/sites/default/files/document_files/2007_WRR_TRMM_Global_Runoff.pdf
	// Broad CN/rainfall extremes below are numerical stress cases, not a
	// claim that the empirical conversions are calibrated over that range.
	curveNumbers := []float64{math.Nextafter(100, 0), 0.281 / 0.01281}
	for i := 0; i <= 1000; i++ {
		curveNumbers = append(curveNumbers, float64(i)/10)
	}
	for _, cn := range curveNumbers {
		for _, p := range []float64{0, 1e-12, 1e-8, 0.001, 0.01, 0.025, 0.05, 0.1, 1} {
			for i, multiplier := range []float64{2.281, 1, 0.427} {
				amc := i + 1
				want := 0.0
				if cn == 0 {
					want = p
				} else if p > 0 && cn < 100 {
					// Equivalent to S2=25.4/CN2-0.254 m without the
					// subtractive cancellation near CN=100.
					s := 0.254 * (100 - cn) / cn * multiplier
					want = p * s / (p + s)
				}
				got := infiltration.Scscn(p, cn, amc)
				if math.IsNaN(got) || math.IsInf(got, 0) || got < -tolerance || got > p+tolerance {
					t.Fatalf("p=%g m, CN=%.17g, AMC%d: retention %g m outside finite [0, %g] within %g m", p, cn, amc, got, p, tolerance)
				}
				if math.Abs(got-want) > tolerance {
					t.Fatalf("p=%g m, CN=%.17g, AMC%d: retention %.17g m, independent oracle %.17g m", p, cn, amc, got, want)
				}
			}
		}
	}
}
