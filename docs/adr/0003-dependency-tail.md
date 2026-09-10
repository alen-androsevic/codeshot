# 0003: go-runewidth's dependency tail

## Status

Accepted, with a condition for revisiting.

## Context

The spec (§5) names codeshot's dependencies exactly: `github.com/creack/pty`,
`github.com/mattn/go-runewidth`, `golang.org/x/image`. "Nothing else."

`go-runewidth` at v0.0.30 is no longer a leaf. It requires
`github.com/clipperhouse/uax29/v2`, which is therefore in `go.mod` as an
indirect requirement and in the build:

```
$ go mod graph | grep runewidth
codeshot github.com/mattn/go-runewidth@v0.0.30
github.com/mattn/go-runewidth@v0.0.30 github.com/clipperhouse/uax29/v2@v2.2.0
```

So the "nothing else" rule is already broken, and it was broken by accident —
nobody chose `uax29`, it arrived attached to a version bump. A rule breached
without anyone deciding to breach it is worth converting into a decision, one
way or the other, before it becomes precedent.

## What the dependency buys

`go-runewidth` answers one question for codeshot: how many cells does this
rune occupy? That is asked in exactly one place, `vt.Emulator.print`, and its
answer decides the whole grid's alignment. Getting it wrong shifts every
column after a wide rune.

The recent versions that pull in `uax29` are the ones that handle grapheme
clusters and the newer emoji sequences properly — a flag, a family emoji, a
skin-tone modifier. Those are exactly the inputs where a naive width table
gives the wrong answer, and where a wrong answer is visible as misalignment
rather than as a subtly wrong glyph.

## The alternative

Pin `go-runewidth` to a pre-v0.0.30 release, from before the `uax29` split.
Those cuts have no dependencies at all and would restore the spec's rule
exactly.

The cost is the grapheme-cluster handling. codeshot cannot draw colour emoji
anyway — Apple Color Emoji is a bitmap `sbix` font that `golang.org/x/image`
cannot read, which the spec records as a known limitation (§7) — so an emoji
in a capture already renders as tofu. But *width* is a separate question from
*coverage*: a tofu box in the wrong number of cells misaligns everything after
it on the line, while a tofu box in the right number of cells is merely ugly.
The older cut would make emoji-bearing captures misalign, not just look plain.

## Decision

The current choice stands: `go-runewidth` at v0.0.30, with `uax29` in the
tail, and the spec's dependency rule is hereby amended to record it rather
than to be quietly violated.

The reasoning is that `uax29` is a small, single-purpose, pure-Go library
doing the Unicode segmentation that the width question actually requires; it
brings no cgo, no external binary, and no tail of its own. It is much closer
to a vendored table than to a framework. Trading correct alignment on
real-world input for the tidiness of a one-line dependency list is a bad
trade, and the spec's rule was written to keep out weight and cgo, not to keep
out a transitive Unicode table.

## When to revisit

- If `uax29` grows a dependency tail of its own, or acquires cgo. Then the
  reason this was acceptable no longer holds, and the older runewidth cut
  becomes the better option.
- If `go-runewidth` becomes a leaf again, take that version.
- If codeshot ever grows a real font-fallback chain that can draw emoji, the
  width question gets harder rather than easier, and this dependency becomes
  more load-bearing, not less — which is an argument for keeping it, recorded
  here so the argument is not re-litigated from scratch.
