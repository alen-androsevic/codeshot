package domain

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"paradajz danas":     "paradajz-danas",
		"git status --short": "git-status-short",
		"  ls   -la  ":       "ls-la",
		"cat /etc/hosts":     "cat-etc-hosts",
		"echo 'hi there!'":   "echo-hi-there",
		"LS -la":             "ls-la",
		"":                   "codeshot",
		"!!!":                "codeshot",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugIsBounded(t *testing.T) {
	long := ""
	for i := 0; i < 50; i++ {
		long += "word "
	}
	if got := Slug(long); len(got) > 60 {
		t.Errorf("Slug produced %d characters, want at most 60", len(got))
	}
}

func TestResolveNamePrefersWhatWasAskedFor(t *testing.T) {
	none := func(string) bool { return false }
	if got := ResolveName("shot.png", "ls", none); got != "shot.png" {
		t.Errorf("got %q", got)
	}
	if got := ResolveName("shot", "ls", none); got != "shot.png" {
		t.Errorf("got %q, want the extension added", got)
	}
	if got := ResolveName("", "paradajz danas", none); got != "paradajz-danas.png" {
		t.Errorf("got %q", got)
	}
}

func TestResolveNameAvoidsCollisions(t *testing.T) {
	taken := map[string]bool{"ls.png": true, "ls-2.png": true}
	got := ResolveName("", "ls", func(n string) bool { return taken[n] })
	if got != "ls-3.png" {
		t.Errorf("got %q, want ls-3.png", got)
	}
}

func TestResolveNameDoesNotRenameAnExplicitChoice(t *testing.T) {
	// Asking for a name is asking for that name; overwriting is the CLI's
	// decision to make with --force, not the domain's.
	got := ResolveName("shot.png", "ls", func(string) bool { return true })
	if got != "shot.png" {
		t.Errorf("got %q, want the explicit name untouched", got)
	}
}

// TestResolveNameAddsPNGUnlessItIsThere pins what "a missing extension gets
// .png" means (design §9). It used to mean "no dot anywhere", so a name with
// a dot for any other reason was taken to have an extension already:
// `codeshot v0.2.0 -- git log` wrote PNG bytes to a file called v0.2.0, the
// first time codeshot was used to picture its own release. The only
// extension that makes a name finished is .png, because PNG is the only
// thing codeshot writes - a name ending in anything else gets .png after it,
// rather than a PNG wearing someone else's extension.
func TestResolveNameAddsPNGUnlessItIsThere(t *testing.T) {
	none := func(string) bool { return false }
	cases := map[string]string{
		"v0.2.0":             "v0.2.0.png",
		"my.backup":          "my.backup.png",
		"shot.jpg":           "shot.jpg.png",
		"shot.PNG":           "shot.PNG",
		"shot.png":           "shot.png",
		"~/Desktop/shot":     "~/Desktop/shot.png",
		"dir.d/shot":         "dir.d/shot.png",
		"dir.d/v1.2":         "dir.d/v1.2.png",
		".hidden":            ".hidden.png",
		"release-notes.png.": "release-notes.png..png",
	}
	for in, want := range cases {
		if got := ResolveName(in, "ls", none); got != want {
			t.Errorf("ResolveName(%q) = %q, want %q", in, got, want)
		}
	}
}
