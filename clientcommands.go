package main

import (
	"os"
	"exec"
	"regexp"
	"io/ioutil"
)

func echoCommand(argv []string, confopts map[string]string) ([]string, os.Error) {
	return argv, nil
}

func execCommand(argv []string, confopts map[string]string) ([]string, os.Error) {
	reStr, ok := confopts["Allow"]
	env := confopts["Env"]

	if !ok {
		globalConfig.debug.DebugPrintf(3, "\"Allow\" not configured, defaulting to \".+\"")
		reStr = ".+"
	}

	re, e := regexp.Compile(reStr)
	if e != nil {
		return nil, e
	}

	newpath, e := exec.LookPath(argv[0])
	if e != nil {
		return []string{"Command not found"}, e
	}
	globalConfig.debug.DebugPrintf(3, "\tFound executable in $PATH: %s", newpath)

	if !re.MatchString(newpath) {
		globalConfig.debug.DebugPrintf(1, "\tReceived unallowed command \"%s\" (Rule: \"%s\")", newpath, confopts["Allow"])
		return []string{"Not allowed"}, os.NewError("Not allowed")
	}

	cmd_exec, e := exec.Run(newpath, argv, []string{env}, "/", exec.DevNull, exec.Pipe, exec.Pipe)
	if e != nil {
		return []string{"Could not execute"}, e
	}

	globalConfig.debug.DebugPrintf(2, "Executed %s: PID %d", argv[0], cmd_exec.Pid)
	cmd_exec.Wait(0)

	var output [2]string
	read, _ := ioutil.ReadAll(cmd_exec.Stdout)
	output[0] = string(read)
	read, _ = ioutil.ReadAll(cmd_exec.Stderr)
	output[1] = string(read)
	return &output, nil
}

func cpCommand(argv []string, confopts map[string]string) ([]string, os.Error) {
	src, e := os.Open(argv[0], os.O_RDONLY, 0)
	if e != nil {
		return nil, e
	}

	srcstat, e := src.Stat()
	if e != nil {
		return nil, e
	}

	dst, e := os.Open(argv[1], os.O_WRONLY|os.O_CREATE, srcstat.Permission())
	if e != nil {
		return nil, e
	}

	data, e := ioutil.ReadAll(src)
	if e != nil {
		return nil, e
	}

	_, e = dst.Write(data)
	if e != nil {
		return nil, e
	}

	return nil, nil

}
