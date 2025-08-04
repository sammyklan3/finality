package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"strings"
	"time"

	"github.com/sammyklan3/finality/raft"
)

var (
	remote string
)

func init() {
	flag.StringVar(&remote, "remote", "", "Address of remote server to send request to")
	flag.Parse()

	if strings.TrimSpace(remote) == "" {
		flag.Usage()
		log.Fatalln("no remote provided")
	}
}

func sendNewRequest(msg string) (*raft.Reply, error) {
	client, err := rpc.Dial("tcp", remote)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	request := raft.Request{Msg: msg}
	reply := raft.Reply{}

	log.Println("sending new request to raft cluster...")
	err = client.Call("RaftServer.NewRequest", &request, &reply)
	if err != nil {
		return nil, err
	}

	return &reply, nil
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("\nenter msg to send: ")
		scanner.Scan()
		msg := scanner.Text()
		msg = strings.TrimSpace(msg)

		if msg == "" {
			log.Println("please enter non-empty valid input")
			continue
		}

		if strings.ToLower(msg) == "exit" {
			break
		}

		start := time.Now()

		reply, err := sendNewRequest(msg)
		if err != nil {
			log.Printf("error executing msg on raft cluster; %v\n", err)
			continue
		}

		elapsed := time.Since(start)
		log.Printf("result:\n\tmsg: %v\n\treply: %#v\n\tin %v\n", msg, reply, elapsed)
	}
	log.Println("exiting program")
}
