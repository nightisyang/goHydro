# Maintenance fork

This fork carries a small set of corrections to
[`maseology/goHydro`](https://github.com/maseology/goHydro), based on upstream
commit `fe22136746fde56ac98eda07fb3db4864590b9d0` (21 July 2026).
Its purpose is to preserve reviewed fixes and reproducible numerical tests.
It does not establish that every model in the repository is suitable for a
particular application or calibrated against observations.

## Changes

| Area | Correction | Regression evidence |
|---|---|---|
| Build | Pin dependencies; adapt `SortMapInt` and three CSV-reader calls; restore the missing URL for the existing `goHGS` gitlink | Entire root module compiles; real SWAT load fixture preserves one header and both data rows; Git can resolve the submodule mapping |
| Terrain filling | Seed every open boundary, including edges initially sloping into a basin | A hand-derived 5 m spill-level fixture fails before the correction |
| Flat terrain | Order initial queue entries, use all eligible fallback exits, and apply final repairs after evaluating them | Repeated elevations, directions and upstream counts; independent fill and graph checks |
| D8 distances | Add an opt-in neighbour-distance callback for rectangular or geographic grids | Hand-derived cardinal/diagonal route changes, invalid-distance rejection, and post-fill/copy repeatability |
| SCS curve number | Correct dry coefficient `0.281` to `2.281`; handle zero rainfall before division | Literal dry/normal/wet amounts, CN boundaries and an independent retention sweep |
| Oudin PET | Remove an extra factor of 1,000; document extraterrestrial radiation | Independent daily-depth values and pinned airGR comparisons |
| GR4J | Handle the single-bin second hydrograph at `X4=0.5` days | Real constructor/update, impulse conservation and 21 airGR reference scenarios |

The compatibility commit preserves SWAT's historical one-header-row CSV
contract; it does not validate SWAT's hydrological predictions. The earlier
mmio implementation always skipped one header
([historical source](https://github.com/maseology/mmio/blob/ce09b718be6df2eaf916c84d85e47595bcd36798/csv.go)).

## Run the checks

Use Go 1.26.2 or a compatible newer toolchain. CI reads the exact minimum
version from `go.mod`, and runs on Linux and macOS:

```sh
go mod download
go mod verify
go mod tidy -diff
go test -mod=readonly -race -count=1 ./...
go vet -mod=readonly ./...
```

These commands test the **root module**. The `grid`, `convolution`, and `gmet`
subdirectories remain independent modules; root builds consume the explicitly
pinned upstream versions of those modules. No source changes are carried in
those submodules by this series. Packages without tests receive compilation
and vet checks, not numerical certification.

The existing `goHGS` git submodule has its missing `.gitmodules` mapping
restored, with its original commit unchanged. CI does not initialize that
separate repository; it is not part of the root-module validation.
An unreachable return after the existing `snowpack.HMETS.Update` panic was
removed so whole-module static analysis can run. The stub still panics;
this cleanup does not implement or validate that snow model.

The regression suite requires no DEM download, Python, Fortran, external
service, or private evaluation workspace. It includes:

- 125 synthetic terrains, each repeated eight times in both flat modes:
  2,000 calculations, plus the separate hand-derived boundary regression.
- 27,081 sampled SCS retention cases, literal values and endpoint tests.
- Five literal Oudin depths, cutoff/zero cases, and eight airGR fixtures.
- 21 GR4J scenarios totaling 2,574 days, checking runoff, evaporation,
  percolation, both stores, and zero-exchange water balances.

The terrain oracle uses minimax-path relaxation rather than the production
priority queue. Its independent graph traversal checks cycles, invalid or
uphill links, interior sinks, and contributing counts. See
[`tem/testdata`](../tem/testdata/README.md) for synthetic input provenance.
Reference generation, source revisions and justified floating-point tolerances
are documented with the
[`Oudin`](../pet/testdata/oudin-reference.md) and
[`GR4J`](../rainrun/testdata/README.md) fixtures. GR4J regeneration uses actual
airGR Fortran; the source and compiled binaries are not vendored here.

## Input and boundary contracts

**Terrain:** use consistent horizontal/elevation units; the regression grid
uses metres and eight neighbours. Every grid edge and cell adjacent to an
inactive cell acts as an open boundary. A no-data hole is therefore an outlet
under this policy. Missing observations, lakes, sea masks, and closed
boundaries need an explicit caller policy. Tiny artificial height increments
resolve flats; filled differences are not automatically observed pond depths.
Repeatability is verified on the fixtures, not proved for all possible DEMs.
Equal-sized boundary components and one-cell flat handling remain targets for
further adversarial evaluation.

`tem.NewFromRealWithDistances(r grid.Real, distance func(from, to int) float64)`
returns `(*tem.TEM, error)` and uses caller-supplied horizontal distances for
D8 routing, including routing rebuilt by `FillDepressions`. All directed
active-neighbour distances are checked for finite positive values before
routing; use the same length unit as elevation. The callback must be pure and
stable for the model's lifetime. No per-edge distance cache is allocated;
geographic-grid callers can precompute row spacing while retaining their native
cells and georeferencing. Nil preserves `NewFromReal`'s square-cell routing.

This callback affects D8 routing only. `TEC.G`/`TEC.A` still use the existing
square-grid slope/aspect calculation, and depression filling/flat correction
are unchanged. `ClipToActives` and `SubSet` retain the callback and original
cell IDs. Gob serialization retains the graph but cannot retain a function;
reconstruct with the distance-aware constructor before rerouting saved terrain.
Successful legacy `(*TEM).New(path)` loads clear any previous callback, so
reusing a model cannot apply the old grid's distances to newly loaded terrain.
Returned load errors leave the callback unchanged.
Existing constructors and keyed `TEM` literals remain source-compatible; any
external positional `TEM` literal must become keyed because `TEM` now retains
a private callback.

**SCS retention:** `Scscn` accepts metres of cumulative event rainfall
**after initial abstraction**, a normal-condition curve number, and AMC
`1`, `2`, or `3` for dry, normal, or wet conditions. Its return is
post-abstraction event retention. Callers define event boundaries, account for
initial abstraction, validate finite nonnegative rain and supported CN/AMC
values, and difference cumulative results if increments are required.
It is not a soil-profile infiltration-rate or groundwater-recharge model.
The sampled extremes test arithmetic; they do not establish empirical
applicability across every sampled curve number.

**Oudin:** radiation is extraterrestrial energy in MJ/m²/day, temperature is
daily mean Celsius, and output is m/day. Surface incoming solar radiation is
not interchangeable with extraterrestrial radiation. The evaluated formula
uses temperature-dependent latent heat and water density; airGR uses constant
effective values. The reference tests document that difference.

**GR4J:** use consistent metre-based depths, inputs and capacity/exchange
parameters with the current constructor; `X4` remains in days. The first
`Update` result is soil evaporation; total actual evaporation includes
`min(rain, PET)`. `Storage()` excludes water queued in the two hydrographs.
The reference tests set explicitly equal initial stores to compare daily
equations; they do not validate the constructor's initialization optimizer.

## Remaining limitations

- `PenmanMonteith` is unfinished and exits the process even for valid inputs.
- `snowpack.HMETS.Update` remains an unfinished stub that panics.
- `SineCurvePET` allocates approximately half the supplied annual total and
  uses a fixed seasonal shape.
- Penman's air-density approximation does not account for pressure; its wind
  coefficients and valid domain require separate interpretation.
- Several routines accept invalid values or terminate the process instead of
  returning errors. Validate application inputs before calling them.
- Potential evaporation is atmospheric demand. Deriving actual soil/crop
  losses requires water availability and surface/crop assumptions.
- Agreement with another implementation and consistent graph topology do not
  establish field accuracy. Soil, weather, terrain data and calibration remain
  application responsibilities.

These limitations are not corrected by this maintenance series. Keep an
application adapter explicit about which functions it exposes.

## Consuming and maintaining the fork

The module identity stays `github.com/maseology/goHydro` to preserve upstream
imports. A consuming application's `go.mod` can replace that module with a
**pinned commit or version** of `github.com/nightisyang/goHydro`. Keep the
replacement in the consuming application; local filesystem replacements do
not belong in this fork. No application dependency or terrain engine is
switched by these commits.

Retain upstream history and small, reviewable fixes. Review incoming upstream
changes and run the regressions before updating a pin. The correction commits
and independent test cases can support future upstream contributions; no
upstream submission is implied by publishing this fork.
