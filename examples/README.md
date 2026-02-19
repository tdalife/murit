# Figure-8 Example Walkthrough

This directory contains one complete MuRiT example.

## What is tracked in git
- `figure8.csv`: multifiltration input for a clique complex (`N,k` header + edge generator sets).
- `figure8.paths`: three monotone path literals in `R^2`.

All filtration coordinates are normalized to `[0,1]` with midpoint `0.5`.

## What is generated locally (gitignored)
After running MuRiT, these files are created locally:
- `figure8_pathNN.aux`: pathwise auxiliary matrices.
- `figure8_pathNN.ripser`: translated ripser outputs.

Provided visualization artifact:
- `figure8_overview.png`: combined complex/path/persistence visualization.

## Run MuRiT on this example
From the repository root:

```bash
murit \
  --complex examples/figure8.csv \
  --path examples/figure8.paths \
  --ripser --dim 1
```

If `murit` is not in `PATH`, use the executable path directly.
If `ripser` is not in `PATH`, pass it explicitly with `--ripser <path-to-ripser>`.

## How to read the `.ripser` files
Intervals are shown as:
```text
[[birth_vector], [death_vector]):
```

Meaning:
- the class is born when the path first reaches `birth_vector`,
- the class dies when the path first reaches `death_vector`,
- `inf` means it does not die within the chosen path.

Expected behavior for this figure-8:
- path 1: two `H1` classes, deaths at `[1,0.5]` and `[1,1]`.
- path 2: two `H1` classes, deaths at `[0.5,1]` and `[1,1]`.
- path 3: two `H1` classes, both die at `[1,1]`.

Interpretation: path direction/order controls when each loop is born and filled.
