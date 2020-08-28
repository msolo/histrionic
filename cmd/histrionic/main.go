package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"
)

/*
1. track exit code
2. track exit timestamp
2b. track elapsed time using EPOCHREALTIME and DEBUG trap?
  or PS0 hack to record time?
3. track host/session for collation
4. allow periodic sync to filesystem?
5. allow periodic re-read by another history? using alarm or shell background?

history -a <file> appends all items since the last time history -a was called. this does not seem documented, but might be handy.
if the last command is a duplicate, nothing is appended so you can't tell an empty command from successful re-run.

PROMPT_COMMAND=prompt

function prompt {
	__rc=$?
	history -a /dev/stdout | histrionic append -rc $__rc
	fc -nl | histrionic append -rc $__rc # looks ok too, just the command, no metadata.
}

*/

type histRecord struct {
	Timestamp time.Time
	Cmd       string
	SessionId string
	Hostname  string
	ExitCode  int
}

type cmdFunc func(args []string)

var cmdMap map[string]cmdFunc

var doc = `History recorder.`

func init() {
	cmdMap = map[string]cmdFunc{
		"append": cmdAppend,
		"dump":   cmdDump,
	}
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, doc)
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	cmdName := args[0]
	args = args[1:]

	if cmdName == "help" {
		if len(args) > 0 {
			cmdName = args[0]
			args = []string{"-h"}
		}
	}

	if cmd, ok := cmdMap[cmdName]; ok {
		cmd(args)
	} else {
		flag.Usage()
		os.Exit(1)
	}
}

func cmdAppend(args []string) {
	flags := flag.NewFlagSet("append", flag.ExitOnError)

	defaultHistFile := os.Getenv("HISTFILE")
	if defaultHistFile == "" {
		defaultHistFile = os.ExpandEnv("$HOME/.bash_history")
	}
	histFile := flags.String("histfile", defaultHistFile+".hjs", "History file.")
	session := flags.String("session", "", "Bash session id.")
	exitCode := flags.Int("exit-code", 0, "Exit code for command.")
	flags.Parse(args)

	bcmd, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println("histFile:", *histFile)
	// env := os.Environ()
	// sort.Strings(env)
	// fmt.Println("env:", strings.Join(env, "\n"))
	r := histRecord{Timestamp: time.Now(),
		Cmd:       string(bytes.TrimSpace(bcmd)),
		Hostname:  os.Getenv("HOSTNAME"),
		SessionId: *session,
		ExitCode:  *exitCode,
	}

	f, err := os.OpenFile(*histFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	rb, err := json.Marshal(r)
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.Write(rb)
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.Write([]byte("\n"))
	if err != nil {
		log.Fatal(err)
	}

	if err = f.Sync(); err != nil {
		log.Fatal(err)
	}
}

func cmdDump(args []string) {
	flags := flag.NewFlagSet("dump", flag.ExitOnError)

	defaultHistFile := os.Getenv("HISTFILE")
	if defaultHistFile == "" {
		defaultHistFile = os.ExpandEnv("$HOME/.bash_history")
	}
	histFile := flags.String("histfile", defaultHistFile+".hjs", "History file.")
	print0 := flags.Bool("print0", false, "Print commands null delimited.")
	coalesce := flags.Bool("coalesce", false, "Coalesce duplicates and failing commands.")
	noLineNumber := flags.Bool("n", false, "Do not print line numbers.")
	flags.Parse(args)

	f, err := os.Open(*histFile)
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(f)

	rs := make([]*histRecord, 0, 1024)
	// Iterate over history items.
	for dec.More() {
		r := &histRecord{}
		if err := dec.Decode(r); err != nil {
			log.Fatal(err)
		}
		rs = append(rs, r)
	}

	prunedRs := make([]*histRecord, 0, 1024)
	if *coalesce {
		cmdMap := make(map[string]*histRecord)
		for i := len(rs) - 1; i >= 0; i-- {
			r := rs[i]
			oldR, ok := cmdMap[r.Cmd]
			if ok {
				// if the command might succeed, record it as a success
				if oldR.ExitCode != 0 && r.ExitCode == 0 {
					oldR.ExitCode = r.ExitCode
				}
			} else {
				prunedRs = append(prunedRs, r)
				cmdMap[r.Cmd] = r
			}
		}
	} else {
		for i := len(rs) - 1; i >= 0; i-- {
			prunedRs = append(prunedRs, rs[i])
		}
	}

	for i, r := range prunedRs {
		cmd := ""
		if !*noLineNumber {
			cmd = cmd + strconv.Itoa(len(prunedRs)-i) + "\t"
		}
		if r.ExitCode != 0 {
			cmd = cmd + "err:" + strconv.Itoa(r.ExitCode) + "\t"
		} else {
			cmd = cmd + "ok\t"
		}

		cmd = cmd + r.Cmd

		// cmd = cmd + "\t" + r.Timestamp.Format(time.RFC3339)
		io.WriteString(os.Stdout, cmd)
		if *print0 {
			io.WriteString(os.Stdout, "\000")
		} else {
			io.WriteString(os.Stdout, "\n")
		}
	}
}
