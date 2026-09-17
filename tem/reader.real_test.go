package tem

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/maseology/goHydro/grid"
)

func TestNewFromRealWithDistancesRectangularRouting(t *testing.T) {
	// The centre (4) is 10 m, east (5) is 7 m, south (7) is 8 m.
	// Square spacing chooses the 3 m drop east. With dx=3 m, dy=1 m,
	// east has slope 3/3=1 and south has slope 2/1=2, so south wins.
	for _, diagonal := range []bool{false, true} {
		t.Run(fmt.Sprintf("diagonal=%t", diagonal), func(t *testing.T) {
			r := distanceTestReal(t)
			wantLegacy := 5
			if diagonal {
				// A 4 m drop southwest instead: 4/sqrt(2)>2 for a square,
				// but 4/sqrt(10)<2 with physical rectangular spacing.
				r.A[5], r.A[6] = 100, 6
				wantLegacy = 6
			}
			legacy, err := NewFromReal(r)
			if err != nil {
				t.Fatal(err)
			}
			physical, err := NewFromRealWithDistances(r, rectangularTestDistance(3, 3, 1))
			if err != nil {
				t.Fatal(err)
			}
			if got := legacy.Downslopes()[4]; got != wantLegacy {
				t.Fatalf("square spacing: centre routes to %d, want cell %d", got, wantLegacy)
			}
			if got := physical.Downslopes()[4]; got != 7 {
				t.Fatalf("physical spacing: centre routes to %d, want south cell 7", got)
			}
			if !reflect.DeepEqual(legacy.TEC, physical.TEC) {
				t.Fatal("distance callback changed elevations or ancillary slope/aspect")
			}
		})
	}
}

func TestNewFromRealWithNilDistancesMatchesLegacy(t *testing.T) {
	for _, fixFlats := range []bool{false, true} {
		t.Run(flatModeName(fixFlats), func(t *testing.T) {
			r := distanceTestReal(t)
			legacy, err := NewFromReal(r)
			if err != nil {
				t.Fatal(err)
			}
			withNil, err := NewFromRealWithDistances(r, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(legacy.TEC, withNil.TEC) || !reflect.DeepEqual(legacy.Downslopes(), withNil.Downslopes()) {
				t.Fatal("nil distance callback changed legacy constructor results")
			}
			legacy.FillDepressions(r.GD, fixFlats, "")
			withNil.FillDepressions(r.GD, fixFlats, "")
			if !reflect.DeepEqual(legacy.TEC, withNil.TEC) || !reflect.DeepEqual(legacy.Downslopes(), withNil.Downslopes()) {
				t.Fatal("nil distance callback changed legacy post-fill results")
			}
		})
	}
}

func TestNewFromRealWithDistancesRejectsInvalidNeighbours(t *testing.T) {
	for _, invalid := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		// Validate each direction, including an uphill neighbour that would
		// not be selected initially but could become eligible after filling.
		for _, pair := range [][2]int{{4, 0}, {0, 4}} {
			t.Run(fmt.Sprintf("%g/%d-to-%d", invalid, pair[0], pair[1]), func(t *testing.T) {
				distance := rectangularTestDistance(3, 3, 1)
				model, err := NewFromRealWithDistances(distanceTestReal(t), func(from, to int) float64 {
					if from == pair[0] && to == pair[1] {
						return invalid
					}
					return distance(from, to)
				})
				if err == nil || model != nil {
					t.Fatalf("invalid neighbour distance %g: got model %v, error %v", invalid, model, err)
				}
			})
		}
	}
}

func TestNewFromRealWithDistancesSkipsInactiveNeighbours(t *testing.T) {
	r := distanceTestReal(t)
	delete(r.A, 0)
	r.GD.RemoveActives([]int{0})
	distance := rectangularTestDistance(3, 3, 1)
	model, err := NewFromRealWithDistances(r, func(from, to int) float64 {
		if _, ok := r.A[from]; !ok {
			t.Fatalf("callback received inactive origin %d", from)
		}
		if _, ok := r.A[to]; !ok {
			t.Fatalf("callback received inactive destination %d", to)
		}
		return distance(from, to)
	})
	if err != nil {
		t.Fatal(err)
	}
	model.FillDepressions(r.GD, false, "")
	if got := model.Downslopes()[4]; got != 7 {
		t.Fatalf("physical spacing with inactive corner: centre routes to %d, want 7", got)
	}
}

func TestNewFromRealWithDistancesSurvivesFillAndRepeats(t *testing.T) {
	for _, fixFlats := range []bool{false, true} {
		t.Run(flatModeName(fixFlats), func(t *testing.T) {
			var firstRoutes, firstCounts map[int]int
			var firstCells map[int]TEC
			for repeat := 0; repeat < 8; repeat++ {
				r := distanceTestReal(t)
				model, err := NewFromRealWithDistances(r, rectangularTestDistance(3, 3, 1))
				if err != nil {
					t.Fatal(err)
				}
				model.FillDepressions(r.GD, fixFlats, "")
				routes, counts := model.Downslopes(), model.ContributingCellCounts()
				if routes[4] != 7 {
					t.Fatalf("repeat %d: post-fill centre routes to %d, want south cell 7", repeat, routes[4])
				}
				if repeat == 0 {
					firstRoutes, firstCounts, firstCells = routes, counts, model.TEC
				} else if !reflect.DeepEqual(routes, firstRoutes) || !reflect.DeepEqual(counts, firstCounts) || !reflect.DeepEqual(model.TEC, firstCells) {
					t.Fatalf("repeat %d changed physical-distance routing, counts, or elevations", repeat)
				}
			}
		})
	}
}

func TestDistanceRoutingSurvivesTerrainCopies(t *testing.T) {
	r := distanceTestReal(t)
	model, err := NewFromRealWithDistances(r, rectangularTestDistance(3, 3, 1))
	if err != nil {
		t.Fatal(err)
	}
	clipped := model.ClipToActives(r.GD)
	subset, _ := model.SubSet(5)
	for name, copied := range map[string]*TEM{"clip": &clipped, "subset": subset} {
		t.Run(name, func(t *testing.T) {
			gd := grid.NewDefinition(t.Name(), 3, 3, 1)
			var missing []int
			for cell := 0; cell < 9; cell++ {
				if _, ok := copied.TEC[cell]; !ok {
					missing = append(missing, cell)
				}
			}
			gd.RemoveActives(missing)
			copied.FillDepressions(gd, false, "")
			if got := copied.Downslopes()[4]; got != 7 {
				t.Fatalf("copied terrain lost physical distances: centre routes to %d, want 7", got)
			}
		})
	}
}

func TestLegacyNewClearsPreviousDistancesOnSuccessfulLoad(t *testing.T) {
	r := distanceTestReal(t)
	legacy, err := NewFromReal(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.uhdem")
	if err := legacy.SaveUHDEM(path, r.GD); err != nil {
		t.Fatal(err)
	}
	for _, fixFlats := range []bool{false, true} {
		t.Run(flatModeName(fixFlats), func(t *testing.T) {
			reused, err := NewFromRealWithDistances(r, rectangularTestDistance(3, 3, 1))
			if err != nil {
				t.Fatal(err)
			}
			if got := reused.Downslopes()[4]; got != 7 {
				t.Fatalf("before reload: centre routes to %d, want physical-distance south cell 7", got)
			}
			if err := reused.New(path); err != nil {
				t.Fatal(err)
			}
			fresh, err := NewTEM(path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(reused.TEC, fresh.TEC) || !reflect.DeepEqual(reused.Downslopes(), fresh.Downslopes()) {
				t.Fatal("fresh and reused legacy loaders disagree immediately after loading")
			}
			reused.FillDepressions(r.GD, fixFlats, "")
			fresh.FillDepressions(r.GD, fixFlats, "")
			if got := reused.Downslopes()[4]; got != 5 {
				t.Fatalf("after legacy reload and fill: centre routes to %d, want square-distance east cell 5", got)
			}
			if !reflect.DeepEqual(reused.TEC, fresh.TEC) || !reflect.DeepEqual(reused.Downslopes(), fresh.Downslopes()) {
				t.Fatal("legacy reload retained earlier physical distances during post-fill routing")
			}
		})
	}
}

func TestLegacyNewPreservesPreviousDistancesOnLoadError(t *testing.T) {
	for _, filename := range []string{"invalid.uhdem", "unsupported.xyz"} {
		t.Run(filename, func(t *testing.T) {
			r := distanceTestReal(t)
			model, err := NewFromRealWithDistances(r, rectangularTestDistance(3, 3, 1))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), filename)
			// A readable file with an unsupported binary header exercises
			// a returned load error; missing files make the legacy I/O exit.
			if err := os.WriteFile(path, []byte{1, 'x'}, 0600); err != nil {
				t.Fatal(err)
			}
			if err := model.New(path); err == nil {
				t.Fatal("expected legacy load error")
			}
			model.FillDepressions(r.GD, false, "")
			if got := model.Downslopes()[4]; got != 7 {
				t.Fatalf("failed reload changed physical routing: centre routes to %d, want south cell 7", got)
			}
		})
	}
}

func distanceTestReal(t *testing.T) grid.Real {
	t.Helper()
	return grid.Real{
		GD: grid.NewDefinition(t.Name(), 3, 3, 1),
		A: map[int]float64{
			0: 100, 1: 100, 2: 100,
			3: 100, 4: 10, 5: 7,
			6: 100, 7: 8, 8: 100,
		},
	}
}

func rectangularTestDistance(cols int, dx, dy float64) func(int, int) float64 {
	return func(from, to int) float64 {
		rowDifference := float64(from/cols - to/cols)
		columnDifference := float64(from%cols - to%cols)
		return math.Hypot(columnDifference*dx, rowDifference*dy)
	}
}
