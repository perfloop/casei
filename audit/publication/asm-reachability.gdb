# Run against a root test binary on an amd64 AVX-512 host. Each breakpoint
# disables itself after the first hit, so the output is one HIT line per entry.
set pagination off
set confirm off
set print thread-events off
break github.com/tsenart/casei.rootSkip32.abi0
break github.com/tsenart/casei.rootSkip64.abi0
break github.com/tsenart/casei.literalSkip32.abi0
break github.com/tsenart/casei.literalSkip64.abi0
break github.com/tsenart/casei.runMask32.abi0
break github.com/tsenart/casei.runMask64.abi0
break github.com/tsenart/casei.probeSkip32.abi0
break github.com/tsenart/casei.probeSkip64.abi0
break github.com/tsenart/casei.asciiOnlyProbeSkip64.abi0
break github.com/tsenart/casei.pairSkip32.abi0
break github.com/tsenart/casei.pairSkip64.abi0
break github.com/tsenart/casei.pairSetSkip32.abi0
break github.com/tsenart/casei.pairSetSkip64.abi0
break github.com/tsenart/casei.pairPairSkip64.abi0
break github.com/tsenart/casei.pairSecondSkip32.abi0
break github.com/tsenart/casei.pairSecondSkip64.abi0
break github.com/tsenart/casei.filterSkip32.abi0
break github.com/tsenart/casei.filterSkip64.abi0
break github.com/tsenart/casei.triplePairSkip32.abi0
break github.com/tsenart/casei.triplePairSkip64.abi0
break github.com/tsenart/casei.tripleMixedSkip64.abi0
break github.com/tsenart/casei.tripleASCIIUTF8Skip64.abi0
break github.com/tsenart/casei.tripleSharedPrefixSkip64.abi0
break github.com/tsenart/casei.tripleSkip32.abi0
break github.com/tsenart/casei.tripleSkip64.abi0
break github.com/tsenart/casei.pairShuftiSkip64.abi0
break github.com/tsenart/casei.pairShuftiWithOnesSkip64.abi0
break github.com/tsenart/casei.asciiPairDirectSkip64.abi0
break github.com/tsenart/casei.asciiPairShortSkip64.abi0
break github.com/tsenart/casei.tripleShuftiSkip64.abi0
break github.com/tsenart/casei.asciiPairAnchorSkip64.abi0
break github.com/tsenart/casei.probeVBMISkip64.abi0
break github.com/tsenart/casei.asciiPairDirectVBMISkip64.abi0
break github.com/tsenart/casei.asciiPairAnchorVBMISkip64.abi0
break github.com/tsenart/casei.pairPairWordSkip64.abi0
break github.com/tsenart/casei.pairPairVBMISkip64.abi0
commands 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36
silent
printf "HIT %d\n", $_hit_bpnum
disable $_hit_bpnum
continue
end
run -test.run ^Test -test.count=1
