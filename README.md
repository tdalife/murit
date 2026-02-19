# MuRiT

## What MuRiT Computes
`MuRiT` computes **pathwise persistent homology** for a multifiltered clique complex.

Input model:
- finite vertex set `V = {0,...,N-1}`,
- parameter dimension `k`,
- for each listed edge `e = (i,j)`, a non-empty set of minimal generator vectors `M_e ⊂ R^k` (an antichain),
- a totally ordered path `p_1, ..., p_T` in `R^k` (coordinatewise non-decreasing).

Semantics:
- edge `e` is present at filtration value `x` iff `exists m in M_e: m <= x` coordinatewise,
- the complex at `x` is the clique complex on all present edges.

Pathwise reduction used by MuRiT:
- `entry(e) = min{ t in {1,...,T} : exists m in M_e with m <= p_t }`,
- if no such `t`, then `entry(e) = T+1` (never appears on this path),
- write lower-triangular auxiliary matrix with these integer entry indices,
- run ripser on that auxiliary matrix,
- translate interval endpoints from indices back to filtration vectors on the path.

## Run
Run with the `murit` executable:
```bash
murit --complex <file.csv> --path <path-literal-or-path-file> [--options]
```

If `murit` is not in `PATH`, use the executable path directly.

`--ripser` behavior:
- omit `--ripser`: do not run ripser,
- `--ripser`: use `ripser` from `PATH`,
- `--ripser <path>`: use the provided ripser executable path.
If ripser cannot be resolved, MuRiT exits with an error.

## Input and Output Formats

### Complex Input (`--complex`)
CSV with comments/blank lines allowed:
```text
# comments are allowed
N,k
i,j,"[[a1,...,ak],[b1,...,bk],...]"
```

Rules:
- `N >= 1`, `k >= 1`
- `0 <= i < j < N`
- one row per edge
- generator list must be valid JSON, non-empty, vectors of length `k`
- duplicate `(i,j)` rows are rejected
- dominated generators are pruned automatically to the minimal antichain
- omitted edges are treated as permanently absent

### Path Input (`--path`)
One syntax only: a **path literal** = JSON array of vectors:
```text
[[a1,a2,...],[b1,b2,...],...]
```

`--path` accepts:
- inline literal (argument starts with `[`),
- or a `.paths` file where each non-empty non-comment line is one path literal.

Validation:
- each vector has length `k`,
- vectors are coordinatewise non-decreasing along the path,
- empty path is invalid.

### Output Files
When `--path` is a path file, MuRiT writes per-path files:
- `<complex-stem>_path01.aux`
- `<complex-stem>_path01.ripser` (with `--ripser`)
- etc.

File meanings:
- `*.csv`: complex input.
- `*.paths`: one path literal per line.
- `*_pathNN.aux`: lower-triangular integer entry-index matrix.
- `*_pathNN.ripser`: ripser output with interval endpoints translated to filtration vectors.

Ripser threshold behavior:
- MuRiT always sets ripser threshold to `len(path)` internally so omitted 1-simplices remain absent on the path.

## Basic Usage
```bash
murit \
  --complex examples/figure8.csv \
  --path examples/figure8.paths \
  --ripser --dim 1
```

For the full figure-8 walkthrough (commands + interpretation), see:
- `examples/README.md`

## Example Visualization
- `examples/figure8_overview.png`

## Changelog
- `CHANGELOG.md`

## Citing
Maximilian Neumann, Michael Bleher, Lukas Hahn, Samuel Braun, Holger Obermaier, Mehmet Soysal, Rene Caspart, and Andreas Ott. "MuRiT: Efficient Computation of Pathwise Persistence Barcodes in Multi-Filtered Flag Complexes via Vietoris-Rips Transformations". arXiv, July 7, 2022. https://doi.org/10.48550/arXiv.2207.03394.
