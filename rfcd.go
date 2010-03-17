package main

import (
	"net"
	"fmt"
	"os"
	"bufio"
	"flag"
	"strings"
)

const (
	DEFAULT_CONF = "/etc/rcfd.conf"
)

type Command struct {
	name string
	command string
	args []string
}

type CmdParser struct {
	cmds map[string] Command
}

func createListener(addr string) net.Listener {
	tcpaddr, e := net.ResolveTCPAddr(strings.Split(addr, "\n", 2)[0])

	if e != nil {
		fmt.Fprintf(os.Stderr,"ResolveTCPAddr: %s\n", e)
		os.Exit(1)
	}

	socket, e := net.ListenTCP("tcp", tcpaddr)
	if e != nil {
		fmt.Fprintf(os.Stderr,"ListenTCP: %s\n", e)
		os.Exit(1)
	}

	return socket
}

func parseCmdLine() (string, bool) {
	file := flag.String("c", DEFAULT_CONF, "Path to configuration file")
	help := flag.Bool("h", false, "Print help")
	flag.Parse()

	return *file, *help
}

func NewCmdParser() (*CmdParser) {
	parser := new(CmdParser)
	parser.cmds = make(map[string] Command)
	return parser
}

func (c *CmdParser) AddCommand(cname, command string, args []string) {
	c.cmds[cname] = Command{cname, command, args}
}

func (c *CmdParser) GetCommand(cname string) (Command, bool) {
	cmd, ok := c.cmds[cname]
	return cmd, ok
}

func (c *CmdParser) ExecuteCommand(cname string) bool {
	cmd, ok := c.GetCommand(cname)
	if ok {
		_, e := os.ForkExec(cmd.GetCommand(), cmd.GetArgs(), nil, "/", nil)
		if e != nil {
			return false
		}
	}
	return ok
}

func (c *Command) GetName() string {
	return c.name
}

func (c *Command) GetCommand() string {
	return c.command
}

func (c *Command) GetArgs() []string {
	return c.args
}

func readConfig(file string) (string, *CmdParser) {
	parser := NewCmdParser()

	h, e := os.Open(file, os.O_RDONLY, 0)
	if e != nil {
		fmt.Fprintf(os.Stderr,"Open: %s\n", e)
		os.Exit(1)
	}
	r := bufio.NewReader(h)
	addr, e := r.ReadString('\n')
	line, e := r.ReadString('\n')
	for e != os.EOF {
		t := strings.Split(line, "=", 2)
		name := strings.TrimSpace(t[0])
		t = strings.Split(strings.TrimSpace(t[1])," ",0)
		cmd := t[0]
		args := t
		parser.AddCommand(name, cmd, args)
		line, e = r.ReadString('\n')
	}
	return addr, parser
}

func accepter(l net.Listener, c chan net.Conn) {
	for {
		i, _ := l.Accept()
		c<-i
	}
}

func clientHandler(parser *CmdParser, c net.Conn) {
	r := bufio.NewReader(c)
	s, e := r.ReadString('\n')
	s = strings.Split(s, "\n", 2)[0]
	for e != os.EOF {
		fmt.Printf("Recieving from %s\n", c.RemoteAddr())
		b := parser.ExecuteCommand(s)
		if b {
			fmt.Fprintf(c, "OK\n")
		} else {
			fmt.Fprintf(c, "ERR\n")
		}
		s, e = r.ReadString('\n')
		s = strings.Split(s, "\n", 2)[0]
	}
}

func main() {
	file, help := parseCmdLine()
	if help {
		fmt.Printf("Usage:\n")
		flag.PrintDefaults() ;
		os.Exit(0) ;
	}

	addr, parser := readConfig(file)

	listener := createListener(addr)
	clients := make(chan net.Conn)
	go accepter(listener, clients)
	for {
		c := <-clients
		go clientHandler(parser, c)
	}
}
