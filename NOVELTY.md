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

## Follow-up assessment: direct raw-byte fold transitions

### Status

**Negative assessment for raw UTF-8 byte accept masks with partial-width
matcher state.**  The proposed scan can avoid constructing a Go `rune` or a
canonical-fold token, but its states and transitions are exactly a compressed
case-expanded UTF-8 byte automaton.  It is therefore a different operational
placement of decoding work, not a distinct search construction.  Per
`AGENTS.md` §§1 and 3, no implementation or performance experiment follows
from this assessment.

### Construction assessed

Let `E(a)` be the set of UTF-8 byte strings for all members of the simple-fold
orbit of a valid pattern rune `a`.  For an opaque invalid pattern byte `b`,
let `E(b) = {b}`.  For pattern units `a₁ … aₘ`, the proposed raw-byte matcher
accepts the concatenation language

```
E(a₁) E(a₂) … E(aₘ).
```

Its proposed representation assigns each runtime state a raw-byte accept mask
and carries a state for a prefix of a multi-byte alternative already seen.  A
`k` position, for example, accepts the one-byte alternatives `k` and `K` and
the three-byte alternative `e2 84 aa` for KELVIN SIGN.  A `s` position adds
`c5 bf` for LONG S.  A sigma position has the three two-byte alternatives
`ce b2`, `ce b3`, and `ce a3`.  The partially consumed `e2`, `e2 84`, `c5`,
and `ce` prefixes are the proposed extra states.

The arbitrary-byte contract requires one more component.  A raw scanner must
not let an opaque-byte pattern `84` match the middle byte of `e2 84 aa`.
Consequently it must carry the standard finite UTF-8 lexical/boundary state,
including invalid-sequence error behavior, or an equivalent delayed-byte
mechanism.  Otherwise it fails the existing `lone continuation vs kelvin
bytes` trap.  This may avoid a call to `utf8.DecodeRuneInString`, but it is
still the UTF-8 prefix automaton embedded in the matcher state.

### Constructive reduction to the known construction

For every byte string in `E(a)`, draw a byte-labelled path from the state
before pattern position `a` to the state after it.  The nodes after each
proper prefix of a multi-byte string are precisely the partial-width states
above.  Product that graph with the UTF-8 lexical/boundary automaton, label
its terminals with pattern indices, and carry each active path's native byte
start alongside the ordinary search state.  Selecting the leftmost start and
then the lowest terminal pattern index gives exactly the repository contract,
including opaque bytes.

A per-state byte mask is merely a packed encoding of the outgoing labelled
edges in that graph.  Expanding a mask into one edge per accepted byte
recovers the case-expanded byte graph; packing equal edge sets into masks
recovers the proposal.  A bitset of simultaneously active states is likewise
the standard subset representation of that graph.  For a pattern set, union
all pattern graphs and use the same construction; the multi-pattern state is
still a state or subset of the same expanded byte automaton.

Thus width changes add ordinary prefix paths, not a new transition relation.
No normalized stream or `q` token has to be materialized for this equivalence:
the omission changes storage and scheduling only.  A block implementation
would precompute several transitions of this same graph.  It would remain an
acceleration of the graph unless it supplies a transition not obtainable by
mask packing and byte-edge expansion.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Case-expanded literal and UTF-8 automata | UTS #18; RE2; rust-regex / regex-automata; `CONTEXT.md` §1b | Each member of `E(a)` is an ordinary UTF-8 alternative.  The path expansion above is their literal construction. |
| Native byte-level fold-set structures | ClickHouse `MultiVolnitskyCaseInsensitiveUTF8` and Quamina; `CONTEXT.md` §1d | These already derive byte structures from fold alternatives.  The proposal changes neither their byte-language basis nor the meaning of an intermediate encoding-prefix state. |
| Masked SIMD literal engines | Teddy and FDR / Hyperscan; `CONTEXT.md` §§1d and 3 | Byte or nibble masks are a compact representation of accepted outgoing alternatives; they do not create a different accepted transition relation. |
| Canonical-fold quotient assessment above | This file; `CONTEXT.md` §1b | The earlier rejected plan materializes a quotient token.  This plan does not materialize one, but directly expands the same fold sets into UTF-8 byte paths, which is the other already-catalogued representation. |

### Falsification search and result

The negative would be falsified by a precise raw-byte state whose update and
outputs cannot be obtained from the expanded byte paths plus the UTF-8
boundary state — not merely by avoiding a `rune` allocation, packing masks
differently, or precomputing a block transition.  In particular, it would
need a proof that expanding every mask into byte edges fails to preserve an
accepted match, its byte start, or its selected pattern index.

Applying that test to the proposed state gives the opposite result: every
partial multi-byte-progress state is a proper-prefix node of an `E(a)` path,
every accept mask expands to its outgoing byte edges, and the required
boundary state is the ordinary UTF-8 lexical product.  KELVIN SIGN, LONG S,
and the sigma trio exercise the different-width and shared-prefix paths rather
than breaking the reduction.  The proposed mechanism is consequently a
relabeled/compressed case-expanded byte automaton, the closed outcome named in
the case hypothesis.

### Decision

This is a documented negative finding, not a candidate optimization.  The
repository must not add a known-art implementation merely to benchmark its
decode cost; doing so would violate the invention and negative-result rules in
`AGENTS.md`.

## Follow-up assessment: fixed-width lossy projection with survivor verification

### Status

**Negative assessment for the proposed `index_fold` projection scan.**  The
published `casefold` crate already describes this exact one-byte-per-character
projection as a lossy case-insensitive index/hash key and explicitly prescribes
using it as a candidate filter followed by verification against the original
text.  Generating that same key stream online, scanning it in fixed-width SIMD
blocks, and verifying survivors changes when the key is stored and compared;
it does not identify a new search state or transition.

### Construction assessed

Let `p` be `casefold::index_fold_char` on each valid Unicode character: ASCII
is simple-folded to an ASCII byte and every non-ASCII character becomes
`0x80 | (simple_fold(r) & 0x7f)`.  A Go implementation would additionally
need an opaque-unit rule for invalid UTF-8 so that a candidate cannot begin in
the middle of a valid encoding.  That contract detail is necessary for this
repository, but does not change the projection/filter construction.

The assessed plan projects the needle once, emits `p` for haystack units in
source order, and searches the projected sequence for the projected needle.
A projected equality is only a survivor: it recovers its original byte start
with either a running unit-to-byte cursor or a replay/offset map, then calls a
true simple-fold verifier.  Fixed-stride SIMD scanning is only a batched way
to evaluate the projected equality.  The projection has no false negatives
for a fold-equal rendering, but its seven-bit non-ASCII payload admits false
positives and the verifier decides the match.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| `casefold::index_fold` | GitHub [`rust-gems` `casefold` v0.1.0 `index_fold.rs`](https://github.com/github/rust-gems/blob/d08a649afb47f1ec303f30ec3d062444291b5ec3/crates/casefold/src/index_fold.rs) and [crate README](https://github.com/github/rust-gems/blob/d08a649afb47f1ec303f30ec3d062444291b5ec3/crates/casefold/README.md#single-byte-index-fold) | The implementation defines the same ASCII/high-bit projection.  Its docs call the result a fixed-width key for indexing or hashing with acceptable collisions; the README says to use it as a candidate filter with no false negatives and verify exact hits against original text. |
| Case-insensitive n-gram indexing | The same `casefold` README, “Why one byte per character?” | A stored projected document and an online projected scan differ only in persistence and lookup scheduling.  Both compare the same projected k-character key and verify original-text candidates. |
| SIMD candidate/confirm literal engines | `CONTEXT.md` §§1d and 3--5; Teddy/FDR/Snort sources listed in `CONTEXT.md` §10 | The two-stage SIMD-candidate-plus-verifier shape is explicitly pre-conceded known art.  Replacing a byte/nibble fingerprint with this published character projection does not create a new confirmation transition. |
| Source-position recovery after a reduced-space search | `CONTEXT.md` §1b (alphabet sampling with position mapping; rust/regex reverse re-scan) | Carrying byte offsets beside projected units is a position map; replaying after a rare survivor is re-scan recovery.  They select the same source start for the same projected hit. |

### Why the online scan is an equivalent repacking

For a valid haystack decoded into units `h₀, h₁, …`, materializing
`p(h₀)p(h₁)…` and searching that byte string produces exactly the same survivor
unit indexes as emitting `p` in a streaming loop and comparing blocks as they
arrive.  Keeping a byte cursor merely decorates each emitted unit with its
source coordinate; replaying derives that coordinate later.  Neither changes
which projected windows equal `p(needle)`.

At a survivor, the proposed true-fold verifier is the only operation that
separates a collision from a match.  Thus the complete accepted predicate is
“projected-key equality, then the existing exact predicate,” which is the
filter/verify pipeline the crate documents for its n-gram key.  SIMD width,
projected-block layout, a rare-survivor replay threshold, and collision density
can affect cost, but not the representation or accepted language.  In
particular, offset recovery is not a new block transition: it is bookkeeping
outside the projected comparison and verifier.

This also cannot reopen either closed assessment above.  Unlike the lossless
fold-orbit quotient and raw-byte fold automaton, this plan deliberately loses
information before searching; its only way back to correct semantics is the
known confirmation stage, not a new recognizer for the fold language.

### Falsification search and result

This negative would be falsified by a source showing that `index_fold` is not
usable as a lossy candidate filter followed by original-text verification, or
by a precise online state/update whose outputs cannot be reproduced from the
projected stream, an ordinary projected-string search, a source-position map
(or replay), and the same verifier.  A faster SIMD implementation, a lower
collision rate on a corpus, or a different replay threshold would not falsify
that reduction.

The `casefold` source and README were read at commit
`d08a649afb47f1ec303f30ec3d062444291b5ec3`; its `Cargo.toml` identifies the
crate as version `0.1.0`.  They give the opposite of the first falsifier: the
projection is documented as a collision-tolerant index/hash key, and the README
explicitly names candidate filtering followed by exact verification.  The
repository's existing survey independently places two-stage candidate/verify
engines and reduced-space offset recovery in known art.  No distinct state or
transition remained after that comparison.

### Decision

This is a documented negative finding, not a candidate optimization.  Per
`AGENTS.md`, the repository must not add a Go port, SIMD scan, or collision
benchmark merely to measure a published filter/verify construction.  The
requested Cyrillic and hazard collision-density sweep is therefore not
performed: it could characterize the known technique's cost, but cannot make
it an invention.  No performance claim is made.

## Provenance

This contribution adds only the novelty assessments in this file.  The
original assessment, the raw-byte follow-up, and the fixed-width projection
assessment were written for this repository from the current `AGENTS.md`,
`README.md`, `CONTEXT.md`, source and test files, and the cited source
locations; they contain no copied implementation code and make no external
performance claim.  The last assessment also records the inspected upstream
crate commit and version.  If implementation files are added later, each
non-trivial file will identify its authorship and source provenance here.
