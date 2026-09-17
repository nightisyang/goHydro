package tem

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/maseology/goHydro/grid"
)

type fillTestFixture struct {
	Name       string    `json:"name"`
	Rows       int       `json:"rows"`
	Cols       int       `json:"cols"`
	Elevations []float64 `json:"elevations"`
}

// Eight repeats catch map-order changes in flat membership and open-edge exit
// selection. The fixtures include depressions, flats and missing-data holes.
func TestFillDepressionsReferenceAndRepeatability(t *testing.T) {
	data, err := os.ReadFile("testdata/fill-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []fillTestFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	fixtures = append(fixtures, fillTestFixture{Name: "flat-zero", Rows: 9, Cols: 9, Elevations: make([]float64, 81)})
	twoOutlets := make([]float64, 11*13)
	for cell := range twoOutlets {
		r, c := cell/13, cell%13
		twoOutlets[cell] = 10
		if r > 0 && r < 10 && c > 0 && c < 12 {
			twoOutlets[cell] = 5
		}
	}
	twoOutlets[5*13], twoOutlets[5*13+12] = 4, 4
	fixtures = append(fixtures, fillTestFixture{Name: "two-outlets", Rows: 11, Cols: 13, Elevations: twoOutlets})
	for _, fixture := range fixtures {
		if fixture.Rows*fixture.Cols != len(fixture.Elevations) {
			t.Fatalf("invalid dimensions for %s", fixture.Name)
		}
		want := referenceTestFill(fixture.Rows, fixture.Cols, fixture.Elevations)
		for _, fix := range []bool{false, true} {
			t.Run(fixture.Name+"/"+flatModeName(fix), func(t *testing.T) {
				var firstHeights map[int]float64
				var firstDirections, firstCounts map[int]int
				for repeat := 0; repeat < 8; repeat++ {
					model := filledTestTerrain(t, fixture.Rows, fixture.Cols, fixture.Elevations, fix)
					heights := make(map[int]float64, len(model.TEC))
					for cell, value := range model.TEC {
						heights[cell] = value.Z
						if math.IsNaN(value.Z) || math.IsInf(value.Z, 0) || math.Abs(value.Z-want[cell]) > 1e-8 {
							t.Fatalf("repeat %d cell %d: filled %.12g, independent minimum spill level %.12g", repeat, cell, value.Z, want[cell])
						}
					}
					checkTestDrainage(t, model, fixture.Rows, fixture.Cols, fixture.Elevations)
					directions, counts := model.Downslopes(), model.ContributingCellCounts()
					if repeat == 0 {
						firstHeights, firstDirections, firstCounts = heights, directions, counts
					} else if !reflect.DeepEqual(firstHeights, heights) || !reflect.DeepEqual(firstDirections, directions) || !reflect.DeepEqual(firstCounts, counts) {
						t.Fatalf("identical DEM changed elevations, routes, or upstream counts on repeat %d", repeat)
					}
				}
			})
		}
	}
}

// An independent minimax-path relaxation, without a priority queue or artificial
// gradients: find each cell's lowest possible spill level to any open edge.
func referenceTestFill(rows, cols int, z []float64) []float64 {
	filled := make([]float64, len(z))
	for cell, height := range z {
		filled[cell] = math.Inf(1)
		if height == -9999 || testOpenBoundary(cell, rows, cols, z) {
			filled[cell] = height
		}
	}
	for pass := 0; pass < len(z); pass++ {
		changed := false
		for cell, height := range z {
			if height == -9999 {
				continue
			}
			for _, n := range testNeighbours(cell, rows, cols) {
				if z[n] == -9999 {
					continue
				}
				candidate := math.Max(height, filled[n])
				if candidate < filled[cell] {
					filled[cell] = candidate
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	return filled
}

// Every edge of this basin initially slopes inward, including its 5 m spill
// point. Seeding only existing flow outlets leaves the 1 m floor unfilled.
func TestFillDepressionsUsesEveryOpenBoundaryCell(t *testing.T) {
	z := []float64{
		10, 10, 10, 10, 10,
		10, 1, 1, 1, 10,
		10, 1, 1, 1, 5,
		10, 1, 1, 1, 10,
		10, 10, 10, 10, 10,
	}
	for _, fixFlats := range []bool{false, true} {
		t.Run(flatModeName(fixFlats), func(t *testing.T) {
			model := filledTestTerrain(t, 5, 5, z, fixFlats)
			for row := 1; row < 4; row++ {
				for col := 1; col < 4; col++ {
					got := model.TEC[row*5+col].Z
					if math.IsNaN(got) || math.Abs(got-5) > 1e-8 {
						t.Errorf("floor (%d,%d): got %.12g m, want 5 m", row, col, got)
					}
				}
			}
			checkTestDrainage(t, model, 5, 5, z)
		})
	}
}

func flatModeName(fix bool) string {
	if fix {
		return "flat-correction"
	}
	return "simple-gradient"
}

func filledTestTerrain(t *testing.T, rows, cols int, z []float64, fix bool) *TEM {
	t.Helper()
	gd := grid.NewDefinition(t.Name(), rows, cols, 30)
	active := make(map[int]float64, len(z))
	var missing []int
	for cell, height := range z {
		if height == -9999 {
			missing = append(missing, cell)
		} else {
			active[cell] = height
		}
	}
	gd.RemoveActives(missing)
	model, err := NewFromReal(grid.Real{GD: gd, A: active})
	if err != nil {
		t.Fatal(err)
	}
	model.FillDepressions(gd, fix, "")
	return model
}

var testNeighbourOffsets = [][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}}

func testNeighbours(cell, rows, cols int) []int {
	row, col := cell/cols, cell%cols
	var cells []int
	for _, d := range testNeighbourOffsets {
		r, c := row+d[0], col+d[1]
		if r >= 0 && r < rows && c >= 0 && c < cols {
			cells = append(cells, r*cols+c)
		}
	}
	return cells
}

func testOpenBoundary(cell, rows, cols int, z []float64) bool {
	r, c := cell/cols, cell%cols
	if r == 0 || c == 0 || r == rows-1 || c == cols-1 {
		return true
	}
	for _, n := range testNeighbours(cell, rows, cols) {
		if z[n] == -9999 {
			return true
		}
	}
	return false
}

// Independently traverse the directed graph to detect cycles, interior sinks,
// invalid links, and incorrect upstream counts. No library graph helper is used
// to calculate the expected counts.
func checkTestDrainage(t *testing.T, model *TEM, rows, cols int, z []float64) {
	t.Helper()
	ds := model.Downslopes()
	degree, want := make(map[int]int), make(map[int]int)
	for cell := range model.TEC {
		want[cell] = 1
		if to, ok := ds[cell]; ok {
			if _, exists := model.TEC[to]; !exists {
				t.Fatalf("invalid edge %d -> %d", cell, to)
			}
			adjacent := false
			for _, n := range testNeighbours(cell, rows, cols) {
				adjacent = adjacent || n == to
			}
			if !adjacent || model.TEC[to].Z > model.TEC[cell].Z+1e-8 {
				t.Fatalf("non-neighbour or uphill edge %d -> %d", cell, to)
			}
			degree[to]++
		} else if !testOpenBoundary(cell, rows, cols, z) {
			t.Errorf("interior sink at %d", cell)
		}
	}
	var ready []int
	for cell := range model.TEC {
		if degree[cell] == 0 {
			ready = append(ready, cell)
		}
	}
	for next := 0; next < len(ready); next++ {
		cell := ready[next]
		if to, ok := ds[cell]; ok {
			want[to] += want[cell]
			degree[to]--
			if degree[to] == 0 {
				ready = append(ready, to)
			}
		}
	}
	if len(ready) != len(model.TEC) {
		t.Fatalf("cycle: visited %d of %d cells", len(ready), len(model.TEC))
	}
	got := model.ContributingCellCounts()
	for cell, count := range want {
		if got[cell] != count {
			t.Errorf("cell %d: contributing count %d, want %d", cell, got[cell], count)
		}
	}
}
