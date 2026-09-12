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

# _codeshot_strip_pipe drops the last pipeline segment from a command line.
# It is split out from the shim so it can be tested on its own; recovering
# the history line cannot be, since a non-interactive shell keeps no history.
_codeshot_strip_pipe() {
	local line="$1"
	line="${line%|*}"
	# Trim the trailing whitespace the split leaves behind.
	line="${line%"${line##*[![:space:]]}"}"
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
		# fc -ln -1 is the line being run; history is written before it runs.
		command="$(_codeshot_strip_pipe "$(fc -ln -1 2>/dev/null)")"
	fi
	if [[ -n "$command" ]]; then
		command codeshot --command "$command" "$@"
	else
		command codeshot "$@"
	fi
}
