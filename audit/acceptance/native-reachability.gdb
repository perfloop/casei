# Run against a root test binary on an amd64 AVX-512F/BW/VBMI host. Each
# breakpoint disables itself after the first hit, leaving one HIT line per
# changed native entry.
set pagination off
set confirm off
set print thread-events off
set startup-with-shell off
break github.com/tsenart/casei.literalSkipExact64.abi0
break github.com/tsenart/casei.pairPairConfirmVBMI64.abi0
break github.com/tsenart/casei.rawByteMultiAnchorSkip64.abi0
commands 1
silent
printf "HIT 1\n"
disable 1
continue
end
commands 2
silent
printf "HIT 2\n"
disable 2
continue
end
commands 3
silent
printf "HIT 3\n"
disable 3
continue
end
run -test.run ^(TestLiteralSkipExact64MatchesModel|TestUnicodePairVariableConfirm|TestRawByteMultiAnchorVBMISkip64MatchesTableModel)$ -test.count=1
