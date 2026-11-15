package fonts

import (
	"sort"
	"strings"
)

// SymbolFamilies is the installed families that exist to carry symbols a
// text font has no reason to have: Nerd Fonts, which put icons in the
// private use area, and Powerline patches.
//
// They cannot go in the platform's fixed fallback list. That list is names
// codeshot knows in advance, and a Nerd Font's name is invented by whoever
// patched it - "JetBrainsMono Nerd Font", "Hack Nerd Font Propo", one per
// family per variant. So they are found by the shape of the name instead,
// which is a convention the whole Nerd Fonts project follows.
//
// A symbols-only font comes first, because it is the one that carries the
// icons and nothing else: consulting it cannot change how any letter looks.
// The rest follow alphabetically, so that the same machine gives the same
// picture twice (design §2).
func (i Index) SymbolFamilies() []string {
	var names []string
	for _, f := range i.Families {
		if isSymbolFamily(f.Name) {
			names = append(names, f.Name)
		}
	}
	sort.Slice(names, func(a, b int) bool {
		if x, y := symbolsOnly(names[a]), symbolsOnly(names[b]); x != y {
			return x
		}
		return names[a] < names[b]
	})
	return names
}

func isSymbolFamily(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "nerd font") || strings.Contains(n, "powerline")
}

// symbolsOnly reports whether a family is the icons-only cut. Nerd Fonts
// ships it as "Symbols Nerd Font" and "Symbols Nerd Font Mono"; the name
// starts with the word, where a patched text family ends with it.
func symbolsOnly(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "symbols")
}
