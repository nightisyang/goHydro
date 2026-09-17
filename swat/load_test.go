package swat

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadSkipsOneHeaderAndRetainsBothDataRows(t *testing.T) {
	previousTopology := topo
	t.Cleanup(func() { topo = previousTopology })
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	basins := write("basins.csv", "SWSID,SUBKM,SLSUBBSN,CHL,CHS,CHN,SURLAG,GWDELAY,ALPHABF\n11,1.25,100,1,0.01,0.03,4,30,0.05\n22,2.5,120,2,0.02,0.04,5,40,0.06\n")
	channels := write("channels.csv", "SWSID,CHL,CHS,CHW,CHD,CHN\n11,1,0.01,10,1,0.03\n22,2,0.02,12,1.5,0.04\n")
	hrus := write("hrus.csv", "SWSID,HRUFR,HRUSLP,OVN,CN2,CV,ESCO,CLAY,SOLBD,SOLAWC,SOLK\n11,1,0.05,0.15,75,1000,0.9,20,1.3,0.15,10\n22,1,0.06,0.18,80,2000,0.8,30,1.2,0.2,5\n")
	topology := write("topology.txt", "11,-1\n22,11\n")
	basin, order := Load(basins, hrus, channels, topology)
	if len(basin) != 2 || !reflect.DeepEqual(order, []int{11, 22}) {
		t.Fatalf("loaded %d basins in order %v, want two basins in order [11 22]", len(basin), order)
	}
	for _, want := range []struct {
		id, outflow         int
		area, channel, soil float64
	}{{11, 22, 1.25, 6.4, 127}, {22, -1, 2.5, 10.8, 172}} {
		got, ok := basin[want.id]
		if !ok {
			t.Fatalf("missing basin %d", want.id)
		}
		if got.Ca != want.area || got.Outflow != want.outflow {
			t.Errorf("basin %d: area %g/outflow %d, want %g/%d", want.id, got.Ca, got.Outflow, want.area, want.outflow)
		}
		aquifer, pond, flow, channel, soil := got.StorageAll()
		if aquifer != 0 || pond != 0 || flow != 0 || math.IsNaN(channel) || math.IsNaN(soil) || math.Abs(channel-want.channel) > 1e-9 || math.Abs(soil-want.soil) > 1e-9 {
			t.Errorf("basin %d: initial storage %v, want %v mm", want.id, []float64{aquifer, pond, flow, channel, soil}, []float64{0, 0, 0, want.channel, want.soil})
		}
	}
}
