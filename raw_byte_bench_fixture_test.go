package casei

import "strings"

// These benchmark fixture sizes are independent of raw-plan admission. They
// make the construction and candidate-density shapes available before the
// transition optimization so the same public benchmark can measure both arms.
const (
	rawByteBenchmarkCorpusBytes   = 2 << 20
	rawBytePublicationCorpusBytes = 5 << 20
	rawByteFreshSampleBytes       = 4 << 10
)

var rawByteCyrillicPatterns = []string{
	"Шерлок Холмс",
	"Джон Уотсон",
	"Ирен Адлер",
	"инспектор Лестрейд",
	"профессор Мориарти",
}

// rawByteFalseCandidates emits ordinary Cyrillic root units followed by an
// ASCII mismatch. Each planted start reaches exactly two shared-plan unit
// transitions: the root and the reset byte.
func rawByteFalseCandidates(period, groups int) string {
	if period < len("Дx") {
		panic("raw byte candidate period is too short")
	}
	bytes := []byte(strings.Repeat("x", period*groups))
	for at := 0; at+len("Дx") <= len(bytes); at += period {
		copy(bytes[at:], "Дx")
	}
	return string(bytes)
}

// rawByteFalseCandidatesAtLeast rounds groups up so fixtures cross a requested
// byte boundary even when the candidate period does not divide it.
func rawByteFalseCandidatesAtLeast(period, bytes int) string {
	return rawByteFalseCandidates(period, (bytes+period-1)/period)
}

// rawByteLateMatchCandidatesAtLeast keeps the same repeated false-root shape
// through the final sample window, then replaces one late slot with the first
// literal. The returned offset is the ordinary byte offset of that occurrence.
func rawByteLateMatchCandidatesAtLeast(period, bytes int) (string, int) {
	pattern := rawByteCyrillicPatterns[0]
	if period < len(pattern) {
		panic("raw byte late-match period is too short")
	}
	out := []byte(rawByteFalseCandidatesAtLeast(period, bytes))
	start := len(out) - period
	copy(out[start:], pattern)
	return string(out), start
}

// rawByteEarlyMatchCandidatesAtLeast plants one complete literal at a fixed
// early offset while retaining a long dense suffix. It guards the public Find
// call that can stop before later admission sample windows.
func rawByteEarlyMatchCandidatesAtLeast(period, bytes, start int) (string, int) {
	pattern := rawByteCyrillicPatterns[0]
	if period < len(pattern) || start < 0 || start%period != 0 {
		panic("invalid raw byte early-match placement")
	}
	out := []byte(rawByteFalseCandidatesAtLeast(period, bytes))
	if start+len(pattern) > len(out) {
		panic("raw byte early match is outside its corpus")
	}
	copy(out[start:], pattern)
	return string(out), start
}

// rawByteNearMissCandidates plants a whole literal prefix with a final Cyrillic
// mismatch. It exercises candidate confirmation without returning a match.
func rawByteNearMissCandidates(period, groups int) string {
	const nearMiss = "Шерлок Холми"
	if period < len(nearMiss) {
		panic("raw byte near-miss period is too short")
	}
	bytes := []byte(strings.Repeat("x", period*groups))
	for at := 0; at+len(nearMiss) <= len(bytes); at += period {
		copy(bytes[at:], nearMiss)
	}
	return string(bytes)
}

func rawByteNearMissCandidatesAtLeast(period, bytes int) string {
	return rawByteNearMissCandidates(period, (bytes+period-1)/period)
}

// rawBytePublicationCandidates starts with one complete literal and leaves an
// independently dense suffix. It is shared by enumeration tests that need a
// concrete accepted callback before exercising later work.
func rawBytePublicationCandidates(period int) string {
	return rawByteCyrillicPatterns[0] + rawByteFalseCandidatesAtLeast(period, rawBytePublicationCorpusBytes)
}

// rawByteMatchedCandidatesAtLeast plants non-overlapping pattern-zero matches
// at a fixed byte period for enumeration and tail tests.
func rawByteMatchedCandidatesAtLeast(period, bytes int) string {
	groups := (bytes + period - 1) / period
	out := []byte(strings.Repeat("x", period*groups))
	for at := 0; at+len(rawByteCyrillicPatterns[0]) <= len(out); at += period {
		copy(out[at:], rawByteCyrillicPatterns[0])
	}
	return string(out)
}

func rawByteLongPrefixPatterns() []string {
	prefix := strings.Repeat("Д", 100)
	return []string{prefix + "a", prefix + "b"}
}
