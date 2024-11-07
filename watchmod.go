package watchmod

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
)

// flags holds setting settable by flag.
// Example with flags:
//
//	go run cmd/main.go -config=watchmod.json5 -daemon=false
type flags struct {
	ConfigPath string
	Daemon     bool
}

var FC flags
var C Config
var FlagsParsed = false

var regexes []*regexp.Regexp

func ParseFlags() {
	flag.StringVar(&FC.ConfigPath, "config", "watchmod.json5", "Path for the watchmod config.")
	flag.BoolVar(&FC.Daemon, "daemon", true, "Run as daemon.  If false, runs command and shuts down.")
	flag.BoolVar(&C.PrintStdOut, "printStdout", true, "Print the standard output from commands.  If false, standard out from commands is not printed. ")
	flag.Parse()
	parseConfig(&C)
	FlagsParsed = true

}

func Run() {
	if !FlagsParsed {
		ParseFlags()
	}

	// TODO set version on build.
	// v, _ := gitversion.Version()
	// log.Printf("watchmod version: %s", v)

	sort.Strings(C.ExcludeFiles) // must be sorted for search
	setStringRegexes()
	// log.Printf("Config: %+v\n", c)
	// log.Printf("WatchCommand: %+v\n", C.WatchCommand)
	// log.Printf("Exclude Files: %+v\n", C.ExcludeFiles)
	// log.Printf("Exclude Strings: %+v\n", C.ExcludeStrings)

	if !FC.Daemon {
		fmt.Println("Flag `daemon` set to false.  Running commands in config and exiting.")
		C.RunCmdOnStart = true
	}

	var expanded = make(map[string]string)
	for k, v := range C.WatchCommand {
		var err error
		// For windows slashes
		k, err = expandPath(os.ExpandEnv(k))
		if err != nil {
			panic(err)
		}

		v, err = expandPath(os.ExpandEnv(v))
		if err != nil {
			panic(err)
		}
		expanded[k] = v
		if C.RunCmdOnStart {
			runCmd(v)
		}
	}

	// Iterate over the map and print each key-value pair
	for key, value := range expanded {
		fmt.Printf("watching dir: %s, script: %s\n", key, value)
	}

	if !FC.Daemon {
		return
	}

	done := make(chan bool)
	for dir, cmd := range expanded {
		go Watch(dir, cmd)
	}
	<-done
}

// Watch is for each dir/cmd to watch/run.
func Watch(dir, cmd string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Write) {
					log.Printf("File changed: %+s, event: %s\n", event.Name, event.Op.String())
					fileName := filepath.Base(event.Name)
					if Excluded(fileName) {
						continue
					}
					runCmd(cmd)
				}
			case err := <-watcher.Errors:
				log.Fatal(err)
			}
		}
	}()

	err = watcher.Add(dir)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Done setting up watchmod for " + dir)

	// Block main goroutine forever.
	<-make(chan struct{})
}

// jError is for parsing errors from JSON.  If JSON, check for error by looking
// for `"success":false` and field "error" not nil.  i.e.
//
// {"success":false,"msg":"call had an error","path":"/admin/job/minPage"} Or
// {"error":"call had an error","path":"/admin/job/minPage"}
type jError struct {
	Success bool            `json:"success,omitempty" `
	Err     json.RawMessage `json:"error,omitempty" `
}

func runCmd(cmd string) {
	log.Printf("Start run %q\n", cmd)
	start := time.Now()

	commandOut := exec.Command("pwsh", cmd)
	stdoutStderr, err := commandOut.CombinedOutput()
	if err != nil {
		log.Printf("️ watchmod error: %s; On cmd: %s; Error: \n%s\n", err, cmd, stdoutStderr)
	} else if C.PrintStdOut && len(stdoutStderr) != 0 {
		log.Printf("%s", stdoutStderr)
	}

	// Check for JSON errors.
	je := new(jError)
	err = json.Unmarshal(stdoutStderr, je)
	if err == nil { // Ignore errors assuming command output is not JSON.
		if !je.Success || len(je.Err) != 0 {
			log.Printf("watchmod command error: %s\n%s\n\n", cmd, stdoutStderr)
		}
	}

	elapsed := time.Since(start)
	log.Printf("End   run %q in %s\n", cmd, elapsed)
}

// Essentially just a search function.
func excludeByFileName(fileName string) (excluded bool) {
	i := sort.SearchStrings(C.ExcludeFiles, fileName) // binary search
	if len(C.ExcludeFiles) == i {
		return false
	}
	if C.ExcludeFiles[i] != fileName { // Is the element the thing?  If not, it's new.
		return false
	}
	return true
}

func setStringRegexes() {
	for _, ext := range C.ExcludeStrings {
		escaped := regexp.QuoteMeta(ext)
		reg := regexp.MustCompile(escaped)
		regexes = append(regexes, reg)
	}
}

func matchExcludeString(filename string) (excluded bool) {
	for _, reg := range regexes {
		matched := reg.Match([]byte(filename))
		if matched {
			log.Printf("Input Matched, FileName and ExcludeExt: [%s], [%s]\n", filename, reg)
			return true
		}
	}
	return false
}

func Excluded(fileName string) (excluded bool) {
	excluded = excludeByFileName(fileName)
	if excluded {
		log.Printf("Excluded by file name: %s.  Doing nothing.  \n", fileName)
		return true
	}

	excluded = matchExcludeString(fileName)
	if excluded {
		log.Printf("Excluded by string: %s.  Doing nothing.  \n", fileName)
		return true
	}

	return false
}
