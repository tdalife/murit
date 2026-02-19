# Changelog

All notable changes to this project are documented in this file.

This changelog follows the spirit of [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and uses [Semantic Versioning](https://semver.org/).

## [1.0.0] - 2026-02-19

### Added
- Canonical multifiltration input for clique complexes via `--complex`:
  - CSV header `N,k`,
  - one row per 1-simplex `(i,j)` with grouped JSON generator list,
  - support for incomparable generators per 1-simplex.
- Generator normalization to canonical minimal antichains:
  - duplicate generator removal,
  - dominated generator pruning.
- Unified path ingestion through `--path`:
  - inline JSON literal support,
  - path-file support (one path literal per non-empty line).
- Multi-path run orchestration with deterministic outputs:
  - `*_pathNN.aux`,
  - `*_pathNN.ripser`.
- Ripser executable resolution via `--ripser`:
  - `--ripser` uses `PATH`,
  - `--ripser <path>` uses an explicit executable.
- Expanded automated tests in `main_test.go` for parser validation, normalization, path validation, entry-index behavior, and ripser output translation.
- New example package centered on a digital figure-8:
  - `examples/figure8.csv`,
  - `examples/figure8.paths`,
  - `examples/figure8_overview.png`.

### Changed
- Core model terminology and implementation now consistently use:
  - **multifiltered clique complex**,
  - **multifiltration**,
  - 1-simplex generator antichains.
- CLI and docs were rewritten around the new model and file formats.
- Auxiliary matrix generation remains multithreaded, with deterministic row write order.
- Ripser threshold is now set internally to `len(path)` for each path run.
- Go module baseline updated to `go 1.26`.

### Removed
- Legacy distance-matrix + minima input pipeline.
- Legacy CLI assumptions and old-format behavior.
- User-facing `--threshold` flag.
- Legacy example assets:
  - `examples/diamond.dist`,
  - `examples/diamond.minima`,
  - `examples/test.dist`,
  - `examples/test.minima`.

### Fixed
- Robust translation of ripser output including infinite deaths (`inf`) and ANSI control-sequence cleanup.
- Preservation of “omitted 1-simplices never appear” semantics along a path.

### Breaking Changes
- **Input format is not backward-compatible** with pre-1.0 releases.
- `--complex` is required; old metric/minima inputs are no longer supported.
- `--path` uses JSON path-literal syntax (inline or per-line in a path file).
- `--threshold` has been removed from the public CLI.

## [0.4.0] - 2022-06-20
- Historical release (pre-changelog details are preserved in git history and tags).

## [0.3.1] - 2022-06-20
- Historical release (pre-changelog details are preserved in git history and tags).
