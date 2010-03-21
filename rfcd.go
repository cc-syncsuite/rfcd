package main

import (
	"bytes"
	"net"
	"fmt"
	"os"
	"exec"
	"bufio"
	"io"
	"flag"
	"strings"
	"time"
	"strconv"
)

const (
	DEFAULT_CONF = "/etc/rfcd.conf"
)

/*
BUG(asu): If client closes connection, before answer has been sent,
	programm crashes silently
*/

type Command struct {
	name    string
	numargs int
	command string
}

type CmdParser struct {
	cmds map[string]Command
}

func createListener(addr string) net.Listener {
	tcpaddr, e := net.ResolveTCPAddr(strings.Split(addr, "\n", 2)[0])

	if e != nil {
		fmt.Fprintf(os.Stderr, "ResolveTCPAddr: %s\n", e)
		os.Exit(1)
	}

	socket, e := net.ListenTCP("tcp", tcpaddr)
	if e != nil {
		fmt.Fprintf(os.Stderr, "ListenTCP: %s\n", e)
		os.Exit(1)
	}

	return socket
}

func readConfig(file string) (string, *CmdParser) {
	parser := NewCmdParser()

	h, e := os.Open(file, os.O_RDONLY, 0)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Open: %s\n", e)
		os.Exit(1)
	}
	r := bufio.NewReader(h)
	addr, e := r.ReadString('\n')
	addr = strings.Split(addr, "\n", 0)[0] //remove trailing newline
	for line, e := r.ReadString('\n'); e == nil; line, e = r.ReadString('\n') {
		line = strings.Split(line, "\n", 0)[0] // remove trailing newline
		split := strings.Split(line, "=", 2)
		parms := strings.Split(split[0], ",", 0)
		if len(parms) < 2 || len(split) != 2 {
			fmt.Printf("Skipped line\n")
			continue
		}
		n, e := strconv.Atoi(strings.TrimSpace(parms[1]))
		if e != nil {
			fmt.Fprintf(os.Stderr, "Atoi: %s\n", e)
			os.Exit(1)
		}
		parser.AddCommand(strings.TrimSpace(parms[0]),
			n,
			strings.TrimSpace(split[1]))
	}
	return addr, parser
}

func parseCmdLine() (string, bool) {
	file := flag.String("c", DEFAULT_CONF, "Path to configuration file")
	help := flag.Bool("h", false, "Print help")
	flag.Parse()

	return *file, *help
}

func NewCmdParser() *CmdParser {
	parser := new(CmdParser)
	parser.cmds = make(map[string]Command)
	return parser
}

func (c *CmdParser) AddCommand(cname string, numargs int, command string) {
	c.cmds[cname] = Command{cname, numargs, command}
}

func (c *CmdParser) GetCommand(cname string) (Command, bool) {
	cmd, ok := c.cmds[cname]
	return cmd, ok
}

func dump(out chan byte, in io.Reader) {
	b := make([]byte, 1)
	for _, e := in.Read(b); e == nil; _, e = in.Read(b) {
		out <- b[0]
	}
}

func (c *CmdParser) ExecuteCommand(cname string, args []string, output chan byte) (bool, string) {
	cmd, ok := c.GetCommand(cname)
	if ok {
		if len(args) != cmd.GetNumArgs() {
			return false, "Arg count mismatch"
		}
		cmdpath, e := exec.LookPath(cmd.GetCommand())
		if e != nil {
			return false, e.String()
		}
		nargs := strings.Split(cmdpath+","+strings.Join(args, ","), ",", 0)
		proc, e := exec.Run(cmdpath, nargs, nil, "/", exec.DevNull, exec.Pipe, exec.Pipe)
		if e != nil {
			return false, e.String()
		}
		go func() {
			dump(output, bytes.NewBufferString("> Errors:\n"))
			dump(output, proc.Stderr)
			dump(output, bytes.NewBufferString("> Output:\n"))
			dump(output, proc.Stdout)
			close(output)
		}()
	}
	return ok, "Command not configured"
}

func (c *Command) GetName() string { return c.name }

func (c *Command) GetCommand() string { return c.command }

func (c *Command) GetNumArgs() int { return c.numargs }

func accepter(l net.Listener, c chan net.Conn) {
	for {
		i, e := l.Accept()
		if e == nil {
			fmt.Printf("[%s] %s connected\n", time.LocalTime(), i.RemoteAddr())
			c <- i
		}
	}
}

func splitCmd(s string) (cmd string, args []string) {
	t := strings.Split(s, "\n", 2)
	t = strings.Split(t[0], ",", 0)
	cmd = t[0]
	args = t[1:]
	return
}

func clientHandler(parser *CmdParser, c net.Conn) {
	r := bufio.NewReader(c)
	for s, e := r.ReadString('\n'); e == nil; s, e = r.ReadString('\n') {
		cmd, args := splitCmd(s)

		fmt.Printf("\tReceived command: \"%s\"(", cmd)
		for _, arg := range args {
			fmt.Printf("\"%s\" ", arg)
		}
		fmt.Printf(")\n")

		output := make(chan byte)
		b, s := parser.ExecuteCommand(cmd, args, output)

		if !b {
			fmt.Fprintf(c, "ERR\n%s\n", s)
		} else {
			fmt.Fprintf(c, "OK\n")
			fmt.Printf("\tSending answer...\n")
			var ok os.Error
			for _byte := range output {
				if ok == nil {
					_,ok = c.Write([]byte{_byte})
				}
			}
		}
	}
}

func main() {
	file, help := parseCmdLine()
	if help {
		fmt.Printf("Usage:\n")
		flag.PrintDefaults()
		os.Exit(0)
	}
	addr, parser := readConfig(file)
	listener := createListener(addr)
	clients := make(chan net.Conn)
	go accepter(listener, clients)
	for c := range clients {
		go clientHandler(parser, c)
	}
}
