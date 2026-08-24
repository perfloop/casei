//go:build amd64 && go1.24

package casei

import (
	"bytes"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/cpu"
)

func BenchmarkLiteralSkipExactCeiling(b *testing.B) {
	if !cpu.X86.HasAVX512F || !cpu.X86.HasAVX512BW {
		b.Skip("AVX-512 BW exact-byte path is disabled")
	}
	input := []byte(strings.Repeat("x", 5<<20))
	target := uint64(' ') * byteOnes
	b.Run("candidate", func(b *testing.B) {
		b.SetBytes(int64(len(input)))
		for b.Loop() {
			_ = literalSkipExact64(unsafe.SliceData(input), len(input), target)
		}
	})
	b.Run("index_byte", func(b *testing.B) {
		b.SetBytes(int64(len(input)))
		for b.Loop() {
			_ = bytes.IndexByte(input, ' ')
		}
	})
}
