# histrionic

Better shell history (at least for bash)

Histrionic is a small tool that stores extra metadata about shell history and makes it readily searchable with `fzf`. If you are using `fzf` already, the gains are modest and mostly present when you have a large number of machines or sessions.

Shell commands are stored immediately, rather than on shell exit. This allows completing across all active shell sessions. When searching, common commands are pruned and coalesced so the results are more relevant. Errors are also presented so you can filter out commands based on exit status.

# Getting Started

You should backup your existing shell history just in case and examine the shell integration a bit beforehand.

> NOTE: When the shell exits, command history is pruned, culled and the bash history file is overwritten with a new version managed by `histrionic`. This is not critical, but it can be useful to keep the in-bash prefix completion mechanism lean and relevant.

```
go get github.com/msolo/histrionic/cmd/histrionic
source $(go env GOPATH)/src/github.com/msolo/histrionic/histrionic.bash
```

# Demo
```
  2390    ok      ./test/test.sh
  2391    ok      fg
  2392    ok      git remote add origin https://github.com/msolo/histrionic.git
  2393    ok      man git-branch
  2394    ok      git branch -M master
  2395    ok      git st
  2396    ok      lt
  2397    ok      git log
  2398    ok      cd cmd/histrionic/
  2399    ok      go install
  2400    ok      l
  2401    ok      ..
> 2402    err:130 histrionic dump -coalesce -prune ~/.bash-archives/rogue.hjs..
  2402/2402 +S
>

  histrionic dump -coalesce -prune ~/.bash-archives/rogue.hjs | fzf -d'\t' -n2
  ..3 --tiebreak=index --tac --no-multi --preview-window=down:4:wrap  --previe
  w='echo {3}'
```

# Key Bindings within bash
 * Ctrl-R - search history and the current shell session
 * Meta-R - search history and all active sessions on the machine

# Key Bindings within `fzf`
 * Ctrl-Q - toggle preview pane on/off
