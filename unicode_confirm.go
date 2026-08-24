package casei

import "unicode/utf8"

const (
	unicodePairConfirmMaxParts     = 20
	unicodePairConfirmPartSize     = 10
	unicodePairConfirmSkippedParts = 2

	unicodePairConfirmSkippedAt    = (unicodePairConfirmMaxParts - unicodePairConfirmSkippedParts) * unicodePairConfirmPartSize
	unicodePairConfirmMaxLengthAt  = unicodePairConfirmMaxParts * unicodePairConfirmPartSize
	unicodePairConfirmAnchorAt     = unicodePairConfirmMaxLengthAt + 1
	unicodePairConfirmNAt          = unicodePairConfirmMaxLengthAt + 2
	unicodePairConfirmValidAt      = unicodePairConfirmMaxLengthAt + 3
	unicodePairConfirmPackedSize   = unicodePairConfirmMaxLengthAt + 4
	unicodePairConfirmMinAt        = unicodePairConfirmPackedSize
	unicodePairConfirmVariableSize = unicodePairConfirmPackedSize + 1
	unicodePairConfirmVariableFlag = 2
)

// unicodePairConfirm is stable assembly input for one bounded literal. The
// 204-byte fixed layout stores up to three one- or two-byte values plus their
// source offset in each ten-byte part. A 205-byte variable layout instead
// stores up to three three-byte-padded raw forms per part and adds the minimum
// possible match width; its cursor advances by the form that actually matched.
// Both layouts finish with maximum width, anchor offset, part count, and flags.
type unicodePairConfirm string

func (confirm unicodePairConfirm) valid() bool {
	if (len(confirm) != unicodePairConfirmPackedSize && len(confirm) != unicodePairConfirmVariableSize) ||
		confirm[unicodePairConfirmValidAt]&1 == 0 ||
		confirm[unicodePairConfirmMaxLengthAt] == 0 || confirm[unicodePairConfirmAnchorAt] >= confirm[unicodePairConfirmMaxLengthAt] {
		return false
	}
	if confirm.variable() {
		parts := int(confirm[unicodePairConfirmNAt])
		return len(confirm) == unicodePairConfirmVariableSize && parts > 0 && parts <= unicodePairConfirmMaxParts &&
			confirm[unicodePairConfirmMinAt] > 0 && confirm[unicodePairConfirmMinAt] <= confirm[unicodePairConfirmMaxLengthAt]
	}
	if len(confirm) != unicodePairConfirmPackedSize {
		return false
	}
	skipped := confirm.skippedN()
	parts := int(confirm[unicodePairConfirmNAt])
	return (skipped == 0 || skipped == unicodePairConfirmSkippedParts) && parts+skipped <= unicodePairConfirmMaxParts &&
		(parts != 0 || skipped != 0)
}

func (confirm unicodePairConfirm) variable() bool {
	return len(confirm) > unicodePairConfirmValidAt && confirm[unicodePairConfirmValidAt]&unicodePairConfirmVariableFlag != 0
}

func (confirm unicodePairConfirm) maxLength() int {
	return int(confirm[unicodePairConfirmMaxLengthAt])
}

func (confirm unicodePairConfirm) minLength() int {
	if confirm.variable() && len(confirm) == unicodePairConfirmVariableSize {
		return int(confirm[unicodePairConfirmMinAt])
	}
	return confirm.maxLength()
}

func (confirm unicodePairConfirm) anchorAt() int {
	return int(confirm[unicodePairConfirmAnchorAt])
}

func (confirm unicodePairConfirm) skippedN() int {
	return int(confirm[unicodePairConfirmValidAt] >> 1)
}

// makeUnicodePairConfirm moves pair-pair's two raw tokens to trailing slots
// when confirmAt is supplied. The vector transition proves those slots from
// its low-six-bit tables plus UTF-8 byte classes; matchWidthAt still checks
// every slot and remains a complete raw-token oracle for scalar replay.
func makeUnicodePairConfirm(pattern string, anchorAt int, confirmAt ...int) unicodePairConfirm {
	if anchorAt < 0 || anchorAt > 255 || len(pattern) > 255 || len(confirmAt) > 1 {
		return ""
	}

	skippedAt := [unicodePairConfirmSkippedParts]int{}
	skipN := 0
	if len(confirmAt) != 0 {
		if confirmAt[0] < 0 || confirmAt[0] > 255 || confirmAt[0] == anchorAt {
			return ""
		}
		skippedAt[0], skippedAt[1] = anchorAt, confirmAt[0]
		skipN = unicodePairConfirmSkippedParts
	}

	forms, _ := patternRawForms(pattern)
	packed := make([]byte, unicodePairConfirmPackedSize)
	at, parts, skipped := 0, 0, 0
	for _, unit := range forms {
		r, size := utf8.DecodeRuneInString(pattern[at:])
		if r == utf8.RuneError && size == 1 || len(unit) == 0 || len(unit) > 3 {
			return ""
		}
		width := len(unit[0])
		if width < 1 || width > 2 || width != size {
			return ""
		}

		var packedPart [unicodePairConfirmPartSize]byte
		packedPart[6], packedPart[7], packedPart[8] = uint8(at), uint8(width), uint8(len(unit))
		for i, form := range unit {
			if len(form) != width {
				return ""
			}
			value := uint16(form[0])
			if width == 2 {
				value |= uint16(form[1]) << 8
			}
			valueAt := i * 2
			packedPart[valueAt], packedPart[valueAt+1] = uint8(value), uint8(value>>8)
		}

		isSkipped := false
		for i := range skipN {
			if at == skippedAt[i] {
				isSkipped = true
				break
			}
		}
		if isSkipped {
			if skipped == skipN {
				return ""
			}
			partAt := unicodePairConfirmSkippedAt + skipped*unicodePairConfirmPartSize
			copy(packed[partAt:], packedPart[:])
			skipped++
		} else {
			if parts == unicodePairConfirmMaxParts-skipN {
				return ""
			}
			partAt := parts * unicodePairConfirmPartSize
			copy(packed[partAt:], packedPart[:])
			parts++
		}
		at += size
	}
	if at != len(pattern) || parts+skipped == 0 || skipped != skipN {
		return ""
	}
	packed[unicodePairConfirmMaxLengthAt] = uint8(at)
	packed[unicodePairConfirmAnchorAt] = uint8(anchorAt)
	packed[unicodePairConfirmNAt] = uint8(parts)
	packed[unicodePairConfirmValidAt] = 1 | uint8(skipped<<1)
	return unicodePairConfirm(string(packed))
}

// makeUnicodePairVariableConfirm records the same literal as a short sequence
// of raw forms. Unlike the fixed-offset representation above, its confirmation
// cursor advances by the width of the form that actually matched. The pair-pair
// screen still fixes the start: makeUnicodePairAnchor only records anchors
// before the first width-changing fold orbit.
func makeUnicodePairVariableConfirm(pattern string, anchorAt int) unicodePairConfirm {
	if anchorAt < 0 || anchorAt > 255 || !utf8.ValidString(pattern) {
		return ""
	}
	forms, _ := patternRawForms(pattern)
	if len(forms) == 0 || len(forms) > unicodePairConfirmMaxParts {
		return ""
	}
	packed := make([]byte, unicodePairConfirmVariableSize)
	minLength, maxLength := 0, 0
	for part, unit := range forms {
		if len(unit) == 0 || len(unit) > 3 {
			return ""
		}
		minWidth, maxWidth := utf8.UTFMax+1, 0
		partAt := part * unicodePairConfirmPartSize
		for formIndex, form := range unit {
			if len(form) == 0 || len(form) > 3 {
				return ""
			}
			copy(packed[partAt+formIndex*3:], form)
			if len(form) < minWidth {
				minWidth = len(form)
			}
			if len(form) > maxWidth {
				maxWidth = len(form)
			}
		}
		packed[partAt+9] = byte(len(unit))
		minLength += minWidth
		maxLength += maxWidth
	}
	if minLength == maxLength || maxLength > 255 {
		return ""
	}
	packed[unicodePairConfirmMaxLengthAt] = byte(maxLength)
	packed[unicodePairConfirmAnchorAt] = byte(anchorAt)
	packed[unicodePairConfirmNAt] = byte(len(forms))
	packed[unicodePairConfirmValidAt] = 1 | unicodePairConfirmVariableFlag
	packed[unicodePairConfirmMinAt] = byte(minLength)
	return unicodePairConfirm(string(packed))
}

func (confirm unicodePairConfirm) matchesPartAt(haystack string, at, partAt int) bool {
	value := uint16(haystack[at+int(confirm[partAt+6])])
	if confirm[partAt+7] == 2 {
		value |= uint16(haystack[at+int(confirm[partAt+6])+1]) << 8
	}
	if value == uint16(confirm[partAt])|uint16(confirm[partAt+1])<<8 {
		return true
	}
	if confirm[partAt+8] >= 2 && value == uint16(confirm[partAt+2])|uint16(confirm[partAt+3])<<8 {
		return true
	}
	return confirm[partAt+8] >= 3 && value == uint16(confirm[partAt+4])|uint16(confirm[partAt+5])<<8
}

func (confirm unicodePairConfirm) matchWidthAt(haystack string, at int) (int, bool) {
	if !confirm.valid() || at < 0 || len(haystack)-at < confirm.minLength() {
		return 0, false
	}
	start := at
	if confirm.variable() {
		for part := 0; part < int(confirm[unicodePairConfirmNAt]); part++ {
			partAt := part * unicodePairConfirmPartSize
			matched := false
			for form := 0; form < int(confirm[partAt+9]); form++ {
				formAt := partAt + form*3
				width := 1
				switch first := confirm[formAt]; {
				case first >= 0xe0:
					width = 3
				case first >= 0xc0:
					width = 2
				}
				if len(haystack)-at < width {
					continue
				}
				equal := true
				for i := 0; i < width; i++ {
					equal = equal && haystack[at+i] == confirm[formAt+i]
				}
				if equal {
					at += width
					matched = true
					break
				}
			}
			if !matched {
				return 0, false
			}
		}
		return at - start, true
	}
	if len(haystack)-at < confirm.maxLength() {
		return 0, false
	}
	for part := range int(confirm[unicodePairConfirmNAt]) {
		if !confirm.matchesPartAt(haystack, at, part*unicodePairConfirmPartSize) {
			return 0, false
		}
	}
	for skipped := range confirm.skippedN() {
		partAt := unicodePairConfirmSkippedAt + skipped*unicodePairConfirmPartSize
		if !confirm.matchesPartAt(haystack, at, partAt) {
			return 0, false
		}
	}
	return confirm.maxLength(), true
}
