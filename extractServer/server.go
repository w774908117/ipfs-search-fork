package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
)

const (
	HOST = "localhost"
	PORT = "9999"
	TYPE = "tcp"
)

func main() {
	// get server information from env
	url, ok := os.LookupEnv("SERVER_URL")
	if !ok {
		url = HOST + ":" + PORT
	}
	listen, err := net.Listen(TYPE, url)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// close listener
	defer listen.Close()
	fmt.Println("Start listening on URL %s", url)
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		go handleIncomingRequest(conn)
	}
}
func handleIncomingRequest(c net.Conn) {
	fmt.Printf("Serving %s\n", c.RemoteAddr().String())
	for {
		dec := json.NewDecoder(c)
		cidFile := &WantedCID{}
		err := dec.Decode(cidFile)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Println(cidFile.Cid)
	}
	c.Close()
}
