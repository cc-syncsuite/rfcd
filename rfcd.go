package main

import (
	"flag"
	"io"
	"io/ioutil"
	"os"
	"json"
	"fmt"
)

const (
	DEFAULT_CONF = "/etc/rfcd.conf"
)

var (
	globalConfig rfcdConfig
)


type rfcdConfig struct {
	BindAddr string
	Port int
	Debug int
}

func parseCmdLine() (string) {
	file := flag.String("c", DEFAULT_CONF, "Path to configuration file")
	flag.Parse()

	return *file
}

func readConfig(r io.Reader) (rfcdConfig, os.Error) {
	var config rfcdConfig
	bRead, error := ioutil.ReadAll(r)
	if error == nil {
		ok, errTok := json.Unmarshal(string(bRead), &config)
		if !ok {
			error = os.NewError("Offending Token in config file: "+errTok)
		}
	}
	return config, error
}

func readConfigFile(file string) (config rfcdConfig, error os.Error) {
	f, error := os.Open(file, os.O_RDONLY, 0)
	if error == nil {
		config, error = readConfig(f)
	}
	return
}

func panicOnError(msg string, e os.Error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", e)
		panic(msg)
	}
}

func main() {
	configFile := parseCmdLine()

	globalConfig, e := readConfigFile(configFile)
	panicOnError("Reading configuration failed", e)
}
