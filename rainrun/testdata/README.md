# GR4J independent reference

`gr4j-reference.json` contains daily outputs from the unmodified airGR 1.7.9
Fortran implementation at CRAN mirror commit
[`4bb401c6529e20116db357369c9f2bc34f959992`](https://github.com/cran/airGR/tree/4bb401c6529e20116db357369c9f2bc34f959992).
The generator compiles `src/frun_GR4J.f90` and `src/utils_D.f90` and calls the
actual `frun_gr4j` entry point. It does not implement the model equations or
read any goHydro outputs. The fixture records SHA-256 hashes of both source
files, which the generator checks before compiling.

The reference implements the daily GR4J model described by Perrin, Michel and
Andréassian (2003), [Journal of Hydrology 279, 275–289](https://doi.org/10.1016/S0022-1694(03)00225-7).
These synthetic regressions check implementation agreement, not calibration or
forecast accuracy against observed flow.

## Coverage and state convention

- Six time constants (`X4 = 0.5, 0.75, 1, 1.7, 3.5, 8` days) crossed with
  three exchange coefficients (`X2 = -3, 0, 3` mm/day), each run for 128 days.
  The forcing includes wet spells, a 40-day drought and a 180 mm storm.
- Three further runs cover empty stores without rain (30 days), dry recession
  (120 days), and an isolated 100 mm storm (120 days).
- All 2,574 days retain five reference outputs: runoff, total actual
  evaporation, percolation, production storage and routing storage.

Forcings, states and output depths use millimetres; daily fluxes use mm/day.
The parameter order is production capacity `X1` (mm), exchange coefficient
`X2` (mm/day), routing capacity `X3` (mm), and hydrograph time constant `X4`
(days). The Go test converts every depth/flux to metres and leaves `X4` in days.

Both implementations start with the same **explicit** production/routing
stores listed in each case; all hydrograph states start at zero. Matrix cases
use 175 mm and 30 mm respectively. The same-package Go test calls `New` to
construct the model, then assigns these two store values. Reference outputs
therefore do not depend on the Go constructor's initial-discharge optimization,
and this comparison does not validate that optimization. A separate test runs
the real constructor and `Update` at `X4=0.5` without overriding its stores.

airGR's 67-slot `StateStart` uses the first two entries for the production and
routing stores, slots 8–27 for UH1 and slots 28–67 for UH2 (one-based indexes).
The generator initializes all other entries to zero. It requests `MISC`
outputs 18, 6, 7, 3 and 11, in that order. goHydro's first `Update` return value
is soil evaporation; adding `min(rain, PET)` makes it comparable with airGR's
total actual evaporation.

The absolute comparison tolerance is `0.00001` mm (10 nanometres), well below
meaningful rainfall precision. airGR declares its routing split as
`doubleprecision, parameter :: B=0.9`: the unsuffixed literal first rounds to
single precision, unlike Go's constant. That small split difference propagates
through the routing state. The original fixture check on macOS/arm64 with
Go 1.26.2 and GNU Fortran 16.2.0 observed maximum runoff and routing-store
differences of `9.85e-7` mm/day and `6.43e-7` mm, respectively. All other
checked outputs differed by less than `4e-13` mm.

Separate impulse tests compare half-day and ordinary hydrographs with literal
S-curve increments, including delayed discharge, conservation and complete
buffer drainage. Zero-exchange reference cases also check each daily water
balance, including both delay buffers: `Storage()` alone excludes that water.

## Regeneration

Normal Go tests read the checked-in JSON and require neither Python nor
Fortran. To regenerate, install Python 3, Git and gfortran, then run from the
goHydro root:

```sh
git -c core.autocrlf=false clone https://github.com/cran/airGR.git /tmp/airgr-reference
git -C /tmp/airgr-reference checkout 4bb401c6529e20116db357369c9f2bc34f959992
python3 rainrun/testdata/generate_gr4j_reference.py /tmp/airgr-reference
go test -mod=readonly ./rainrun -run '^TestGR4J' -count=1
```

The generator needs only Python's standard library. It invokes
`gfortran -O2 -shared -fPIC` on the two pinned files, loads the resulting library
with `ctypes`, and deletes the temporary library on exit. The Go repository
contains neither the airGR source nor compiled reference binaries. Use
`--compiler` to choose a compatible gfortran executable and `--output` to
write a comparison fixture elsewhere. Regeneration on another platform may
change the last floating-point digits; compare numerically within the stated
tolerance, rather than requiring byte identity across compilers.
