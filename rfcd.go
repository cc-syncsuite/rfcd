package main

import (
	"net"
	"fmt"
	"os"
	"bufio"
	"flag"
)

const (
	DEFAULT_CONF = "/etc/rcfd.conf"
)

func createListener(addr string) net.Listener {
	tcpaddr, e := net.ResolveTCPAddr(addr)

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

func readConfig(file string) string {
	h, e := os.Open(file, os.O_RDONLY, 0)
	if e != nil {
		fmt.Fprintf(os.Stderr,"Open: %s\n", e)
		os.Exit(1)
	}
	s, _ := bufio.NewReader(h).ReadString('\n')
	return s
}

func main() {
	file, help := parseCmdLine()
	if help {
		flag.PrintDefaults() ;
		os.Exit(0) ;
	}

	fmt.Printf("%s\n", file)
	addr := readConfig(file)
	fmt.Printf("%s\n", addr)
//	l := createListener(addr)
}
