# MuRiT
Copyright © 2022 Michael Bleher, ...

## Description
MuRiT is a standalone program that determines a Vietoris-Rips transformation for multifiltered flag complex.
It's relevance stems from the fact that it can be used as a Ripser add-on to calculate persistent homology of the flag complex along any totally ordered subfiltration.

The main features of MuRiT

- fully parallelized and memory-efficient
- standalone exectubales for all major platforms
- open source go code
- optimized for use as Ripser add-on, integrated call to local instance of Ripser  

Input formats currently supported by MuRiT

- comma-separated edge annotation file & subfiltration
- comma-separated lower-triangular distance matrix, point annotation file, & subfiltration

## Releases
latest: v0.4

## Build or Modify
MuRiT is written in go, so it can be run directly from source if you have installed go.
This is the best way to go about modifying it to your own needs.

If you want to build your own executables, see here.

## Options
MuRiT offers several run-time options.

```
-debug
      show messages for debugging purposes?
-dist_file string
      distance file name
-fltr_file string
      filtration file name. Each row gives filtration value of corresponding data point.
       Format i,j,k,... (interpreted with lexicraphical order)
-help
      Get help message
-ripser
      run ripser?
-save_aux
      save auxiliary metric?
-sub_fltr string
      sub-filtration along which to compute 1d persistence.
       Format: [VR_0, i_0, j_0, k_0,...]-- ... --(VR_n, i_n, j_n, k_n,...)
-threads string
      number of threads
-verbose
      show status messages?
```

## Citing
TBA
