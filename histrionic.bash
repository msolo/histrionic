if ! which fzf > /dev/null; then
  echo "histrionic: no fzf binary found in $PATH" >&2
  return 1
fi

if ! which histrionic > /dev/null; then
  echo "histrionic: no histrionic binary found in $PATH" >&2
  return 1
fi

if [[ "$__histrionic_session" != "" ]]; then
  echo "histrionic: already inited - skipping" >&2
  return 1
fi

__histrionic_archive_dir="$HOME/.bash-archives"
__histrionic_session="$EPOCHREALTIME-$$"
__histrionic_session_file="$__histrionic_archive_dir/$HOSTNAME@$__histrionic_session.hjs"
__histrionic_archive_file="$__histrionic_archive_dir/$HOSTNAME.hjs"

# If there is no archive, try to create one from existing bash history.
function __histrionic_init {
  mkdir -p -m 755 "$HOME/.bash-archive"
  if [ ! -e $__histrionic_archive_file ]; then
  	histrionic import -bash-histfile $HISTFILE -hostname $HOSTNAME -o $__histrionic_archive_file || return $?
  fi
}

function __histrionic_prompt {
	local rc=$?
	builtin fc -nl -1 | histrionic append -exit-code $rc -session $__histrionic_session -hostname $HOSTNAME -o $__histrionic_session_file
}

function __histrionic_exit {
	  histrionic merge -o $__histrionic_archive_file $__histrionic_archive_file $__histrionic_session_file || return $?
    rm $__histrionic_session_file
	histrionic dump -history-fmt -o $HISTFILE -coalesce -prune $__histrionic_archive_file || return $?
  return $?
}

function __histrionic_search {
  local output
  output=$(
    histrionic dump -coalesce -prune -print0 "$@" |
      FZF_DEFAULT_OPTS="--height ${FZF_TMUX_HEIGHT:-40%} $FZF_DEFAULT_OPTS -d'\t' -n2..3 --tiebreak=index --bind=ctrl-r:toggle-sort --bind=ctrl-q:toggle-preview $FZF_CTRL_R_OPTS --no-multi --tac --read0" $(__fzfcmd) --query "$READLINE_LINE" --preview='echo {3}' --preview-window=down:4:wrap:noborder ) || return
  # intput/output to fzf is tab delimited. the last field must be the command, which can't contain tabs (fingers crossed)
  # This next line does a greedy match for *\t but requires some fancy escaping for reasons I don't quite fully comprehend, but it's bash.
  READLINE_LINE=${output##*$'\t'}
  if [ -z "$READLINE_POINT" ]; then
    echo "$READLINE_LINE"
  else
    READLINE_POINT=0x7fffffff
  fi
}

function __histrionic_search_local {
  echo __histrionic_search $__histrionic_archive_file $__histrionic_session_file
  __histrionic_search $__histrionic_archive_file $__histrionic_session_file
}

function __histrionic_search_host {
  __histrionic_search $__histrionic_archive_file $__histrionic_archive_dir/$HOSTNAME@*.hjs
}

function __histrionic_search_all {
  __histrionic_search $__histrionic_archive_dir/*.hjs
}


__histrionic_init

# Make sure all commands are appended immediately to the session command.
PROMPT_COMMAND="__histrionic_prompt;$PROMPT_COMMAND"

# Install a trap on exit. If we error out, hang for a bit so we can at least see there an error before the shell window closes.
# TODO(msolo) Play nice and make this preserve existing EXIT traps.
trap '__histrionic_exit || sleep 60' EXIT

bind -m emacs-standard -x '"\C-r": __histrionic_search_local'
bind -m emacs-standard -x '"\M-r": __histrionic_search_host'
