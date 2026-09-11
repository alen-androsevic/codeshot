package domain

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// slugMax keeps a generated filename readable. A command long enough to hit it
// is one nobody wants to see spelled out in a directory listing.
const slugMax = 60

// Slug turns a command line into a filename stem: lowercase words joined by
// hyphens, with punctuation dropped rather than escaped.
func Slug(command string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(command) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > slugMax {
		s = strings.Trim(s[:slugMax], "-")
	}
	if s == "" {
		return "codeshot"
	}
	return s
}

// ResolveName decides what the file is called. An explicit name is honoured
// exactly, collision or not - overwriting is the caller's decision, made with
// --force. A generated name steps aside for whatever is already there.
func ResolveName(raw, command string, exists func(string) bool) string {
	if raw != "" {
		return withPNG(raw)
	}
	stem := Slug(command)
	name := stem + ".png"
	for n := 2; exists(name); n++ {
		name = fmt.Sprintf("%s-%d.png", stem, n)
	}
	return name
}

// withPNG adds .png unless the name already ends in it. Only .png counts as
// an extension that finishes a name: PNG is all codeshot writes, so
// `my.backup` or `shot.jpg` get .png after them rather than PNG bytes under
// an extension that says otherwise, and a dot anywhere else - a version
// number, a directory - says nothing about the file's type at all.
func withPNG(name string) string {
	if strings.EqualFold(path.Ext(name), ".png") {
		return name
	}
	return name + ".png"
}
