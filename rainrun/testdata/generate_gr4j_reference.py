#!/usr/bin/env python3
"""Regenerate GR4J fixtures using pinned, unmodified airGR Fortran (stdlib only)."""

import argparse
import ctypes as c
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile


REVISION = "4bb401c6529e20116db357369c9f2bc34f959992"
SOURCE_HASHES = {
    "src/frun_GR4J.f90": "d016fdc2297a00147868cddeae459ee02af2c467b95374b5b94b44a73dcbe3ce",
    "src/utils_D.f90": "3e8b94eff22def207cf61ba8efab59537ba659ec334b7fc2b4fa83eadcde1167",
}


def run_reference(model, parameters, initial_stores, forcing):
    days = len(forcing)
    rain = (c.c_double * days)(*(day[0] for day in forcing))
    pet = (c.c_double * days)(*(day[1] for day in forcing))
    parameters = (c.c_double * 4)(*parameters)
    # StateStart[0:2] are production/routing stores in mm. All 20 UH1
    # and 40 UH2 states, and the five unused slots, start at zero.
    start = (c.c_double * 67)(*initial_stores)
    end = (c.c_double * 67)()
    # Fortran MISC indexes: Qsim, total actual ET, Perc, Prod, Rout.
    columns = (c.c_int * 5)(18, 6, 7, 3, 11)
    output = (c.c_double * (days * 5))()
    n, np, ns, no = map(c.c_int, [days, 4, 67, 5])
    model(c.byref(n), rain, pet, c.byref(np), parameters, c.byref(ns),
          start, c.byref(no), columns, output, end)
    # Fortran is column-major. Retain every daily result, without rounding.
    return [[output[column * days + day] for column in range(5)]
            for day in range(days)]


def make_fixture(model):
    # Synthetic forcing uses only literal/integer arithmetic, no RNG/version
    # dependency. Include wet spells, 40 consecutive rainless days and a storm.
    rain_cycle = [0, 0, 12, 35, 0, 4, 22, 0, 0, 55, 0, 8, 1, 0, 18, 0]
    mixed = [[float(rain_cycle[day % len(rain_cycle)]),
              2.0 + (day % 13) * 0.25] for day in range(128)]
    for day in range(48, 88):
        mixed[day][0] = 0.0
    mixed[96][0] = 180.0
    forcings = {
        "wet_dry_storm": mixed,
        "zero": [[0.0, 3.0] for _ in range(30)],
        "recession": [[0.0, 0.0] for _ in range(120)],
        "isolated_storm": [[100.0, 0.0]] + [[0.0, 0.0] for _ in range(119)],
    }
    cases = []

    def add(name, parameters, initial, forcing):
        cases.append({
            "name": name,
            "parameters": parameters,
            "initial_stores_mm": initial,
            "forcing": forcing,
            "daily": run_reference(model, parameters, initial, forcings[forcing]),
        })

    for x4 in [0.5, 0.75, 1.0, 1.7, 3.5, 8.0]:
        for x2 in [-3.0, 0.0, 3.0]:
            add(f"x4_{x4:g}_x2_{x2:g}", [350.0, x2, 90.0, x4],
                [175.0, 30.0], "wet_dry_storm")
    add("zero_stores_no_rain", [350.0, 0.0, 90.0, 1.7], [0.0, 0.0], "zero")
    add("dry_recession", [350.0, 0.0, 90.0, 1.7], [175.0, 30.0], "recession")
    add("isolated_storm", [350.0, 0.0, 90.0, 1.7], [0.0, 30.0], "isolated_storm")
    return {
        "reference": "https://github.com/cran/airGR",
        "revision": REVISION,
        "source_sha256": SOURCE_HASHES,
        "units": "mm for depths, mm/day for daily fluxes, days for X4",
        "initial_hydrograph_stores": "all zero",
        "forcing_columns": ["rain_mm_day", "pet_mm_day"],
        "daily_columns": ["runoff_mm_day", "actual_et_mm_day", "percolation_mm_day",
                          "production_store_mm", "routing_store_mm"],
        "forcings": forcings,
        "cases": cases,
    }


def write_fixture(path, fixture):
    # Keep each numeric row on one line for a compact, reviewable fixture.
    lines = ["{"]
    for key, value in fixture.items():
        if key not in ("forcings", "cases"):
            lines.append(f"  {json.dumps(key)}: {json.dumps(value)},")
    lines.append('  "forcings": {')
    for index, (name, rows) in enumerate(fixture["forcings"].items()):
        lines.append(f"    {json.dumps(name)}: [")
        lines.append(",\n".join("      " + json.dumps(row, allow_nan=False) for row in rows))
        lines.append("    ]" + ("," if index + 1 < len(fixture["forcings"]) else ""))
    lines.extend(["  },", '  "cases": ['])
    for index, case in enumerate(fixture["cases"]):
        lines.append("    {")
        for key, value in case.items():
            if key != "daily":
                lines.append(f"      {json.dumps(key)}: {json.dumps(value)},")
        lines.append('      "daily": [')
        lines.append(",\n".join("        " + json.dumps(row, allow_nan=False) for row in case["daily"]))
        lines.extend(["      ]", "    }" + ("," if index + 1 < len(fixture["cases"]) else "")])
    lines.extend(["  ]", "}", ""])
    path.write_text("\n".join(lines), encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("airgr", type=Path, help="airGR Git checkout at the pinned revision")
    parser.add_argument("--compiler", default="gfortran")
    parser.add_argument("--output", type=Path,
                        default=Path(__file__).with_name("gr4j-reference.json"))
    args = parser.parse_args()
    airgr = args.airgr.resolve()
    revision = subprocess.check_output(["git", "-C", str(airgr), "rev-parse", "HEAD"], text=True).strip()
    if revision != REVISION:
        parser.error(f"airGR must be at {REVISION}, got {revision}")
    for name, expected in SOURCE_HASHES.items():
        if hashlib.sha256((airgr / name).read_bytes()).hexdigest() != expected:
            parser.error(f"source hash mismatch: {name}; use the unmodified pinned source")
    # Neither the source nor the temporary shared library is copied into goHydro.
    with tempfile.TemporaryDirectory(prefix="airgr-gr4j-") as directory:
        library_path = Path(directory) / "libairgr-reference.so"
        subprocess.run([args.compiler, "-O2", "-shared", "-fPIC",
                        *(str(airgr / source) for source in SOURCE_HASHES),
                        "-o", str(library_path)], check=True)
        library = c.CDLL(str(library_path))
        model = library.frun_gr4j_
        integer, double = c.POINTER(c.c_int), c.POINTER(c.c_double)
        model.argtypes = [integer, double, double, integer, double, integer,
                          double, integer, integer, double, double]
        model.restype = None
        fixture = make_fixture(model)
        write_fixture(args.output, fixture)
    print(f"Wrote {len(fixture['cases'])} cases to {args.output}")


if __name__ == "__main__":
    main()
