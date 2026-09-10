package prompt

import (
	"strings"
	"testing"

	"codeshot/internal/domain"
)

func TestHeaderExpandsCwdAndCommand(t *testing.T) {
	tpl := Template{Text: "{cwd}\r\n> "}
	got := string(tpl.Header(domain.Capture{Cwd: "/tmp/x", Command: "ls -la"}))
	if got != "/tmp/x\r\n> ls -la\r\n" {
		t.Errorf("got %q", got)
	}
}

func TestHeaderShortensTheHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "/Users/alen")
	tpl := Template{Text: "{cwd}\r\n> "}
	got := string(tpl.Header(domain.Capture{Cwd: "/Users/alen", Command: "ls"}))
	if !strings.HasPrefix(got, "~\r\n") {
		t.Errorf("got %q, want the home directory as ~", got)
	}
	got = string(tpl.Header(domain.Capture{Cwd: "/Users/alen/code", Command: "ls"}))
	if !strings.HasPrefix(got, "~/code\r\n") {
		t.Errorf("got %q, want a path below home shortened", got)
	}
}

func TestDefaultTemplateHasTwoLinesAndColour(t *testing.T) {
	got := string(Template{Text: Default}.Header(domain.Capture{Cwd: "/tmp", Command: "ls"}))
	if strings.Count(got, "\r\n") != 2 {
		t.Errorf("got %q, want a cwd line and a command line", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Error("the default prompt has no colour in it")
	}
}

func TestHeaderWithoutACommandIsJustThePrompt(t *testing.T) {
	got := string(Template{Text: "> "}.Header(domain.Capture{}))
	if got != "> \r\n" {
		t.Errorf("got %q", got)
	}
}
