# codeshot shell shim for bash.
#
# Pipe mode cannot know what produced the bytes on its standard input:
# `npm test | codeshot shot.png` hands codeshot the output and nothing else,
# so the prompt line in the picture would have no command on it. This shim
# recovers the command from the shell's own history - the line being run is
# already in it - strips the `| codeshot ...` off the end, and passes what is
# left as --command.
#
# Install by sourcing it from ~/.bashrc:
#
#     source /path/to/codeshot/shim/codeshot.bash
#
# bash only keeps history in an interactive shell, so like zsh's this works
# where you type, not in scripts. It does nothing in wrapper mode
# (`codeshot -- npm test`), where the command is argv and already known, and
# nothing when --command was passed by hand.

# _codeshot_first_word is the first word of a pipeline segment, with the
# leading whitespace the split leaves behind taken off first.
_codeshot_first_word() {
	local seg="$1"
	seg="${seg#"${seg%%[![:space:]]*}"}"
	printf '%s' "${seg%%[[:space:]]*}"
}

# _codeshot_strip_pipe drops codeshot's own segment, and everything after it,
# from a command line: what is left is the command that produced the bytes.
# It is split out from the shim so it can be tested on its own.
#
# A line with no pipe in it produced nothing for codeshot to read - it is
# `codeshot --clip < dump.ansi`, or a redirect - and there is no command to
# recover, so it comes back empty and the prompt line is left out rather than
# claiming codeshot's own invocation was the command.
_codeshot_strip_pipe() {
	local line="$1"
	[[ "$line" == *"|"* ]] || return 0
	# Walk in from the right until codeshot's own segment is the last one.
	# Usually it already is; `npm test | codeshot | pngquant` is why this is
	# a loop rather than a single chop.
	while [[ "$line" == *"|"* ]] && [[ "$(_codeshot_first_word "${line##*|}")" != codeshot ]]; do
		line="${line%|*}"
	done
	[[ "$line" == *"|"* ]] || return 0
	line="${line%|*}"
	# Trim the trailing whitespace the split leaves behind.
	line="${line%"${line##*[![:space:]]}"}"
	printf '%s' "$line"
}

# _codeshot_history_line is the line being run. `history 1` is the current
# one; `fc -ln -1` is the line before it, in a pipeline and out of one, which
# is how this shim spent its first release captioning every picture with
# whatever the user had typed previously. HISTTIMEFORMAT is cleared so that a
# user who has set one does not get a timestamp in the caption, and the index
# `history` prints in front of every line is taken off.
_codeshot_history_line() {
	local line
	line="$(HISTTIMEFORMAT= history 1 2>/dev/null)"
	# `history` prints "  512  npm test": indent, index, then more than one
	# space. Trim, drop the index, and trim what it was padded with.
	line="${line#"${line%%[![:space:]]*}"}"
	line="${line#* }"
	line="${line#"${line%%[![:space:]]*}"}"
	printf '%s' "$line"
}

codeshot() {
	local arg command=""
	for arg in "$@"; do
		# Wrapper mode and an explicit --command both already know better.
		if [[ "$arg" == "--" || "$arg" == "--command" || "$arg" == --command=* ]]; then
			command codeshot "$@"
			return
		fi
	done
	# Only pipe mode needs this, and pipe mode is the case where standard
	# input is not a terminal.
	if [[ ! -t 0 ]]; then
		command="$(_codeshot_strip_pipe "$(_codeshot_history_line)")"
	fi
	if [[ -n "$command" ]]; then
		command codeshot --command "$command" "$@"
	else
		command codeshot "$@"
	fi
}
