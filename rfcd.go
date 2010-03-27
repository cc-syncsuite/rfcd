package main

import (
	"flag"
	"io"
	"io/ioutil"
	"os"
	"json"
	"strconv"
	"net"
	"bufio"
	"strings"
	"regexp"
	"exec"
	"surmc"
)

const (
	DEFAULT_CONF = "/etc/rfcd.conf"
)

var (
	globalConfig rfcdConfig
	builtins     = map[string]CommandFunc{
		"echo": echoCommand,
		"exec": execCommand,
		"cp":   cpCommand,
	}
)

// Config declarations
type rfcdConfig struct {
	BindAddr       string
	Port           int
	Verbosity      int
	Delimiter      string
	Separator      string
	CommandConfigs []CommandConfig
	parsed         map[string]Command
	debug          surmc.Debug
}

func (c *rfcdConfig) GetSeparatorChar() byte { return c.Separator[0] }

func (c *rfcdConfig) GetDelimiterChar() byte { return c.Delimiter[0] }

func (c *rfcdConfig) getCommandConfig(keyword string) (*CommandConfig, os.Error) {
	for _, cc := range c.CommandConfigs {
		if cc.CommandName == keyword {
			return &cc, nil
		}
	}
	return nil, os.NewError("Not a valid key")
}

func (c *rfcdConfig) RegisterCommand(keyword string, fp CommandFunc) {
	if c.parsed == nil {
		c.parsed = make(map[string]Command)
	}
	cc, _ := c.getCommandConfig(keyword)
	opts := stringSliceToMap(cc.CommandParams, ":")
	if c.Verbosity >= 4 {
		globalConfig.debug.DebugPrintf(4, "\t\"%s\" opts:", keyword)
		for key, val := range opts {
			globalConfig.debug.DebugPrintf(4, "\t\t\"%s\" => \"%s\"", key, val)
		}
	}
	c.parsed[strings.ToLower(keyword)] = Command{keyword, fp, opts}
}

func (c *rfcdConfig) GetCommand(keyword string) (cmd Command, ok bool) {
	cmd, ok = c.parsed[strings.ToLower(keyword)]
	return
}

// CommandConfig declaration
type CommandConfig struct {
	CommandName   string
	CommandParams []string
}

// Request declarations
type Request struct {
	con net.Conn
}

func NewRequest(con net.Conn) (r Request) {
	r.con = con
	return
}

func (req *Request) GetWriter() io.Writer { return req.con }

func (req *Request) GetReader() io.Reader { return req.con }

func (req *Request) GetRemoteAddr() string { return req.con.RemoteAddr().String() }

func (req *Request) WriteElement(s string) {
	req.GetWriter().Write([]byte(s + globalConfig.Separator))
}

func (req *Request) DelimitEntity() {
	req.GetWriter().Write([]byte{globalConfig.GetDelimiterChar()})
}

// CommandInternals declarations
type Command struct {
	cmd      string
	fp       CommandFunc
	confopts map[string]string
}

// Command declarations
type CommandFunc func(argv []string, confopts map[string]string) ([]string, os.Error)

func echoCommand(argv []string, confopts map[string]string) ([]string, os.Error) {
	return argv, nil
}

func execCommand(argv []string, confopts map[string]string) ([]string, os.Error) {
	reStr, ok := confopts["Allow"]
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

	cmd_exec, e := exec.Run(newpath, argv, nil, "/", exec.DevNull, exec.Pipe, exec.Pipe)
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

// Program functions

func isWhite(b byte) bool { return b == ' ' || b == '\n' || b == '\t' }

func myTrim(s string) string {
	var i, j int
	for i = 0; i < len(s) && isWhite(s[i]); i++ {
	}
	for j = len(s) - 1; j > 0 && isWhite(s[j]); j-- {
	}
	return s[i : j+1]
}

func parseCmdLine() string {
	file := flag.String("c", DEFAULT_CONF, "Path to configuration file")
	flag.Parse()

	return *file
}

func stringSliceToMap(strslice []string, sep string) (ret map[string]string) {
	ret = make(map[string]string)
	for _, mapentry := range strslice {
		elems := strings.Split(mapentry, sep, 2)
		ret[elems[0]] = elems[1]
	}
	return
}

func readConfig(r io.Reader) (rfcdConfig, os.Error) {
	var config rfcdConfig
	bRead, error := ioutil.ReadAll(r)
	if error == nil {
		ok, errTok := json.Unmarshal(string(bRead), &config)
		if !ok {
			error = os.NewError("Offending Token in config file: " + errTok)
		}
	}
	return config, error
}

func readConfigFile(file string) (config rfcdConfig, error os.Error) {
	f, error := os.Open(file, os.O_RDONLY, 0)
	if error == nil {
		config, error = readConfig(f)
	}
	config.debug.Level = config.Verbosity
	return
}


func fillRequestChannel(listener net.Listener, reqChannel chan Request) {
	for {
		con, e := listener.Accept()
		if e == nil {
			req := NewRequest(con)
			globalConfig.debug.DebugPrintf(1, "%s: connected", req.GetRemoteAddr())
			reqChannel <- req
		}
	}
}

func setupServer(addr string) (chan Request, os.Error) {
	tcpAddr, e := net.ResolveTCPAddr(addr)
	if e == nil {
		listener, e := net.ListenTCP("tcp4", tcpAddr)
		globalConfig.debug.DebugPrintf(2, "Created listener. Error: %s", e)
		if e == nil {
			reqChannel := make(chan Request, 5)
			go fillRequestChannel(listener, reqChannel)
			return reqChannel, e
		}
		return nil, e
	}
	return nil, e
}

func clientHandler(req Request) {
	br := bufio.NewReader(req.GetReader())
	for entity, e := br.ReadString(globalConfig.GetDelimiterChar()); e == nil; entity, e = br.ReadString(globalConfig.GetDelimiterChar()) {
		entity = myTrim(entity)
		globalConfig.debug.DebugPrintf(1, "%s: Received \"%s\"", req.GetRemoteAddr(), entity)
		elems := strings.Split(entity[0:len(entity)-1], globalConfig.Separator, 0)
		elems = elems[0:len(elems)-1]

		if globalConfig.Verbosity >= 3 {
			for k, s := range elems {
				globalConfig.debug.DebugPrintf(3, "%s: Tokenlist: %d = \"%s\"", req.GetRemoteAddr(), k, s)
			}
		}
		cmd, ok := globalConfig.GetCommand(elems[0])
		globalConfig.debug.DebugPrintf(1, "%s: Command found: %t (%p)", req.GetRemoteAddr(), ok, cmd.fp)
		if ok {
			globalConfig.debug.DebugPrintf(2, "%s: Executing \"%s\"", req.GetRemoteAddr(), cmd.cmd)
			fields, e := cmd.fp(elems[1:], cmd.confopts)
			if e != nil {
				req.WriteElement("ERR")
				globalConfig.debug.DebugPrintf(1, "%s: Executing \"%s\" failed! %s", req.GetRemoteAddr(), cmd.cmd, e)
			} else {
				req.WriteElement("OK")
			}
			for _, field := range fields {
				req.WriteElement(field)
			}
		} else {
			req.WriteElement("ERR")
		}
		req.DelimitEntity()
	}
}

func main() {
	configFile := parseCmdLine()

	config, e := readConfigFile(configFile)
	surmc.PanicOnError(e, "Reading configuration failed")

	globalConfig = config
	for _, cmd := range globalConfig.CommandConfigs {
		lowered := strings.ToLower(cmd.CommandName)
		globalConfig.debug.DebugPrintf(3, "Registered \"%s\"-Command", lowered)
		fp, e := builtins[lowered]
		if !e {
			surmc.PanicOnError(os.NewError(""), "Unknown command \"%s\"", lowered)
		}
		globalConfig.RegisterCommand(lowered, fp)
	}

	globalConfig.debug.DebugPrintf(2, "Binding address: %s:%d", globalConfig.BindAddr, globalConfig.Port)
	reqChannel, e := setupServer(globalConfig.BindAddr + ":" + strconv.Itoa(globalConfig.Port))
	surmc.PanicOnError(e, "Opening server failed")

	for req := range reqChannel {
		go clientHandler(req)
	}
}
