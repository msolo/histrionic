 __histrionic_session="$EPOCHREALTIME-$$"

function __histrionic_prompt {
	local rc=$?
	fc -nl | histrionic append -rc $__rc -session $__histrionic_session
}

PROMPT_COMMAND="__histrionic_prompt;$PROMPT_COMMAND"


function __fzf_histrionic__ {
  local output
  output=$(
    histrionic dump -coalesce -print0 |
      FZF_DEFAULT_OPTS="--height ${FZF_TMUX_HEIGHT:-40%} $FZF_DEFAULT_OPTS -d'\t' -n2..3 --tiebreak=index --bind=ctrl-r:toggle-sort $FZF_CTRL_R_OPTS --no-multi --read0" $(__fzfcmd) --query "$READLINE_LINE"
  ) || return
  # intput/output to fzf is tab delimited. the last field must be the command, which can't contain tabs (fingers crossed)
  # This next line does a greedy match for *\t but requires some fancy escaping for reasons I don't quite fully comprehend, but it's bash.
  READLINE_LINE=${output##*$'\t'}
  if [ -z "$READLINE_POINT" ]; then
    echo "$READLINE_LINE"
  else
    READLINE_POINT=0x7fffffff
  fi
}

bind -m emacs-standard -x '"\C-r": __fzf_histrionic__'