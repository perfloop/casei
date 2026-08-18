package casei_test

import (
	"fmt"

	"github.com/tsenart/casei"
)

func ExampleIndexFold() {
	fmt.Println(casei.IndexFold("temperature: 273K", "273k"))
	// Output: 13
}

func ExampleContainsFold() {
	fmt.Println(casei.ContainsFold("payment DECLINED", "Payment declined"))
	// Output: true
}

func ExampleMatcher() {
	patterns := []string{"fatal panic", "oom killed", "segfault"}
	matcher := casei.NewMatcher(patterns)

	match, ok := matcher.Find("worker: OOM KILLED after 1s")
	fmt.Println(ok, match.Start, patterns[match.Pattern])
	// Output: true 8 oom killed
}
