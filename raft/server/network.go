package main

import (
	"fmt"
	"log"
	"net"
)

const (
	MIN_PORT int = 1024
	MAX_PORT int = 8000
)

// Searches for the next available port on the host address
// starting from MIN_PORT=1024 to MAX_PORT=8000.
// The numbers MIN_PORT and MAX_PORT are completely arbitrary,
// use whatever you want.
func NewAddress(host string, port int) (string, int) {
	address := fmt.Sprintf("%v:%v", host, port)
	if portAvailable(address) {
		return host, port
	}

	for i := MIN_PORT; i < MAX_PORT; i++ {
		address := fmt.Sprintf("%v:%v", host, i)
		if portAvailable(address) {
			return host, i
		}
	}

	log.Fatalln("no free ports available on the system")
	return host, port
}

func portAvailable(address string) bool {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}
