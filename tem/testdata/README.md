# Synthetic fill fixtures

`fill-fixtures.json` contains 123 terrain grids used in the 2026-09-17 numerical evaluation: an enclosed basin, a basin with an inactive cell, a flat grid, and 120 reproducible generated grids. Values are metres; `-9999` identifies inactive cells. These are synthetic inputs, not site DEMs or observed water levels.

The generated grids use NumPy `default_rng(20260917)` (PCG64). For each index `i` from 0 through 119:

```python
z = rng.integers(0, 40, size=(9, 11)).astype(float)
if i % 4 == 0:
    z[3:6, 4:7] = rng.integers(0, 6)
if i % 10 == 0:
    z[4, 5] = -9999
```

The hand-built fixtures are fully represented by their small JSON arrays. Tests also construct a zero flat and a two-outlet basin. No Python dependency is needed to run the tests.

The Go tests independently calculate minimum spill elevations by minimax-path relaxation, check graph topology and upstream counts, and repeat each input eight times in both flat modes. Grid edges and neighbours of inactive cells are treated as open outlets; other interpretations of missing data require a separate boundary policy.
