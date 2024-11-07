package watchmod

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct { // Config options settable by config file.
	WatchCommand   map[string]string
	ExcludeFiles   []string
	ExcludeStrings []string
	RunCmdOnStart  bool
	PrintStdOut    bool
}

func parseConfig(i interface{}) {
	expand := os.ExpandEnv(FC.ConfigPath)
	fmt.Printf("Config path: %s\n", FC.ConfigPath)

	// For windows slashes
	expand, err := filepath.Abs(expand)
	if err != nil {
		panic(err)
	}

	file, err := os.Open(expand)
	if err != nil {
		panic(err)
	}

	// wrap reader before passing it to the json decoder for comment stripping
	decoder := json.NewDecoder(file)
	err = decoder.Decode(i)
	if err != nil {
		panic(err)
	}
}
