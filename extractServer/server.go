package main

import (
	"encoding/json"
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-msgio"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
)

const (
	HOST    = "0.0.0.0"
	PORT    = "9999"
	TYPE    = "tcp"
	SaveDir = "/out/"
)

type WantedCID struct {
	Cid      string `json:"cid"`
	FileType string `json:"type"`
}

func downloadFile(cid cid.Cid, saveDir string, gatewayUrl string) {
	log.Printf("Downloading cid %s", cid)
	// files that might be keys
	fileData, err := http.Get(fmt.Sprintf("%s/ipfs/%s", gatewayUrl, cid))
	if err != nil {
		log.Printf("Failed download cid %s", cid)
	}
	saveFile := path.Join(saveDir, cid.String())
	out, err := os.Create(saveFile)
	if err != nil {
		log.Printf("Failed create cid file %s", cid)
	}

	// Write the body to file
	_, err = io.Copy(out, fileData.Body)
	if err != nil {
		log.Printf("Failed create cid file %s", cid)
	}
	out.Close()
}

func main() {
	// get server information from env
	url, ok := os.LookupEnv("SERVER_URL")
	if !ok {
		url = HOST + ":" + PORT
	}

	gatewayUrl, ok := os.LookupEnv("IPFS_GATEWAY_URL")
	if !ok {
		gatewayUrl = "http://127.0.0.1:8080"
	}
	log.Printf("Gateway addrs %s", gatewayUrl)

	listen, err := net.Listen(TYPE, url)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// close listener
	defer listen.Close()
	log.Printf("Start listening on URL %s", url)
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		go handleIncomingRequest(conn, gatewayUrl)
	}
}

func handleIncomingRequest(c net.Conn, gatewayUrl string) {
	fmt.Printf("Serving %s\n", c.RemoteAddr().String())
	reader := msgio.NewReader(c)
	for {
		msg, err := reader.ReadMsg()
		if err != nil {
			if err == io.EOF {
				log.Printf("Recived EOF from connection")
				break
			}
			log.Printf("Error at reading msg %s", err)
			continue
		}
		msgRecvd := &WantedCID{}
		err = json.Unmarshal(msg, msgRecvd)
		if err != nil {
			log.Printf("Failed unmarshal data %s", msg)
			continue
		}
		log.Printf("Processing %s with type of %s", msgRecvd.Cid, msgRecvd.FileType)
		newCid, err := cid.Decode(msgRecvd.Cid)
		if err != nil {
			log.Printf("Invalid cid %s", msgRecvd.Cid)
			continue
		}
		// download cid
		downloadFile(newCid, SaveDir, gatewayUrl)
	}
	c.Close()
}
