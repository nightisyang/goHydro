# Oudin regression references

`../oudin_test.go` contains synthetic numerical fixtures, not observed evaporation.
Ordinary Go tests need neither Python nor Fortran.

The daily-depth literals were evaluated with 50-digit Python `decimal` arithmetic:
`depth = Ra * (T+5) / (100 * lambda * rho)` metres/day,
`lambda = 0.000003*T*T - 0.0025*T + 2.4999` MJ/kg, and
`rho = 999.84` kg/m³ for `T <= 0`, otherwise
`rho = 0.00004*T**3 - 0.008*T*T + 0.0613*T + 999.85` kg/m³.
This checks the unit equation with the library's thermodynamic conventions,
without calling its helpers. For example, at 27°C, `lambda*rho` is exactly
2425.96958454654 MJ/m³ using these decimal coefficients.

The external reference uses unmodified [airGR `src/frun_PE.f90`](https://github.com/cran/airGR/blob/4bb401c6529e20116db357369c9f2bc34f959992/src/frun_PE.f90)
at commit `4bb401c6529e20116db357369c9f2bc34f959992`.
Its source SHA-256 is
`0f7d788c4d75b83c236848e01007d7da7dfa17bf43c93d84087ac9fde591965e`.
Its potential evaporation output is converted from mm/day to m/day.
The radiation passed to Go is independently computed from the Morton geometry
described in that source, using double-precision Python arithmetic.

airGR's radiation conversion implies a constant effective latent heat of
`28.5 * 0.0864 = 2.4624` MJ/kg and density of 1000 kg/m³.
The raw comparison allows 3.3% for the temperature-dependent thermodynamic
conventions, whose difference reaches 3.22% over 0–40°C. The second comparison
multiplies the airGR depth by `2462.4 / (lambda*rho)`; its absolute tolerance is
1e-9 m/day (1e-6 mm/day). This accommodates rounding of the original Fortran's
single-precision literals in the radiation calculation. No constants in the
Fortran reference are changed.

To regenerate the eight external-reference table rows, download the pinned
source into a scratch directory as `frun_PE.f90`, verify its SHA-256, and run:

```sh
gfortran -shared -fPIC -O2 frun_PE.f90 -o libairgr-pet.so
python3 - <<'PY'
import ctypes as C
from decimal import Decimal as D, getcontext
import math as M

getcontext().prec = 50
library = C.CDLL('./libairgr-pet.so')
run = library.frun_pe_oudin_
run.argtypes = [C.POINTER(C.c_int)] + [C.POINTER(C.c_double)] * 4
run.restype = None
cases = [(-45, 1, 40), (-45, 172, 0), (0, 80, 10), (0, 355, 27),
         (2.186, 1, 27), (2.186, 172, 40), (45, 1, 0), (45, 172, 10)]
for latitude, day, temperature in cases:
    f = M.radians(latitude)
    dec = .4093 * M.sin(day / 58.1 - 1.405)
    cz = max(.001, M.cos(f - dec))
    co = max(-1, min(1, 1 - cz / M.cos(f) / M.cos(dec)))
    om = M.acos(co)
    cp = max(.001, cz + M.cos(f) * M.cos(dec) * (M.sin(om) / om - 1))
    ra = 446 * om * cp * (1 + M.cos(day / 58.1) / 30) * .0864
    n = C.c_int(1)
    lat, temp, jd, out = map(C.c_double, (f, temperature, day, 0))
    run(C.byref(n), C.byref(lat), C.byref(temp), C.byref(jd), C.byref(out))
    t = D(temperature)
    heat = D('.000003') * t*t - D('.0025') * t + D('2.4999')
    rho = (D('999.84') if t <= 0 else
           D('.00004') * t**3 - D('.008') * t*t + D('.0613') * t + D('999.85'))
    print(latitude, day, temperature, repr(ra), repr(out.value / 1000), heat*rho)
PY
```
