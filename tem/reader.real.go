package tem

import (
	"fmt"
	"math"

	"github.com/maseology/goHydro/grid"
)

// NewFromReal loads a TEM using square-cell D8 neighbour distances.
func NewFromReal(r grid.Real) (*TEM, error) {
	return NewFromRealWithDistances(r, nil)
}

// NewFromRealWithDistances loads a TEM with caller-supplied horizontal distances
// between active D8 neighbours. Distances must be finite, positive, and in the
// same length unit as elevation. A nil callback preserves NewFromReal routing.
//
// The callback is validated for all directed active-neighbour pairs before
// routing. It must remain pure and return the same distances for the lifetime of
// the TEM: it is retained, without a per-edge cache, for routing after filling.
// It affects D8 routing only; TEC.G and TEC.A retain their existing square-grid
// calculation. The callback is not serialized by SaveGob.
func NewFromRealWithDistances(r grid.Real, distance func(from, to int) float64) (*TEM, error) {
	bufs := r.GD.Buffers(false, true)
	if distance != nil {
		for from := range r.A {
			for _, to := range bufs[from] {
				if to < 0 {
					continue
				}
				if _, active := r.A[to]; !active {
					continue
				}
				d := distance(from, to)
				if d <= 0 || math.IsNaN(d) || math.IsInf(d, 0) {
					return nil, fmt.Errorf("TEM distance %d -> %d must be finite and positive, got %g", from, to, d)
				}
			}
		}
	}
	t := TEM{neighbourDistance: distance}

	t.TEC = buildTECs(r, bufs)
	// t.TEC = make(map[int]TEC, r.GD.Nact)
	// for c, z := range r.A {
	// 	bufz := make(map[int]float64, 9)
	// 	bufz[c] = z
	// 	for _, cc := range bufs[c] {
	// 		if cc >= 0 {
	// 			if !math.IsInf(r.A[cc], 0) {
	// 				bufz[cc] = r.A[cc]
	// 			}
	// 		}
	// 	}

	// 	g, a := gridSlopeAspectTarboton(bufz, c, r.GD.Ncol, r.GD.Cwidth)
	// 	t.TEC[c] = TEC{Z: z, G: g, A: a}
	// }

	ds := t.buildDsFromNeighbours(bufs)
	// t.checkVals()
	t.buildUpslopes(ds)

	return &t, nil
}

func BuildTECs(r grid.Real) map[int]TEC {
	bufs := r.GD.Buffers(false, true)
	return buildTECs(r, bufs)
}
