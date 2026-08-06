//go:build pcre2

// PCRE2 is the first real entrant on the UTF-8 tier. Until it landed, every
// UTF-8 row here ran against Go's regexp -- a scalar NFA -- and a win there
// said only "faster than the floor". PCRE2 with JIT is a serious caseless
// matcher that outside readers know, so a UTF-8 row that beats it is a result
// about the field rather than about the floor.
//
// Semantics: PCRE2_CASELESS in UTF mode matches on simple case folding, the
// same equivalence casei implements, so no fold adapter is needed. Alternation
// is leftmost-first in pattern order, which is the arena's leftmost/lowest-ID
// rule when the alternation is built in pattern order.
//
// Compile and JIT cost is paid once per pattern set, outside the timed region,
// exactly as casei's plan compilation is. Per-call cgo transition cost stays
// inside it, and is charged to PCRE2.
package arena

/*
#cgo pkg-config: libpcre2-8
#define PCRE2_CODE_UNIT_WIDTH 8
#include <pcre2.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

// PCRE2Available reports whether this build has the PCRE2 entrant wired in.
const PCRE2Available = true

// PCRE2 is a compiled, JIT-ready caseless matcher over one pattern set.
type PCRE2 struct {
	code  *C.pcre2_code_8
	md    *C.pcre2_match_data_8
	jit   bool
	alt   string
	pinCh []byte
}

// quoteLiteral renders one literal for a PCRE2 alternation. \Q...\E is exact
// for any byte sequence that does not itself contain \E; the split handles the
// one case that does, so an adversarial needle cannot silently become a
// metacharacter.
func quoteLiteral(s string) string {
	if !strings.Contains(s, `\E`) {
		return `\Q` + s + `\E`
	}
	var b strings.Builder
	for i, part := range strings.Split(s, `\E`) {
		if i > 0 {
			b.WriteString(`\\E`)
		}
		b.WriteString(`\Q`)
		b.WriteString(part)
		b.WriteString(`\E`)
	}
	return b.String()
}

// NewPCRE2 compiles patterns into one caseless UTF-8 alternation and JITs it.
// Compilation is setup, not measurement: it happens outside the timed region.
func NewPCRE2(patterns []string) (*PCRE2, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("pcre2: empty pattern set")
	}
	quoted := make([]string, len(patterns))
	for i, p := range patterns {
		quoted[i] = quoteLiteral(p)
	}
	alt := strings.Join(quoted, "|")

	cpat := C.CString(alt)
	defer C.free(unsafe.Pointer(cpat))

	var errcode C.int
	var erroff C.PCRE2_SIZE
	code := C.pcre2_compile_8(
		(C.PCRE2_SPTR8)(unsafe.Pointer(cpat)),
		C.PCRE2_SIZE(len(alt)),
		C.PCRE2_CASELESS|C.PCRE2_UTF,
		&errcode, &erroff, nil,
	)
	if code == nil {
		buf := make([]byte, 256)
		C.pcre2_get_error_message_8(errcode, (*C.PCRE2_UCHAR8)(unsafe.Pointer(&buf[0])), C.PCRE2_SIZE(len(buf)))
		return nil, fmt.Errorf("pcre2: compile at %d: %s", int(erroff), strings.TrimRight(string(buf), "\x00"))
	}

	p := &PCRE2{code: code, alt: alt}
	// JIT is the point of including PCRE2; an interpreted run would understate
	// the field and flatter casei.
	p.jit = C.pcre2_jit_compile_8(code, C.PCRE2_JIT_COMPLETE) == 0
	p.md = C.pcre2_match_data_create_from_pattern_8(code, nil)
	if p.md == nil {
		C.pcre2_code_free_8(code)
		return nil, fmt.Errorf("pcre2: match data allocation failed")
	}
	runtime.SetFinalizer(p, func(x *PCRE2) { x.Close() })
	return p, nil
}

// JIT reports whether the JIT compiler accepted the pattern set.
func (p *PCRE2) JIT() bool { return p.jit }

// FirstIndex returns the leftmost match offset in bytes, or -1.
func (p *PCRE2) FirstIndex(haystack string) int {
	if len(haystack) == 0 {
		// A zero-length subject still needs a valid pointer for PCRE2.
		return p.match(unsafe.Pointer(&emptySubject[0]), 0)
	}
	return p.match(unsafe.Pointer(unsafe.StringData(haystack)), len(haystack))
}

var emptySubject = [1]byte{}

func (p *PCRE2) match(ptr unsafe.Pointer, n int) int {
	rc := C.pcre2_match_8(
		p.code,
		(C.PCRE2_SPTR8)(ptr),
		C.PCRE2_SIZE(n),
		0, 0, p.md, nil,
	)
	if rc < 0 {
		return -1
	}
	ov := C.pcre2_get_ovector_pointer_8(p.md)
	return int(*(*C.PCRE2_SIZE)(unsafe.Pointer(ov)))
}

// Close releases the compiled pattern set.
func (p *PCRE2) Close() {
	if p.md != nil {
		C.pcre2_match_data_free_8(p.md)
		p.md = nil
	}
	if p.code != nil {
		C.pcre2_code_free_8(p.code)
		p.code = nil
	}
}
