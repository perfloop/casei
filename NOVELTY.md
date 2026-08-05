# Novelty assessment: fold-orbit alphabet

## Status

**Negative assessment for the initially proposed construction.**  Compiling the
simple-fold orbits used by a pattern into class identifiers and scanning those
identifiers is useful engineering to evaluate, but it is not claimed here as a
new search construction.  It is the quotient-alphabet form of canonical case
folding / case-expanded matching.  No performance or novelty claim should be
made for that construction unless a later state transition is identified that
is not equivalent to this quotient.

## Precise construction assessed

For each decoded valid rune `r`, let `q(r)` be the identifier of its
`unicode.SimpleFold` orbit.  A compiler may restrict the assigned identifiers
to orbits occurring in the pattern set: an encountered rune from every other
orbit produces a distinguished mismatch token.  Invalid UTF-8 bytes remain
individual opaque tokens, rather than entering an orbit.  Each pattern is then
a sequence of tokens `q(r)` (or opaque-byte tokens).  A scan decodes the
haystack once, maps each decoded unit through `q`, and advances a matcher over
that token sequence.  For ASCII this can be represented by a 256-entry table;
KELVIN SIGN, LONG S, sigma, and other non-ASCII members require decoded-rune
escape handling.  A multi-pattern form shares the same `q` over the union of
pattern orbits.

The intended difference from the reference is operational: a candidate does
not call `foldHasPrefix` and re-enumerate a fold orbit at each candidate start.
It compares already-classified symbols instead.  This document's conclusion is
that this operational difference does **not** create a distinct state
representation: `q` is exactly canonical simple folding, with the canonical
rune replaced by an arbitrary dense class number.

## Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Case-expanded literal automata and UTF-8 automata | UTS #18; RE2 (`github.com/google/re2`); rust-regex / regex-automata (`github.com/rust-lang/regex`); summarized in `CONTEXT.md` §1b | Expanding every fold orbit at a pattern position and determinizing recognizes the same language as equality after `q`.  Quotienting equal alternatives into one class is the standard equivalent representation. |
| Teddy and FDR literal engines | Hyperscan, NSDI 2019; aho-corasick Teddy documentation; `CONTEXT.md` §§1b, 1d, 3 | Their byte/nibble masks encode finite sets of accepted byte forms before a confirmation stage.  A class table changes mask representation and when classification occurs, not the accepted transition relation. |
| `veloz` ASCII prefilter plus `EqualFold` confirmation | `github.com/mhr3/veloz`, including its AVX2 lineage; `CONTEXT.md` §§1, 2, 3, 5 | `veloz` keeps folding at candidate confirmation; the assessed plan moves it to stream classification.  That is a cost placement difference, not a new language or state machine. |
| Pre-folded corpus plus exact search | `arena/bench_test.go` (`ceiling`); Unicode `CaseFolding.txt`; `CONTEXT.md` §§1b, 9 | An eagerly materialized canonical stream is `q(haystack)` with canonical runes instead of dense IDs.  An online table/escape implementation is the same stream without storing it. |
| Native fold-set byte structures | ClickHouse MultiVolnitskyCaseInsensitiveUTF8 and Quamina, cited in `CONTEXT.md` §1d | These establish that deriving byte-level structures from case-fold data is not new.  They differ in contract or scope, but neither difference turns a direct orbit quotient into a new construction. |
| Standard dictionary matching over a relabeled alphabet | Aho--Corasick (1975), cited in `CONTEXT.md` §1d | Applying a trie/DFA/AC transition to `q(patterns)` is ordinary dictionary matching after a homomorphic relabeling.  It would also be an alternate Aho--Corasick engine, which the repository rules prohibit as a runtime escape hatch. |

## Why it is an equivalent repacking

Simple-fold equality is an equivalence relation on valid runes.  For valid
runes `a` and `b`,

```
a fold-equals b  if and only if  q(a) == q(b).
```

The map is therefore a lossless quotient for this matching predicate.  Replacing
one position's finite set of case encodings with one class token does not add a
transition, remove a transition, or encode any information unavailable to a
case-expanded automaton.  UTF-8 width changes only affect decoding and the
mapping from token index back to byte offset; they do not change the quotient
identity.  Keeping invalid bytes as singleton tokens likewise matches the
existing opaque-byte rule exactly.

For a single pattern, exact matching of `q(pattern)` in `q(haystack)` is
canonical-fold-then-search.  For multiple patterns, a shared alphabet followed
by a dictionary automaton is canonical-fold-then-dictionary-search.  A 256-byte
ASCII table, vector lookup, sparse orbit table, or a non-ASCII escape path
changes storage and cost, not that equivalence.  “Never refolds during
verification” is consequently the online/precomputed-folding tradeoff already
represented by the arena ceiling, not a new matcher construction.

## Falsification search and result

This assessment would be falsified by a construction whose runtime state
cannot be reduced to a token from the fold-equivalence quotient followed by an
ordinary literal/dictionary transition.  Examples of potentially distinct
claims would be a block transition that jointly preserves variable UTF-8 width,
leftmost byte offsets, and multiple pattern states without materializing or
serially feeding quotient tokens, together with a proof that no
case-expanded/canonical-stream automaton has the same transition relation; or
a published implementation/paper that explicitly describes this exact quotient
plan as a named construction.

The repository's prior-art review was checked before this document was added:
`CONTEXT.md` §§1b--1d explicitly catalogs case-expanded UTF-8 automata,
pre-folded streams, native fold-set structures, and multi-pattern automata.
Those sources already account for every component of the assessed plan.  The
result is that the proposed orbit-class table is classified as engineering, not
an invention.  No external browsing result is being asserted beyond the
sources cited above.

## Provenance

This contribution currently adds only this assessment.  `NOVELTY.md` was
written for this repository from the current `AGENTS.md`, `README.md`,
`CONTEXT.md`, and the cited source locations; it contains no copied
implementation code.  If implementation files are added later, each
non-trivial file will identify its authorship and source provenance here.
