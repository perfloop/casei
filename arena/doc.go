// Package arena holds the competitive field: the baselines a candidate is
// measured against, the scenario corpora, and the scoreboard.
//
// It is a separate Go module on purpose. The candidate module must not be able
// to import a competitor -- an engine that calls veloz cannot beat veloz, it
// can only add overhead to it, and the scoreboard would still report a number
// as though a search had been invented. Keeping the field in its own module
// makes that class of result unrepresentable rather than merely discouraged.
package arena
