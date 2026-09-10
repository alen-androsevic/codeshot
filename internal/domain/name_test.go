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
