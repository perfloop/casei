# CONTEXT — the known frontier of case-insensitive substring search

This document catalogs every technique known to this problem as of August
2026, with sources and measured numbers. It exists to draw one line: **an
approach only counts as new if it is not in this document.** Anything below
is fair game to use, combine, and tune — but using it is engineering, not
invention.

## 1. State of the art, measured

From rebar's published curated results (2025-12-19, x86-64), the
case-insensitive literal benchmark `sherlock-casei-en` (English, ASCII-heavy):

| engine | exact | caseless | caseless penalty |
|---|---|---|---|
| hyperscan | 32.0 GB/s | **29.2 GB/s** | ~1.1× |
| pcre2/jit | 26.3 GB/s | 18.8 GB/s | 1.4× |
| rust/regex | 29.4 GB/s | 10.5 GB/s | 2.8× |
| re2 | 12.1 GB/s | 2.6 GB/s | 4.7× |
| go/regexp | 4.2 GB/s | 46.3 MB/s | ~90× |

And `sherlock-casei-ru` (Russian — true UTF-8 case folding):

| engine | exact (ru) | caseless (ru) |
|---|---|---|
| pcre2/jit | 32.8 GB/s | **18.0 GB/s** |
| hyperscan | 4.3 GB/s | 7.4 GB/s |
| rust/regex | 35.6 GB/s | 8.4 GB/s |
| re2 | 768 MB/s | 948 MB/s |
| go/regexp | 2.1 GB/s | 48.6 MB/s |

Notably, rebar includes `rust/memchr/memmem` — the reference exact-match
substring engine — and it is absent from every caseless benchmark: no
dedicated caseless substring engine exists in that suite at all. The
caseless columns are contested only by general regex engines, and **no
dedicated case-insensitive UTF-8 substring algorithm exists anywhere** —
beyond ASCII, every engine routes through general regex machinery.

In Go, the strongest published caseless search is
[mhr3/veloz](https://github.com/mhr3/veloz) `ascii.IndexFold` (NEON on arm64
with AVX2/SSE amd64 ports). Open PRs on that repository
([#2](https://github.com/mhr3/veloz/pull/2),
[#4](https://github.com/mhr3/veloz/pull/4),
[#5](https://github.com/mhr3/veloz/pull/5),
[#6](https://github.com/mhr3/veloz/pull/6)) carry a staged
adaptive-prefilter design measured (Apple M3 Max) at **74–77 GB/s** on
64KB–1MB miss scans — about 6× the single-pass kernel it replaces — with
published tables across M3 Max, Graviton 3, and Graviton 4. Those PRs, their
benchmark tables, and their design are all prior art for this arena, and
their techniques are itemized below.

The physics: on cached haystacks a modern core streams tens of GB/s
single-threaded, and the M3 Max numbers above show ~76 GB/s is reachable
*with* folding. Incumbent engines at 10–29 GB/s are nowhere near the
bandwidth ceiling. The headroom is real.

## 1b. Unicode simple folding: the terrain

Facts that shape any fast UTF-8 caseless algorithm (see Unicode
CaseFolding.txt, C+S mappings; implemented as `unicode.SimpleFold` in Go):

- Fold orbits are tiny — almost all have 2 members, a few have 3 (σ/ς/Σ,
  k/K/U+212A KELVIN, s/S/U+017F LONG S, å/Å/U+212B ANGSTROM, µ/U+03BC/Μ) —
  and there are only ~1,400 multi-member orbits in all of Unicode.
- **The ASCII hazard set is tiny and needle-computable**: for a pure-ASCII
  needle, the only non-ASCII code points that can participate in a match
  are the non-ASCII fold-mates of its letters — in practice K (U+212A,
  3 bytes) for k, ſ (U+017F, 2 bytes) for s, and nothing else. A caseless
  ASCII scan is *almost* semantically complete; what stops it is a finite,
  precomputable byte-pattern set.
- Windows change byte length across fold-mates (k = 1 byte, K = 3), so a
  fixed-window verify strategy cannot be sound in general; matching is per
  code point with an offset map back to bytes.
- Simple folding is locale-independent: İ (U+0130) and ı (U+0131) fold only
  to themselves. Full folding (ß→ss, length-changing in code points) is a
  DIFFERENT relation and explicitly out of scope here — it is what makes
  "caseless" ill-defined in many libraries; simple folding is what regex
  engines implement.
- `ToLower`/`ToUpper` normalization is NOT folding: it splits the sigma
  orbit (ς stays ς, Σ→σ), re-encodes bytes, and shifts offsets. The common
  idiom is both slow and wrong.

What engines do today for caseless UTF-8 literals: expand the literal's
case variants into alternations / byte classes and feed general multi-
pattern machinery — Teddy buckets (rust/regex), FDR (Hyperscan), UTF-8
automata (regex-automata, RE2). That is the known approach, and the §1
Russian numbers are its cost. Also known but unfused: simdutf-class SIMD
UTF-8 validation/classification runs at tens of GB/s — nobody has fused
that classification with a search loop.

## 2. Case-folding primitives (in-register, branchless)

All known ways to fold or compare ASCII case without a branch per byte:

- **OR 0x20 with a precomputed mask**: when the byte you compare against is
  known (a prefilter byte), fold the haystack with a single `OR` whose mask
  is 0x20 if that byte is a letter, 0x00 otherwise — chosen once at setup.
  The compare target is stored pre-lowercased. Variant: duplicate the whole
  scan loop for the non-letter case to delete even that OR.
- **Signed-range trick (x86)**: `PADDB 0x1f` maps 'a'..'z' to 0x80..0x99, a
  single signed `PCMPGTB` against 0x9a isolates lowercase, then subtract
  0x20 under that mask. Four ops, no table (veloz amd64).
- **Unsigned-wraparound detects**: `(b+133) ≥ᵤ 230` detects 'a'..'z' (the
  wraparound doubles as the upper bound); `(b+191) <ᵤ 26` detects 'A'..'Z';
  `((b|0x20)+159) <ᵤ 26` detects letters of either case.
- **Shifted-domain TBL fold (NEON)**: pre-subtract 0x60 so 'a'..'z' land on
  table indexes 1..26; a two-register `TBL` (0x20 at those slots, 0
  elsewhere, out-of-range → 0) yields the fold delta; comparisons can stay
  in the shifted domain, skipping the un-shift.
- **Allow-mask pair fold (veloz PR #7)**: to compare two vectors caselessly,
  compute `allow = TBL(table, (a & b) − 0x40)` and test
  `(a ^ b) BIC allow == 0`. One table lookup per *pair* instead of folding
  each operand: if the XOR difference is exactly bit 5 and both operands are
  letters, `a & b` indexes the 'A'..'Z' rows. 5 vector ops per 16 bytes.
- **XOR-tolerance verify (raw needle)**: `tolerable = 0x20 where
  (diff == 0x20 AND is_letter)`, then `diff ^ tolerable == 0`. Symmetric,
  no tables, ~9 ops.
- **Needle-derived fold mask (prefolded needle)**: when the needle is known
  lowercase, fold the haystack *only at positions where the needle byte is a
  letter* and compare exactly elsewhere: ~6 ops. The pattern-side fold is
  paid once at setup.
- **SWAR (scalar words)**: Mycroft-style range mask over 8 bytes,
  `folded = x − (mask >> 2)`.
- **Hash-domain fold**: map `c ∈ 'a'..'z' → c − 0x80, else c − 0x60` before
  hashing; both cases collapse to one value and the uniform shift cancels in
  comparison.
- **256-byte fold LUT** for scalar tails; **32-byte TBL** beats it in vector
  code.

## 3. Prefilter designs

- **Single rare-byte broadcast scan** (memchr-style stage 1): compare every
  haystack byte against one needle byte chosen for rarity, verify at hits.
- **Two-byte anchor at fixed offsets**: two comparison streams at
  `base+off1` and `base+off2` advanced in lockstep (two cursors — no
  cross-vector shifting or realignment needed); AND the two match masks.
  Candidates must match both anchors, quadratically rarer false positives.
- **first2/last2 word anchors, case-folded (veloz amd64)**: match 16-bit
  pairs from both ends of the needle with `PCMPEQW` on folded data; odd
  alignments via `PALIGNR(15)` against the *previous folded block* (no
  reload, no re-fold); collapse even/odd movemasks with
  `m & (m>>1) & 0x5555`. Middle-only verification afterwards — the four
  anchor bytes are already proven, needles ≤ 4 need no verify at all.
- **Anchor choice policies**: (a) positional — first+last byte, middle if
  they're equal (Two-Way/Muła lineage); (b) statistical — the two rarest
  distinct bytes by a background frequency table; (c) hybrid — statistical
  choice, overridden to first+last spread when the chosen pair is adjacent
  *and* both bytes are common (adjacent common bytes give correlated false
  positives on periodic text).
- **Teddy** (Rust regex / Hyperscan lineage): nibble-indexed shuffle
  prefilter matching up to 8 short literals at once; handles caseless by
  putting both case variants in the mask tables.
- **FDR** (Hyperscan): bucketed shift-or over reversed domain masks —
  Hyperscan's main literal engine; caseless via bucket duplication.
- **SVE2 `MATCH`**: one instruction tests each haystack byte against a
  16-token set — both case variants of two rare bytes interleaved gives a
  dual-anchor caseless prefilter in two instructions per vector; positions
  via `BRKB`+`CNTP`; tails via `WHILELT` predication (no mask tables).

## 4. Candidate extraction and iteration (movemask-less ISAs)

- **2-bit syndrome (ARM optimized-routines lineage)**: AND the compare mask
  with `0x4010040140100401`, two `ADDP.B16` folds compress 16 lanes into a
  32-bit word (2 bits/byte); first hit via `RBIT`+`CLZ`, then `>>1`.
- **SHRN-#4 nibble movemask**: narrow a 16-byte 0x00/0xFF mask to a 64-bit
  word (4 bits/byte); full-equality test via `CMN #1`.
- **Any-match gate before extraction**: `ADDP.D2` + `FMOV` (2 ops) tests
  "any lane set" in the hot loop; syndrome extraction is deferred until a
  hit is known. `UMAXV.4S` (4 wide lanes, fewer reduction stages) where the
  vector holds raw difference bits rather than lane masks.
- **Syndrome-persistent iteration**: keep the syndrome live in a GPR across
  candidate verifications; retire a failed candidate by clearing its bit
  (`BIC`), re-extract the next — never rescan memory for the next candidate
  in a chunk.
- **Superblock OR-trees with preserved partials**: 128-byte iterations
  reduce 8 compare masks through an OR tree but *retain* the per-64B
  intermediates, so locating the hit re-reduces the saved partials instead
  of re-comparing memory.

## 5. Verification

- **Full-window SIMD recompare** with any §2 primitive; `VEOR` + reduction.
- **Skip-known-bytes**: after a dual-anchor hit, verify only the middle
  (`needle[2 : n−2]`).
- **Masked-tail verify**: load a full 16B block past the tail, AND the
  difference with `tail_mask_table[remaining]` (16 prefix-ones rows,
  scaled-register load). Beware: this overreads by up to 15 bytes — safe
  only with explicit buffer-slack guarantees, otherwise it page-faults. The
  overread-free alternative assembles the tail from 8/4/2/1-byte scalar
  loads.
- **Positional slack as the bounds proof**: anchoring the scan at
  `haystack + off1` with trip count in candidate positions means every
  prefilter load is in bounds by construction — no page logic in the scan.

## 6. Rolling hashes, vectorized

- **Scalar Rabin-Karp** (Go stdlib): polynomial hash, PrimeRK = 16777619
  mod 2³², roll by one multiply and two multiply-adds per position.
- **SIMD block hashing**: extract bytes per 32-bit lane, per-lane Horner
  with p, p², p³, weight lanes by {p¹², p⁸, p⁴, 1}, horizontal add; 64-byte
  blocks combine via {p⁴⁸, p³², p¹⁶, 1} and a per-iteration p⁶⁴ multiply.
  Cuts the sequential dependency chain ~64×. Hash the needle and the first
  window in the same interleaved loop.
- **4-way parallel rolling (veloz PRs)**: keep rolling hashes for four
  consecutive alignments in vector lanes; compute per-step deltas
  `new − pow·old` (negate `pow` so removal becomes a multiply-add), build
  the four shifted delta windows with `EXT #12/#8/#4`, chain multiply-adds
  by p⁴..p; the only loop-carried operation is one multiply by p⁴ per four
  positions. Candidate gate: broadcast target hash, `CMEQ.S4`, max-reduce.
- **Fold inside the hash**: apply a §2 wraparound fold in the hashing loop
  and the hash domain itself becomes caseless. Prefolded-needle variant:
  hash the needle raw (already lowercase) and fold only haystack bytes —
  but see §8, this exact variant shipped broken.
- **Reversed-polynomial rolling hash** (Matt Sills): alternative formulation
  avoiding the leading-power multiply; implemented in the veloz lineage but
  never shipped.

## 7. Adaptive control

- **Stage escalation**: 1-byte filter → 2-byte filter → rolling hash, each
  stage stronger and costlier. Known ABI: kernels return
  `position | NOT_FOUND | (EXCEEDED-flag + resume position)` in one word,
  and the orchestrator re-slices the haystack so the next stage never
  rescans cleared territory.
- **Attrition budgets**: count *failed verifications*, bail to the next
  stage when `failures > B + scanned/K`. Published values: Go stdlib
  cutover ≈ 4 + n/16; veloz C lineage 4 + n/16 and 4 + n/256 with
  needle-length-adaptive thresholds; veloz hand kernels 32 + n/8
  (recomputed per failure). Evaluate the budget only when candidates exist
  so the clean hot path pays nothing.
- **Stage-skip heuristics by pattern statistics**: skip the 1-byte stage
  when the filter byte is too common (rank cutoffs ~200/240 of 255) or the
  haystack is small (< 2KB); go straight to the hash stage for long needles
  (> 64B) whose anchors are common (rank > 180).
- **Guaranteed-linear claims**: a hard budget per filter stage plus a
  linear terminal stage bounds *filter* work; note a 32-bit rolling hash
  still admits adversarial collision inputs (O(n·m) verification), so
  worst-case claims must be phrased carefully.

## 8. Rare-byte statistics

- **Background byte-frequency rank table** (Rust memchr lineage; corpus:
  CIA World Factbook, rustc source, Septuaginta), 0 = rarest.
- **UTF-8 lead-byte override**: force 0xC0–0xFF to "most common" so the
  selector prefers continuation bytes (which discriminate between
  characters) over lead bytes (shared across whole ranges).
- **Fold-aware ranks**: `rank_ci(letter) = rank(upper) + rank(lower)`, both
  case slots — a folded filter byte fires on both cases, so 'e'/'E' is even
  worse as a caseless filter than 'e' is as an exact one. (Caveat: rank
  tables are rank-ordered, not frequency-proportional, so the sum is a
  heuristic.)
- **One-pass rarest-distinct-pair selection** with demotion; O(1) sampled
  variants (8 spread positions) for one-shot calls.
- **Corpus-tuned tables**: build the rank table from a sample of the actual
  data (fold counts while building); published claims of ~2× from
  domain-tuned ranks.

## 9. Known traps (all bitten real implementations of this exact problem)

These are encoded as tests in this repository:

1. **0x20-adjacent non-letters**: `[`/`{`, `@`/`` ` ``, `]`/`}`, `\`/`|`,
   `^`/`~` differ by exactly bit 5 and must NOT match. A fold sequence that
   ORs 0x20 without a letter test, or a verify that compares against the
   wrong constant register, accepts them. A shipped NEON kernel in this
   problem's lineage had exactly that bug in its scalar-tail verify path,
   reachable for needles ≥ 16 bytes.
2. **Mixed fold directions**: one shipped rolling-hash variant folded the
   haystack toward *uppercase* while its verifier folded toward *lowercase*
   against a pre-lowercased needle — every letter-containing needle
   silently returned "not found," and no test caught it. Hash domain,
   verify domain, and needle normalization must agree, and the agreement
   must be tested.
3. **Verify-tail overreads**: masked-tail verification reads up to 15 bytes
   past the end of the needle or haystack. Language runtimes do not
   guarantee that slack; unmapped-page edges fault.
4. **Quadratic cliffs**: dual-anchor prefilters degrade to near-full
   verification per position on periodic inputs (`abab…`, `aaaa…`,
   `(a³¹b)ⁿ`). Without an attrition budget and a linear fallback, worst
   case is O(n·m). The adversarial scenarios exist to catch this.
5. **`ToLower` is not folding**: normalization idioms silently diverge from
   fold semantics on the sigma orbit and re-encode bytes, shifting offsets.
   Any "normalize then exact-search" design must use canonical *fold*
   normalization and carry an offset map.
6. **Length-changing windows**: fold-mates differ in UTF-8 byte length
   (k vs U+212A), so fixed-stride window verification is unsound beyond
   ASCII; the trap tests include such windows.
7. **Estimation is not measurement**: predicted speedups in this domain are
   routinely wrong in both directions. Every claim in this arena is a
   benchmark run, never an extrapolation.

## 10. Sources

- rebar (BurntSushi): benchmark harness + published results —
  github.com/BurntSushi/rebar
- Rust memchr crate: `memmem`, packed-pair prefilter, byte-frequency ranks
- mhr3/veloz + PRs #1–#8: Go SIMD ASCII library; staged caseless kernels,
  allow-mask EqualFold, amd64 folded dual-anchor
- Hyperscan (Intel) / Vectorscan: FDR and Teddy literal engines
- ARM optimized-routines: the 2-bit syndrome technique
- Wojciech Muła: SIMD-friendly string matching catalog
- SMART (Faro & Lecroq): the exact-matching algorithm corpus
- Go stdlib `strings`/`internal/bytealg`: Rabin-Karp cutover shape
- Matt Sills: reversed-polynomial Rabin-Karp
- Unicode CaseFolding.txt + UTS#18 (case-insensitive matching semantics)
- simdutf (Lemire et al.): SIMD UTF-8 validation/classification at GB/s
