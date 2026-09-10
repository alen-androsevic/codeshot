# JetBrains Mono

Vendored typeface for codeshot's rasteriser. JetBrains Mono is Ghostty's own
default font, so embedding it is what makes a codeshot resemble a Ghostty
window without any extra configuration.

- **Source:** https://github.com/JetBrains/JetBrainsMono/releases/download/v2.304/JetBrainsMono-2.304.zip
- **Version:** v2.304
- **Archive SHA-256:** `6f6376c6ed2960ea8a963cd7387ec9d76e3f629125bc33d1fdcd7eb7012f7bbf`

The archive's internal paths matched the plan exactly (`unzip -l` confirmed
this before extraction), so no path adaptation was needed:

```
fonts/ttf/JetBrainsMonoNL-Regular.ttf
fonts/ttf/JetBrainsMonoNL-Bold.ttf
fonts/ttf/JetBrainsMonoNL-Italic.ttf
fonts/ttf/JetBrainsMonoNL-BoldItalic.ttf
OFL.txt
```

## Cut: NL (no ligatures)

codeshot draws cell by cell and never shapes text, so ligatures could not
render even if they were present. The "NL" cut is vendored deliberately,
not as an oversight.

## Files in this directory

- `JetBrainsMonoNL-Regular.ttf`
- `JetBrainsMonoNL-Bold.ttf`
- `JetBrainsMonoNL-Italic.ttf`
- `JetBrainsMonoNL-BoldItalic.ttf`
- `OFL.txt` — the SIL Open Font License 1.1 under which JetBrains Mono is
  distributed. Required alongside the font files by the license terms.

To re-verify: download the archive above, confirm its SHA-256 matches, and
diff the extracted TTFs against the ones in this directory.
