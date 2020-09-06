package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/msolo/go-bis/flock"
	"github.com/msolo/go-bis/ioutil2"
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
*/

// TODO(msolo) Add mode to merge/purge
// TOOD(msolo) Figure out how to display session info - maybe record in separate file and then merge on shell exit?

type histRecord struct {
	Timestamp time.Time
	Cmd       string
	SessionId string
	Hostname  string
	ExitCode  int
}

func readRecords(fname string) ([]*histRecord, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)

	rs := make([]*histRecord, 0, 1024)
	// Iterate over history items.
	for dec.More() {
		r := &histRecord{}
		if err := dec.Decode(r); err != nil {
			return nil, err
		}
		rs = append(rs, r)
	}
	return rs, nil
}

type recordWriter struct {
	wr io.Writer
}

func (recw *recordWriter) WriteRecord(rec *histRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = recw.wr.Write(data)
	return err
}

func newRecordWriter(w io.Writer) *recordWriter {
	return &recordWriter{wr: w}
}

func writeHistory(w io.Writer, rs []*histRecord) {
	for _, r := range rs {
		fmt.Fprintf(w, "#%d\n", r.Timestamp.Unix())
		fmt.Fprintln(w, r.Cmd)
	}
}

func writeLines(w io.Writer, rs []*histRecord, lineNumbers bool, print0 bool) {
	for i, r := range rs {
		cmd := ""
		if lineNumbers {
			cmd += strconv.Itoa(i+1) + "\t"
		}
		if r.ExitCode != 0 {
			cmd += "err:" + strconv.Itoa(r.ExitCode) + "\t"
		} else {
			cmd += "ok\t"
		}

		cmd += r.Cmd

		// cmd += "\t" + r.Timestamp.Format(time.RFC3339)
		io.WriteString(w, cmd)
		if print0 {
			io.WriteString(w, "\000")
		} else {
			io.WriteString(w, "\n")
		}
	}
}

var limitCommands = []string{
	"cat",
	"cd",
	"cp",
	"diff",
	"echo",
	"egrep",
	"find",
	"git add",
	"git commit",
	"grep",
	"head",
	"hg add",
	"hg resolve",
	"l",
	"less",
	"ls",
	"lt",
	"man",
	"mkdir",
	"mv",
	"pbpaste",
	"peg",
	"ping",
	"port",
	"ps",
	"python",
	"rm",
	"rsync",
	"scp",
	"ssh",
	"svn add",
	"tail",
	"telnet",
	"touch",
	"wc",
	"wget",
	"which",
	"whois",
	"xattr",
}

var excludeCommands = []string{
	"bg",
	"code",
	"emacs",
	"emc",
	"fg",
	"locate",
	"mate",
	"md5",
	"open",
}

func hasCmdPrefix(cmd, prefix string) bool {
	if strings.HasPrefix(cmd, prefix) {
		if len(cmd) == len(prefix) {
			return true
		}
		if len(prefix) < len(cmd) && cmd[len(prefix)] == ' ' {
			return true
		}
	}
	return false
}

func matchesLimit(cmd string) (pattern string) {
	for _, prefix := range limitCommands {
		if hasCmdPrefix(cmd, prefix) {
			return prefix
		}
	}
	return ""
}

func matchesExclude(cmd string) bool {
	for _, prefix := range excludeCommands {
		if hasCmdPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

type byTime []*histRecord

func (a byTime) Len() int           { return len(a) }
func (a byTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTime) Less(i, j int) bool { return a[i].Timestamp.Before(a[j].Timestamp) }

// Sort ascending by time.
func sortByTime(rs []*histRecord) {
	sort.Sort(byTime(rs))
}

// Prune some commands and limit the number of entries for some command types.
// Assume rs is in ascending time order.
func pruneRecords(rs []*histRecord) []*histRecord {
	outRs := make([]*histRecord, 0, len(rs))
	cmdCountMap := make(map[string]int)

	perCmdLimit := 30

	//FIXME(msolo) Should we prune tools that exit 127? That seems to indicate no tool was found, but it's probably not conclusive.
	for i := len(rs) - 1; i >= 0; i-- {
		r := rs[i]
		if matchesExclude(r.Cmd) {
			continue
		}
		pattern := matchesLimit(r.Cmd)
		if pattern != "" {
			cmdCountMap[pattern] += 1
			if cmdCountMap[pattern] > perCmdLimit {
				continue
			}
		}
		outRs = append(outRs, r)
	}

	reverse(outRs)
	return outRs
}

// Reverse records in place.
func reverse(rs []*histRecord) {
	for i := 0; i < len(rs)/2; i++ {
		rs[i], rs[len(rs)-i-1] = rs[len(rs)-i-1], rs[i]
	}
}

// Coalesce exact duplicate commands. For exact commands, record success if it *might* succeed.
// Assume rs is in ascending time order.
func coalesceRecords(rs []*histRecord) []*histRecord {
	outRs := make([]*histRecord, 0, len(rs))
	cmdMap := make(map[string]*histRecord)
	// Iterate in reverse time order.
	for i := len(rs) - 1; i >= 0; i-- {
		r := rs[i]
		oldR, ok := cmdMap[r.Cmd]
		if ok {
			// If the command might succeed, record it as a success.
			if oldR.ExitCode != 0 && r.ExitCode == 0 {
				oldR.ExitCode = r.ExitCode
			}
		} else {
			outRs = append(outRs, r)
			cmdMap[r.Cmd] = r
		}
	}
	// Return rows in ascending order.
	reverse(outRs)
	return outRs
}

type cmdFunc func(args []string)

var cmdMap map[string]cmdFunc

var doc = `Shell command history recorder.

  histrionic append
  histrionic dump
  histrionic import
  histrionic merge

Each mode accepts -h for additional command help.
`

func init() {
	cmdMap = map[string]cmdFunc{
		"append": cmdAppend,
		"dump":   cmdDump,
		"import": cmdImport,
		"merge":  cmdMerge,
	}
	log.SetFlags(0)

	flag.Usage = func() {
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

	outFile := flags.String("o", "", "History file.")
	hostname := flags.String("hostname", os.Getenv("HOSTNAME"), "See the hostname instead of inferring it from $HOSTNAME.")
	session := flags.String("session", "", "Bash session id.")
	exitCode := flags.Int("exit-code", 0, "Exit code for command.")
	timestamp := flags.Int64("timestamp", 0, "Override timestamp.")

	flags.Parse(args)
	bcmd, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println("histFile:", *histFile)
	// env := os.Environ()
	// sort.Strings(env)
	// fmt.Println("env:", strings.Join(env, "\n"))

	ts := time.Now()
	if *timestamp != 0 {
		ts = time.Unix(*timestamp, 0)
	}

	r := histRecord{Timestamp: ts,
		Cmd:       string(bytes.TrimSpace(bcmd)),
		Hostname:  *hostname,
		SessionId: *session,
		ExitCode:  *exitCode,
	}

	f, err := os.OpenFile(*outFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = f.Sync()
		_ = f.Close()
	}()
	recWr := newRecordWriter(f)
	recWr.WriteRecord(&r)
}

func newAtomicFileWriter(fname string, perm os.FileMode) (io.WriteCloser, error) {
	if strings.HasPrefix(fname, "/dev/") {
		return os.OpenFile(fname, os.O_APPEND|os.O_WRONLY, perm)
	}
	return ioutil2.NewAtomicFileWriter(fname, perm)
}

func cmdDump(args []string) {
	flags := flag.NewFlagSet("dump", flag.ExitOnError)

	print0 := flags.Bool("print0", false, "Print commands null delimited.")
	hostname := flags.String("x-hostname", "", "Restrict output to commands that occurred on hostname.")
	coalesce := flags.Bool("coalesce", false, "Coalesce duplicates and failing commands.")
	prune := flags.Bool("prune", false, "Prune low-values commands from the history.")
	noLineNumber := flags.Bool("n", false, "Do not print line numbers.")
	historyFmt := flags.Bool("history-fmt", false, "Write output in a history-compatible format.")
	outFile := flags.String("o", "/dev/stdout", "Output file.")

	flags.Parse(args)

	rs := make([]*histRecord, 0, 1024)
	for _, fname := range flags.Args() {
		trs, err := readRecords(fname)
		if err != nil {
			log.Fatal(err)
		}
		rs = append(rs, trs...)
	}

	if *coalesce {
		rs = coalesceRecords(rs)
	}
	if *prune {
		rs = pruneRecords(rs)
	}

	if *hostname != "" {
		filtered := make([]*histRecord, 0, len(rs))
		for _, r := range rs {
			if r.Hostname == "" || r.Hostname == *hostname {
				filtered = append(filtered, r)
			}
		}
		rs = filtered
	}

	wr, err := newAtomicFileWriter(*outFile, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer wr.Close()

	if *historyFmt {
		writeHistory(wr, rs)
		return
	}
	writeLines(wr, rs, !*noLineNumber, *print0)
}

func cmdImport(args []string) {
	flags := flag.NewFlagSet("import", flag.ExitOnError)

	bashHistFile := flags.String("bash-histfile", "", "Native history file.")
	outFile := flags.String("o", "/dev/stdout", "Output file.")
	hostname := flags.String("hostname", os.Getenv("HOSTNAME"), "Associate all entries with a particular hostname.")

	flags.Parse(args)

	f, err := os.Open(*bashHistFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	wr, err := newAtomicFileWriter(*outFile, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := wr.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	recWr := newRecordWriter(wr)

	rd := bufio.NewReader(f)
	for {
		tsLine, _ := rd.ReadString('\n')
		cmdLine, lineErr := rd.ReadString('\n')

		if lineErr != nil && lineErr != io.EOF {
			log.Fatal(lineErr)
		}
		if lineErr == io.EOF {
			break
		}

		if tsLine[0] != '#' {
			log.Fatal("bad timestamp comment:", string(tsLine))
		}

		ts, err := strconv.Atoi(tsLine[1 : len(tsLine)-1])
		if err != nil {
			log.Fatal(err)
		}

		r := &histRecord{Timestamp: time.Unix(int64(ts), 0),
			Cmd:       strings.TrimSpace(cmdLine),
			Hostname:  *hostname,
			SessionId: "import",
			ExitCode:  0,
		}

		if err := recWr.WriteRecord(r); err != nil {
			log.Fatal(err)
		}
	}
}

func cmdMerge(args []string) {
	flags := flag.NewFlagSet("merge", flag.ExitOnError)
	outFile := flags.String("o", "/dev/stdout", "Output file.")

	err := flags.Parse(args)
	if err != nil {
		log.Fatal("flag error:", err)
	}

	if err := merge(*outFile, flags.Args()); err != nil {
		log.Fatal(err)
	}
}
func merge(outFile string, inputFiles []string) (err error) {
	flock, err := flock.Open(outFile)
	if err != nil {
		return err
	}
	if err := flock.Lock(); err != nil {
		return err
	}
	defer func() {
		if err := flock.Unlock(); err != nil {
			log.Println(err)
		}
	}()

	wr, err := newAtomicFileWriter(outFile, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := wr.Close(); closeErr != nil {
			err = closeErr
		}
	}()
	recWr := newRecordWriter(wr)

	rs := make([]*histRecord, 0, 1024)
	for _, fname := range inputFiles {
		trs, err := readRecords(fname)
		if err != nil {
			return err
		}
		rs = append(rs, trs...)
	}

	sortByTime(rs)

	for _, r := range rs {
		if err := recWr.WriteRecord(r); err != nil {
			return err
		}
	}
	return nil
}
