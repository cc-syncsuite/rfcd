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

type rfcdConfig struct {
	BindAddr  string
	Port      int
	Verbosity int
	Delimiter string
	Separator string
}

func (c *rfcdConfig) GetSeparatorChar() byte { return c.Separator[0] }

func (c *rfcdConfig) GetDelimiterChar() byte { return c.Delimiter[0] }

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
			debug(1, "%s connected", req.GetRemoteAddr())
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
		debug(1, "%s: Received \"%s\"", req.GetRemoteAddr(), entity)
		elems := strings.Split(entity[0:len(entity)-1], globalConfig.Separator, 0)
		for k, s := range elems {
			debug(2, "%s: Tokenlist: %d = \"%s\"", req.GetRemoteAddr(), k, s)
		}
	}
}

func main() {
	configFile := parseCmdLine()

	config, e := readConfigFile(configFile)
	globalConfig = config
	debug(2, "Binding address: %s:%d", globalConfig.BindAddr, globalConfig.Port)
	panicOnError("Reading configuration failed", e)

	reqChannel, e := setupServer(globalConfig.BindAddr + ":" + strconv.Itoa(globalConfig.Port))
	panicOnError("Opening server failed", e)
	for req := range reqChannel {
		go clientHandler(req)
	}
}
