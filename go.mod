module github.com/tsenart/casei

go 1.22

require golang.org/x/sys v0.0.0-20220412211240-33da011f77ad

retract v0.1.0 // AVX-512 pair-tail search could miss matches after the first lane of a final 64-byte block.
