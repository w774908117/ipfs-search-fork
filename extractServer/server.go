package main

import (
	"encoding/json"
	"fmt"
	"github.com/libp2p/go-msgio"
	"log"
	"net"
	"os"
)

const (
	HOST = "0.0.0.0"
	PORT = "9999"
	TYPE = "tcp"
)

type tcpServer struct {
	writer msgio.Writer
}

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
	reader := msgio.NewReader(c)
	for {
		msg, err := reader.ReadMsg()
		if err != nil {
			log.Printf("Error at reading msg %s", err)
			continue
		}
		msgRecvd := &WantedCID{}
		err = json.Unmarshal(msg, msgRecvd)
		if err != nil {
			log.Printf("Failed unmarshal data %s", msg)
			continue
		}
		log.Printf(msgRecvd.Cid)

	}
	c.Close()
}
