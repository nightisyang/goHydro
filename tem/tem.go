package tem

import (
	"encoding/gob"
	"os"
)

// TEM topologic elevation model
type TEM struct {
	TEC  map[int]TEC
	USlp map[int][]int

	neighbourDistance func(from, to int) float64
}

// NumCells number of cells that make up the TEM
func (t *TEM) NumCells() int {
	return len(t.TEC)
}

// SaveGob saves the TEM graph and cells to gob. A caller-supplied neighbour
// distance callback is not serialized; reconstruct a distance-aware TEM before
// rerouting a saved model that needs physical neighbour distances.
func (t *TEM) SaveGob(fp string) error {
	f, err := os.Create(fp)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	err = enc.Encode(t)
	if err != nil {
		return err
	}
	return nil
}

// LoadGob TEM gob
func LoadGob(fp string) (TEM, error) {
	var t TEM
	f, err := os.Open(fp)
	if err != nil {
		return t, err
	}
	defer f.Close()
	enc := gob.NewDecoder(f)
	err = enc.Decode(&t)
	if err != nil {
		return t, err
	}
	return t, nil
}
