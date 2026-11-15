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
	local -a argv=("$@") before after words
	local arg expansion="" sawcommand=0
	local -i split=-1 i
	for (( i = 0; i < ${#argv[@]}; i++ )); do
		arg="${argv[i]}"
		case "$arg" in
			--command|--command=*) sawcommand=1 ;;
		esac
		if [[ "$arg" == "--" ]]; then
			split=$i
			break
		fi
	done

	# Wrapper mode. codeshot execs the command itself, and an alias lives in
	# the shell where no child process can see it - `codeshot -- ll` came
	# back "command not found". The shell is the only thing that can answer,
	# and the shim is in the shell, so it answers here.
	#
	# BASH_ALIASES needs bash 4; on the bash 3.2 Apple ships the lookup finds
	# nothing and an alias behaves as it did before, which is to say not.
	if (( split >= 0 )); then
		before=("${argv[@]:0:split}")
		after=("${argv[@]:split+1}")
		if (( ${#after[@]} > 0 )); then
			expansion="${BASH_ALIASES[${after[0]}]:-}"
		fi
		if [[ -n "$expansion" ]]; then
			# The caption stays what the user typed: their own scrollback
			# says `ll`, not what it stands for.
			(( sawcommand )) || before+=(--command "${after[*]}")
			if [[ "$expansion" == *[\|\&\;\<\>\(\)\`]* ]]; then
				# An alias with a pipe or a redirect in it is a little
				# script, and only a shell can run it.
				command codeshot "${before[@]}" -- bash -c "$expansion \"\$@\"" bash "${after[@]:1}"
			else
				# eval is how the shell itself splits an alias into words,
				# quotes and all. The text is the user's own alias, which
				# they were about to run anyway.
				eval "words=( $expansion )"
				command codeshot "${before[@]}" -- "${words[@]}" "${after[@]:1}"
			fi
			return
		fi
		command codeshot "$@"
		return
	fi

	# Pipe mode is the case where standard input is not a terminal, and the
	# only one that needs history.
	local command=""
	if (( ! sawcommand )) && [[ ! -t 0 ]]; then
		command="$(_codeshot_strip_pipe "$(_codeshot_history_line)")"
	fi
	if [[ -n "$command" ]]; then
		command codeshot --command "$command" "$@"
	else
		command codeshot "$@"
	fi
}
