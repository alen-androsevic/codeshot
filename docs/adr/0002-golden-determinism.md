# 0002: Golden-image determinism across macOS and Linux

## Status

Accepted.

## Context

The spec's first listed risk (§12) is golden-image determinism: the renderer's
tests compare PNGs byte-exact, and that only works if `golang.org/x/image`
rasterises identically wherever the suite runs. If it does not, the fallback
named in the spec is to compare with a per-pixel tolerance instead — a
meaningful loss, because a tolerance hides small real regressions along with
the platform noise it is there to absorb.

The spec asks for this to be verified "in the very first renderer commit". It
was verified during implementation. What was missing is any record of it, so
the risk read as open to anyone looking at the repository, and the next person
to see a golden fail on Linux would have had no way to know whether they were
looking at the thing this risk predicted.

## What was run

The full test suite, including `TestGoldenShots`, in a `golang:1.27` Linux
container. The resulting PNGs were compared byte-for-byte against the
`testdata/*.png` files generated on macOS.

Separately, the Linux capture form used by the `Taskfile`'s demo task —
`script -qc "…" file.ansi`, GNU script's one-shot form, as against BSD/macOS
script's trailing-argument form — was confirmed to produce carriage returns in
its output, which is what keeps a captured dump from stair-stepping when it is
rendered.

## What it showed

The goldens were byte-identical between the two platforms. No tolerance was
needed and none was introduced.

## The caveat

Both machines were arm64. **amd64 was not checked.** So what this ADR records
is that the same architecture on two different operating systems agrees; it is
not evidence that rasterisation is identical across instruction sets. x86-64
and arm64 differ in floating-point details that a rasteriser can be sensitive
to — most obviously the availability of fused multiply-add, which changes
rounding.

That gap matters most for CI, which is the likeliest place for an amd64
runner to appear. If the goldens ever fail there and nowhere else, this is the
first thing to suspect, and the check to run is the one above with the
architecture varied instead of the operating system.

## Decision

Byte-exact comparison stays. No tolerance is introduced against a difference
nobody has observed: a tolerance wide enough to absorb hypothetical
cross-architecture rounding would also be wide enough to swallow a genuine
one-pixel regression, and the whole value of a golden image is that it
notices small things.

If an amd64 difference is ever actually observed, the decision to revisit is
this one, and the options then are a tolerance, per-architecture goldens, or
pinning the rasteriser's arithmetic — in that order of increasing effort and
decreasing looseness.
