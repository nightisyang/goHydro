package pet_test

import (
	"math"
	"testing"

	"github.com/maseology/goHydro/pet"
)

// Decimal-derived daily depths, without calling the production heat/density
// helpers. See testdata/oudin-reference.md for equations and provenance.
// The 35 MJ/m2/day, 27 C case must give about 4.617 mm/day, not 0.004617.
func TestOudinDailyDepthUnits(t *testing.T) {
	for _, tt := range []struct {
		name     string
		ra, temp float64
		want     float64
	}{
		{"warm", 35, 27, 0.004616710807647448},
		{"above_cutoff", 15, -4.999, 0.00000005971170057852827},
		{"freezing", 15, 0, 0.0003000600100816323},
		{"cool", 25, 10, 0.001515479185876706},
		{"hot", 45, 40, 0.008488389695053093},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := pet.Oudin(tt.ra, tt.temp)
			if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("Oudin(%g, %g) = %.17g m/day, want %.17g m/day", tt.ra, tt.temp, got, tt.want)
			}
		})
	}
}

func TestOudinZeroDemand(t *testing.T) {
	for _, tt := range []struct {
		ra, temp float64
	}{
		{35, -10}, {35, -5.000001}, {35, -5},
		{0, -5}, {0, 0}, {0, 27}, {0, 40},
	} {
		if got := pet.Oudin(tt.ra, tt.temp); got != 0 {
			t.Errorf("Oudin(%g, %g) = %g m/day, want zero", tt.ra, tt.temp, got)
		}
	}
}

func TestOudinAirGRReference(t *testing.T) {
	// Outputs of unmodified airGR frun_PE.f90 at
	// 4bb401c6529e20116db357369c9f2bc34f959992, in metres/day.
	// Radiation was calculated independently from Morton's solar geometry.
	// airGR uses a constant effective lambda*rho of 2462.4 MJ/m3;
	// goHydro uses temperature-dependent values (literal energyDensity below).
	// Across 0-40 C that convention explains up to 3.22% difference; a 3.3%
	// relative bound checks the raw reference, then an aligned comparison
	// allows 1e-9 m/day for airGR's single-precision radiation constants.
	// Full provenance and regeneration instructions: testdata/oudin-reference.md.
	for _, tt := range []struct {
		name                           string
		ra, temp, airGR, energyDensity float64
	}{
		{"south_day1", 45.614638986596894, 40, 0.008336008566103312, 2385.6114914},
		{"south_day172", 10.01729434883312, 0, 0.00020340509946973248, 2499.500016},
		{"equator_day80", 38.77929481734797, 10, 0.0023622864756080155, 2474.4648656},
		{"equator_day355", 36.51239907282164, 27, 0.004744951150723647, 2425.96958454654},
		{"tropics_day1", 35.68080245702137, 27, 0.004636881417102905, 2425.96958454654},
		{"tropics_day172", 35.06147875327234, 40, 0.006407433979256792, 2385.6114914},
		{"north_day1", 10.967959773308872, 0, 0.00022270873697612128, 2499.500016},
		{"north_day172", 42.96340230010742, 10, 0.0026171663185356403, 2474.4648656},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := pet.Oudin(tt.ra, tt.temp)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("Oudin(%g, %g) = %g; want finite depth", tt.ra, tt.temp, got)
			}
			if math.Abs(got-tt.airGR) > 0.033*tt.airGR {
				t.Errorf("depth %.17g m/day outside 3.3%% of raw airGR reference %.17g m/day", got, tt.airGR)
			}
			want := tt.airGR * 2462.4 / tt.energyDensity
			if math.Abs(got-want) > 1e-9 {
				t.Errorf("depth %.17g m/day, thermodynamically aligned airGR reference %.17g m/day", got, want)
			}
		})
	}
}
