package rainrun

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

// Exercise the public constructor and update without changing their initial
// state. X4=0.5 is accepted by New and gives UH2 one bin and no delay buffer.
func TestGR4JHalfDayUpdate(t *testing.T) {
	var model GR4J
	model.New(0.350, 0, 0.090, 0.5, 0.000001)
	initial := model.Storage()
	es, q, perc := model.Update(0.050, 0.003)
	for name, value := range map[string]float64{"soil evaporation": es, "runoff": q, "percolation": perc, "storage": model.Storage()} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			t.Fatalf("%s must be finite and nonnegative, got %g", name, value)
		}
	}
	if q <= 0 {
		t.Fatalf("a wet day with a nonempty routing store must produce runoff, got %g", q)
	}
	// Both hydrographs release all input today at X4=0.5, and X2=0
	// excludes exchange. Total evaporation includes 3 mm interception.
	gr4jNear(t, "water balance", model.Storage()+es+0.003+q, initial+0.050, 1e-14)
}

func TestGR4JUnitHydrographImpulse(t *testing.T) {
	// Independently evaluated S-curve increments. At X4=2, UH1's
	// first ordinate is (1/2)^(5/2), and UH2 uses half that ordinate.
	cases := []struct {
		x4       float64
		uh1, uh2 []float64
	}{
		{0.5, []float64{1}, []float64{1}},
		{1, []float64{1}, []float64{0.5, 0.5}},
		{2, []float64{0.1767766952966369, 0.8232233047033631}, []float64{0.08838834764831845, 0.41161165235168155, 0.41161165235168155, 0.08838834764831845}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("X4=%g", tc.x4), func(t *testing.T) {
			var model GR4J
			model.New(0.350, 0, 0.090, tc.x4, 0.000001)
			for _, branch := range []struct {
				name   string
				update func(float64) float64
				want   []float64
			}{{"UH1", model.updateUH1, tc.uh1}, {"UH2", model.updateUH2, tc.uh2}} {
				t.Run(branch.name, func(t *testing.T) {
					const impulse = 0.010
					var total float64
					// Extra zero-input days check that the delay buffer drains.
					for day := 0; day < len(branch.want)+3; day++ {
						var input, want float64
						if day == 0 {
							input = impulse
						}
						if day < len(branch.want) {
							want = impulse * branch.want[day]
						}
						got := branch.update(input)
						gr4jNear(t, fmt.Sprintf("day %d", day+1), got, want, 1e-15)
						total += got
					}
					gr4jNear(t, "conserved impulse", total, impulse, 1e-15)
				})
			}
		})
	}
}

func TestGR4JAirGRReference(t *testing.T) {
	data, err := os.ReadFile("testdata/gr4j-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Forcings map[string][][2]float64 `json:"forcings"`
		Cases    []struct {
			Name          string       `json:"name"`
			Parameters    [4]float64   `json:"parameters"`
			InitialStores [2]float64   `json:"initial_stores_mm"`
			Forcing       string       `json:"forcing"`
			Daily         [][5]float64 `json:"daily"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("airGR fixture contains no cases")
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			forcing, ok := fixture.Forcings[tc.Forcing]
			if !ok || len(forcing) == 0 || len(forcing) != len(tc.Daily) {
				t.Fatal("reference outputs and forcing must have equal nonzero lengths")
			}
			const mmToM = 0.001
			var model GR4J
			model.New(tc.Parameters[0]*mmToM, tc.Parameters[1]*mmToM, tc.Parameters[2]*mmToM, tc.Parameters[3], 0.000001)
			// Compare daily equations from explicitly equal stores, independently
			// of the constructor's initial-discharge optimization. New initializes
			// the UH buffers to zero, matching the reference. X4 stays in days;
			// every depth/flux parameter, input and state is converted to metres.
			model.prd.sto = tc.InitialStores[0] * mmToM
			model.rte.sto = tc.InitialStores[1] * mmToM
			var maxError [5]float64
			for day, input := range forcing {
				before := gr4jFullStorage(&model)
				p, ep := input[0]*mmToM, input[1]*mmToM
				es, q, perc := model.Update(p, ep)
				// airGR reports total actual ET; goHydro reports soil ET only.
				actualET := es + math.Min(p, ep)
				got := [5]float64{q / mmToM, actualET / mmToM, perc / mmToM, model.prd.sto / mmToM, model.rte.sto / mmToM}
				for column, name := range []string{"runoff", "actual ET", "percolation", "production store", "routing store"} {
					if got[column] < 0 {
						t.Fatalf("day %d %s must be nonnegative, got %g mm", day+1, name, got[column])
					}
					// 0.00001 mm (10 nm) is well below meaningful forcing precision.
					// It allows airGR's single-precision 0.9 split constant and the
					// resulting accumulated routing differences; see testdata/README.
					gr4jNear(t, fmt.Sprintf("day %d %s (mm)", day+1, name), got[column], tc.Daily[day][column], 1e-5)
					maxError[column] = math.Max(maxError[column], math.Abs(got[column]-tc.Daily[day][column]))
				}
				if tc.Parameters[1] == 0 {
					// Include the water still queued in both hydrographs. Storage()
					// alone omits it and cannot close a delayed-flow water balance.
					gr4jNear(t, fmt.Sprintf("day %d water balance (m)", day+1),
						gr4jFullStorage(&model)+actualET+q, before+p, 1e-13)
				}
			}
			t.Logf("maximum absolute errors [runoff, ET, percolation, production, routing] in mm: %g", maxError)
		})
	}
}

func gr4jFullStorage(model *GR4J) float64 {
	storage := model.Storage()
	for _, queued := range model.cv1 {
		storage += queued
	}
	for _, queued := range model.cv2 {
		storage += queued
	}
	return storage
}

func gr4jNear(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) || math.IsNaN(want) || math.IsInf(want, 0) || math.Abs(got-want) > tolerance {
		t.Fatalf("%s: got %.17g, want %.17g (absolute tolerance %g)", name, got, want, tolerance)
	}
}
