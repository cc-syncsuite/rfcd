package main

import (
	"flag"
	"io"
	"io/ioutil"
	"os"
	"json"
	"fmt"
	"strconv"
	"net"
	"time"
	"bufio"
	"strings"
)

const (
	DEFAULT_CONF = "/etc/rfcd.conf"
)

var (
	globalConfig rfcdConfig
)

// Config declarations
type rfcdConfig struct {
	BindAddr  string
	Port      int
	Verbosity int
	Delimiter string
	Separator string
	cmdlist   CommandList
}

func (c *rfcdConfig) GetSeparatorChar() byte { return c.Separator[0] }

func (c *rfcdConfig) GetDelimiterChar() byte { return c.Delimiter[0] }

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
	req.GetWriter().Write([]byte(s+globalConfig.Separator))
}

func (req *Request) DelimitEntity() {
	req.GetWriter().Write([]byte{globalConfig.GetDelimiterChar()})
}


// CommandList declarations
type CommandList struct {
	cmds map[string]Command
}

func (c *CommandList) InitCommandList() { c.cmds = make(map[string]Command) }

func (c *CommandList) RegisterCommand(keyword string, fp Command) {
	c.cmds[keyword] = fp
}

func (c *CommandList) GetCommand(keyword string) (fp Command, b bool) {
	fp, b = c.cmds[keyword]
	return
}

// Command declarations
type Command func(argv []string, req Request) os.Error

func echo_command(argv []string, req Request) os.Error {
	for i, k := range argv {
		req.WriteElement(fmt.Sprintf("%d = \"%s\"", i, k))
	}
	return nil
}

// Program functions

func isWhite(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t'
}

func myTrim(s string) string {
	var i,j int
	for i = 0; i < len(s) && isWhite(s[i]); i++ {}
	for j = len(s)-1; j>0 && isWhite(s[j]); j-- {}
	return s[i:j+1]
}

func panicOnError(msg string, e os.Error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", e)
		panic(msg)
	}
}

func debug(level int, msg string, a ...interface{}) {
	if globalConfig.Verbosity >= level {
		newfmt := fmt.Sprintf("DEBUG(%d): [%s] ", level, time.LocalTime())
		fmt.Printf(newfmt+msg+"\n", a)
	}
}

func parseCmdLine() string {
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
	return
}


func fillRequestChannel(listener net.Listener, reqChannel chan Request) {
	for {
		con, e := listener.Accept()
		if e == nil {
			req := NewRequest(con)
			debug(1, "%s: connected", req.GetRemoteAddr())
			reqChannel <- req
		}
	}
}

func setupServer(addr string) (chan Request, os.Error) {
	tcpAddr, e := net.ResolveTCPAddr(addr)
	if e == nil {
		listener, e := net.ListenTCP("tcp4", tcpAddr)
		debug(2, "Created listener. Error: %s", e)
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
		debug(1, "%s: Received \"%s\"", req.GetRemoteAddr(), entity)
		elems := strings.Split(entity[0:len(entity)-1], globalConfig.Separator, 0)

		if globalConfig.Verbosity >= 3 {
			for k, s := range elems {
				debug(3, "%s: Tokenlist: %d = \"%s\"", req.GetRemoteAddr(), k, s)
			}
		}
		cmd, ok := globalConfig.cmdlist.GetCommand(elems[0])
		debug(1, "%s: Command found: %t (%p)", req.GetRemoteAddr(), ok, cmd)
		if ok {
			req.WriteElement("OK")
			debug(2, "%s: Executing \"%s\"", req.GetRemoteAddr(), elems[0])
			cmd(elems[1:], req)
		} else {
			req.WriteElement("ERR")
		}
		req.DelimitEntity()
	}
}

func main() {
	configFile := parseCmdLine()

	config, e := readConfigFile(configFile)
	panicOnError("Reading configuration failed", e)

	globalConfig = config
	globalConfig.cmdlist.InitCommandList()
	debug(3, "Registered \"echo_command\"")
	globalConfig.cmdlist.RegisterCommand("echo", echo_command)

	debug(2, "Binding address: %s:%d", globalConfig.BindAddr, globalConfig.Port)
	reqChannel, e := setupServer(globalConfig.BindAddr + ":" + strconv.Itoa(globalConfig.Port))
	panicOnError("Opening server failed", e)

	for req := range reqChannel {
		go clientHandler(req)
	}
}
