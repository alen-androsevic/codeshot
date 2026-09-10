# 0001: Default prompt glyph

## Status

Accepted.

## Context

The spec flagged an open risk: codeshot's default prompt template wants to
use `❯` (U+276F, "heavy right-pointing angle quotation mark ornament"), the
glyph many shells (Starship, Pure, etc.) use for their prompt marker. Whether
that is safe to ship as the default depends entirely on whether the embedded
typeface — JetBrains Mono NL, vendored in `internal/adapters/fonts/assets`
(see Task 10) — actually has a glyph for it. codeshot never falls back to
another font in this phase, so an uncovered glyph would render as tofu
(`.notdef`) in every codeshot that used the default prompt.

## What was run

```go
func TestReportPromptGlyphCoverage(t *testing.T) {
	s, _ := Embedded()
	for _, r := range []rune{'❯', '›', '»', '▸', '✓', '✗', '─', '│', '·'} {
		t.Logf("%q covered=%v", r, s.CoversRune(r))
	}
}
```

run via `go test ./internal/adapters/fonts/ -run PromptGlyph -v` against the
embedded `Set` built from the vendored TTFs, using `CoversRune`, which asks
the regular face's cmap for the glyph index and treats index 0 (`.notdef`)
as "not covered."

## What it printed

```
=== RUN   TestReportPromptGlyphCoverage
    coverage_test.go:8: '❯' covered=true
    coverage_test.go:8: '›' covered=true
    coverage_test.go:8: '»' covered=true
    coverage_test.go:8: '▸' covered=true
    coverage_test.go:8: '✓' covered=true
    coverage_test.go:8: '✗' covered=true
    coverage_test.go:8: '─' covered=true
    coverage_test.go:8: '│' covered=true
    coverage_test.go:8: '·' covered=true
--- PASS: TestReportPromptGlyphCoverage (0.00s)
PASS
```

Every candidate glyph, including `❯` (U+276F), is covered by the embedded
JetBrains Mono NL regular face.

## Decision

`❯` is covered, so the risk does not materialise: Task 14's default prompt
template keeps `❯` as the prompt marker. No fallback to `>` is needed in this
phase.

If a future change to the vendored font (a version bump, a different cut)
ever drops that glyph, `CoversRune('❯')` will start reporting `false` and the
default prompt should be revisited then — this decision is tied to the
specific TTFs vendored in `internal/adapters/fonts/assets`, not to JetBrains
Mono in the abstract.
