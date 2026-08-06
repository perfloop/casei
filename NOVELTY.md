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

## Follow-up assessment: raw-byte rolling fingerprint with run-membership correction

### Status

**Negative assessment for a fold-invariant raw-byte rolling fingerprint.**
The required algebra does not yield a stable folded window from a raw byte
window plus a cheap run summary.  In the width-preserving ASCII subset, the
correction is a second, position-weighted hash of case-run membership and must
be updated for every entering and leaving byte.  For general Unicode simple
folding, width-changing members make raw and folded window boundaries
variable; exact updates additionally need each unit's folded length, folded
byte contribution, and UTF-8 boundary status.  Keeping that state only in
registers avoids an allocation, but it is still online folding inside the
hash, not recovery from a raw-byte fingerprint without folding.

This is an algebraic negative result.  No SIMD implementation, benchmark, or
performance claim follows from it.

### Window semantics come before a rolling hash

Let `F` map each valid UTF-8 unit to one deterministic simple-fold
representative and leave an invalid byte as its own opaque one-byte unit.  The
particular representative is immaterial to the argument.  The `casefold`
run-table direction recorded in `CONTEXT.md` §1e gives concrete examples:

```
E2 84 AA       (U+212A KELVIN SIGN)  ->  6B       (k)
C8 BA          (U+023A)              ->  E2 B1 A5 (U+2C65)
```

The repository's `orbitMin` reference can choose the opposite representative
for an orbit, but the two members still have unequal input byte widths.  Thus
for a folded one-character needle such as `k`, valid source windows include
`6B`, `4B`, and `E2 84 AA`.  No fixed raw-byte window length contains all
three.  Conversely, the U+023A example shows that a fold can grow from two
source bytes to three folded bytes.

A correct search window must therefore be a sequence of decoded units whose
*folded* byte length equals the needle's folded length, with a source byte
start retained for that sequence.  Moving either endpoint requires knowing
unit boundaries and each unit's folded length.  Choosing a window by raw byte
length instead misses one of the legal renderings; choosing it by character
count is a decoded-unit stream rather than a raw-byte window.

### The favorable fixed-width algebra still needs a second classified stream

Use the usual order-sensitive linear polynomial hash over bytes, with base
`B` (byte literals below are hexadecimal):

```
H(b0 ... b(L-1)) = sum(bj * B^(L-1-j))
H(x || y) = H(x) * B^len(y) + H(y)
```

Grant the most favorable case: an ASCII window has no width changes and the
only transform is `A` through `Z` gaining `0x20`.  Put
`uj = 1` when byte `bj` is an uppercase ASCII letter and zero otherwise.  Then

```
H(F(w)) = H(w) + 0x20 * U(w)
U(w)    = sum(uj * B^(L-1-j))
```

`U`, not the number of uppercase bytes, is the required correction.  For
example, `Aa` and `aA` have the same uppercase count but corrections
`0x20 * B` and `0x20`.  The rolling update for `U` is the same kind of update
as the raw hash:

```
U(i+1) = B * (U(i) - ui * B^(L-1)) + u(i+L)
```

So this restricted case is recoverable only by maintaining a second rolling
hash of a separately classified membership stream.  Each entering byte must
be tested for membership (and each leaving byte's membership must be retained
or recomputed).  A base of one would reduce the correction to a count, but it
also discards byte order and is not an equality fingerprint for substring
search.  The `casefold` run table can compress the membership lookup; it
cannot remove the position-weighted membership state.

For non-ASCII units, the same equation needs a run-specific output-byte
contribution instead of `0x20`, and membership is not byte-local.  A valid
UTF-8 unit must first be recognized and assigned to its run before that
contribution is known.  This is the folding/classification work that the
hypothesis was required to avoid, merely with the transformed bytes fed to a
hash accumulator instead of being stored.

### Width changes break raw-window recovery

For source units `u0 ... u(m-1)`, the desired hash is

```
H(F(u0 ... u(m-1))) =
  sum(H(F(ui)) * B^(sum(len(F(ut)) for t > i)))
```

The raw hash uses `len(ut)` in those exponents instead.  A width-changing
unit changes the weight of every earlier unit, so a per-run packed-word delta
cannot be added to `H(raw)` at a fixed position.  With the Kelvin mapping,
for example,

```
H(61 E2 84 AA) = 61*B^3 + E2*B^2 + 84*B + AA
H(F(61 E2 84 AA)) = H(61 6B) = 61*B + 6B
```

The `61` term changes exponent because the following source unit shrank by
two bytes.  U+023A supplies the opposite direction:

```
H(C8 BA) = C8*B + BA
H(F(C8 BA)) = E2*B^2 + B1*B + A5
```

Knowing only a total width delta or a run count cannot repair these changing
exponents.  It would require the folded-length prefix/suffix position of each
unit and its folded-byte hash.  A state that stores and combines those values
is directly maintaining `H(F(window))`; its update applies the fold transition
for every unit.  It is not a raw-hash correction that escapes folding.

Opaque bytes add a separate necessary state.  `84` alone is an invalid opaque
unit and folds to `84`, whereas the same byte in `E2 84 AA` is a continuation
inside a valid Kelvin-sign unit that folds to `6B`.  A byte-window hash cannot
let a candidate begin at that continuation.  Correct membership therefore
also requires the UTF-8 lexical/boundary state exercised by the existing
`lone continuation vs kelvin bytes` trap in `casei_test.go`.

### A fixed character-lane hash is not raw-byte recovery

There is a valid escape from the *fixed raw-byte window*: pad each decoded
unit to a fixed-width lane, use the run table's delta on that lane, and roll a
hash over the needle's number of units.  Width changes no longer move lane
positions.  But this state must determine every unit's UTF-8 boundary,
opaque-versus-valid status, packed value, and run delta before it can update
the lane hash.  It hashes a folded unit sequence in a different fixed-width
encoding, not `H` of a raw byte window plus an aggregate correction.

Avoiding storage for those lanes does not alter that distinction.  The update
is still an online fold transition for every unit, followed by a hash update;
source offsets must still be carried beside the unit window.  This is the
fold-inside-the-hash fallback described below, not a falsifier of the requested
raw-byte algebra.

### Relation to existing work

`CONTEXT.md` §1e is the source for the run-table arithmetic and the two
width-changing examples above.  Its packed-`u32` relation can supply a
per-unit output value, but it does not make concatenation positions
width-invariant.  `casei.go`, `casei_test.go`, and `CONTEXT.md` §1b specify the
required source-boundary and opaque-byte behavior.

`CONTEXT.md` §6 already lists the viable fallback operation: fold each input
unit in the hashing loop and hash in the folded domain.  Keeping only the hash
rather than materializing the folded bytes does not change that operation.
This conclusion does not reopen the closed orbit-quotient, raw-byte automaton,
or `index_fold` assessments above: it rejects the proposed raw-hash algebra
before relying on any projected-stream reduction.

### Falsification search and result

This negative would be falsified by a precise order-sensitive linear hash and
rolling update that produces the exact folded-window hash for all legal source
windows while avoiding all three requirements: position-weighted run
membership, folded-length/output-coordinate state at width changes, and UTF-8
unit/boundary classification.  It would need to handle `k`/`K`/KELVIN SIGN,
U+023A/U+2C65, and an isolated `0x84` without changing the returned source
byte position.  Merely vectorizing membership tests, storing an implicit
folded hash, or verifying hash collisions after a fold-aware update would not
falsify the result; each retains the work identified by the equations.

Applying that test gives the negative result.  The fixed-width formula demands
`U`; width changes demand output-coordinate state; and invalid-byte opacity
demands UTF-8 lexical state.  No raw-byte rolling fingerprint satisfying the
hypothesis remains to implement.

### Decision

This is a documented negative finding, not a candidate optimization.  Per
`AGENTS.md`, the repository must not add a SIMD/hash implementation or run a
benchmark merely to measure an operation that either fails the required window
semantics or reduces to folding inside a rolling hash.  No performance claim
is made.

## Follow-up assessment: prefix-invariant anchor with arithmetic start recovery

### Status

**Negative assessment for a width-invariant-prefix anchor.**  This would move
work out of the byte scan: choose a pattern position after a fixed-width
prefix, scan for that position's byte forms, subtract the prefix width to get
a proposed start, and confirm the whole pattern.  It is not a new matcher
state.  It is a specialization of the published safe-slice-at-an-offset plus
head/tail confirmation pipeline, combined with the already-rejected raw-byte
fold alternatives.  The subtraction partially evaluates known start recovery;
it does not add a transition or remove the required confirmation.

No implementation or benchmark follows from this assessment.

### Construction assessed

Decode the needle once into simple-fold units.  Call a valid unit
*width-invariant* when every member of its `unicode.SimpleFold` orbit has the
same UTF-8 byte width; an opaque invalid byte is a singleton width-one unit.
Choose an anchor unit `a_j` after a width-invariant prefix
`a_0 ... a_{j-1}`, and let

```
L = sum(encodedWidth(a_i), 0 <= i < j).
```

A scan searches the haystack bytes for any member of the anchor's finite UTF-8
form set `E(a_j)`.  For a hit whose anchor bytes begin at `t`, it proposes
`start = t - L`, then validates the prefix, anchor, and suffix under simple
folding and returns the leftmost validated start.  An implementation may use
SIMD to find `E(a_j)` and may pre-expand a small set of anchor forms.  It must
still reject a byte hit in the middle of a valid UTF-8 encoding, and must treat
an invalid byte as an opaque unit.

For example, a prefix ending before `k` has a known byte width when none of
its units has a cross-width fold mate.  The anchor can then admit `k`, `K`, and
`E2 84 AA` (KELVIN SIGN); a hit at `t` implies only a *candidate* start
`t - L`.  It does not prove that the preceding bytes render the prefix, nor
that `t` is a legal unit boundary.  Those facts remain confirmation work.

The intended operational benefit is real but narrow: the streaming scan need
not call `utf8.DecodeRuneInString` or walk a `SimpleFold` orbit at every byte
position.  The novelty question is whether the resulting candidate state is
more than a known safe window and verifier.

### Current-art check

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Safe folded slice at an arbitrary needle offset, SIMD probes, and head/tail verification | StringZilla current `main`, inspected at `657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0`: [`utf8_uncased.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased.h), [`serial.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/serial.h), [`haswell.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/haswell.h) | Its metadata builder considers every rune-boundary slice, records `offset_in_unfolded` and `length_in_unfolded`, and its SIMD driver scans that selected safe slice.  `sz_utf8_uncased_verify_match_` reverse-verifies the head, forward-verifies the tail, and returns the candidate-window offset minus the verified head width.  The assessed subtraction is this general recovery specialized to a head whose accepted rendering width is `L`. |
| Width-preserving case-insensitive prefix acceleration | PCRE2 [`pcre2_jit_compile.c`](https://raw.githubusercontent.com/PCRE2Project/pcre2/master/src/pcre2_jit_compile.c), `scan_prefix` | For a UTF-8 caseless character, `scan_prefix` stops growing its prefix when `ord2utf(othercase) != len`.  Thus the width-invariance condition is an established acceleration boundary.  Moving the probe to the next character changes the candidate coordinate, not the finite byte alternatives or confirmation relation. |
| Raw UTF-8 form paths and boundary state | “Follow-up assessment: direct raw-byte fold transitions” above | Expanding `E(a_j)` into one-byte and multi-byte alternatives, plus rejecting mid-rune starts, is exactly the byte-path construction already reduced here to a case-expanded UTF-8 automaton. |
| SIMD candidate plus full confirmation | StringZilla sources above; Teddy/FDR/Snort sources cataloged in `CONTEXT.md` §§1b--1d and 3 | A fixed byte displacement before confirmation is ordinary candidate bookkeeping.  It does not make a filter/verify pipeline a new recognizer. |

StringZilla has full-fold rather than this repository's simple-fold contract,
so it is not a drop-in implementation and is not cited as one.  That semantic
difference does not make this state new: safe-slice selection at a pattern
offset, a SIMD candidate scan, and recovery/verification around that slice are
already used for the harder expansion and variable-width setting.  Restricting
those mechanics to simple-fold orbits removes cases; it does not introduce a
new transition.

### Reduction

Take each anchor form in `E(a_j)` and draw its ordinary byte-labelled path.
The scan's output is a pair `(t, form)`.  Because the prefix is
width-invariant, translating that pair to `t - L` is a fixed coordinate change;
it carries no information about whether the prefix actually matches.  The
validator decides precisely that remaining predicate, along with UTF-8
boundary validity, anchor equality, and the suffix.  Replacing an SIMD probe
with a scan of the expanded byte paths produces the same candidate pairs;
replacing `t - L` with StringZilla's general reverse-head recovery produces
the same start on every accepted prefix, since its consumed head length is
then exactly `L`.

Skipping confirmation is unsound: any occurrence of the anchor form can be
preceded by a nonmatching prefix, and raw matching can find a continuation-byte
location.  Pre-expanding and comparing the prefix instead merely moves the
same finite byte paths into the candidate filter.  It is the closed raw-byte
transition construction, not a new state.  A multi-pattern version only adds
`(pattern, anchor, L)` labels to those candidates; merging them in a
dictionary state is ordinary case-expanded dictionary matching, while keeping
per-pattern confirmations is the known candidate/confirm shape and does not
meet the repository's one-engine rule.

Consequently, the apparent distinction — no reverse walk after a successful
anchor probe — is a partial evaluation of a known verifier under a static
width fact.  It can save instructions on a surviving candidate, but it cannot
change the accepted language, a state update, or the offset-selection rule.
Under `AGENTS.md` §1, that is an optimization of published art, not the
required invention.

### Falsification search and result

This negative would be falsified by a block transition that uses the
width-invariant prefix to accept or advance anchored alternatives while
preserving simple-fold equality, source byte boundaries, leftmost order, and
multi-pattern ties, but cannot be expanded into finite anchor byte paths plus
a fixed coordinate translation and ordinary confirmation.  Merely eliminating
a reverse iterator, choosing a different anchor, vectorizing the form probe,
or encoding the prefix forms in a table would not suffice.

The upstream source check gives the opposite result.  StringZilla's current
metadata records an arbitrary safe-slice offset and its verifier already
recovers the start by validating the preceding head; under the invariant the
recovered width is the constant `L`.  PCRE2 independently uses the same
width-equality test to delimit its caseless fast-forward prefix.  Applying the
byte-path expansion above leaves no state or output that cannot be reproduced
by those known ingredients.  The proposed construction is therefore rejected
before performance work.

### Decision

Do not add a prefix-anchor implementation, AVX2 kernel, portable fallback,
or benchmark sweep for this construction.  It would be a specialization and
retuning of known safe-window/filter-and-verify machinery, contrary to
`AGENTS.md` §§1--3.  A future attempt would need the falsifying block
transition above, not a different anchor scoring rule or a faster verifier.

## Provenance

This contribution adds only the novelty assessments in this file.  The
original assessment, the raw-byte follow-up, the fixed-width projection
assessment, the raw-byte rolling-fingerprint assessment, and the
prefix-invariant-anchor assessment were written for this repository from the
current `AGENTS.md`, `README.md`, `CONTEXT.md`, source and test files, and the
cited source locations; they contain no copied implementation code and make
no external performance claim.  The projection assessment records the
inspected upstream crate commit and version; the prefix-anchor assessment
records the current StringZilla source revision inspected through `git
ls-remote`.  If implementation files are added later, each non-trivial file
will identify its authorship and source provenance here.
