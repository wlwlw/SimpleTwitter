package main

import (
	"log"
	"flag"
	"net"
	"strconv"

	"webserver"
)

var (
	serverAddress = flag.String("masterApp", "", "master StwServer host")
	port = flag.Int("port", 80, "port number to listen on")
)

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
}

func main() {
	flag.Parse()

	hostPort := net.JoinHostPort("localhost", strconv.Itoa(*port))
	_, err := webserver.NewWebServer(hostPort, *serverAddress)
	if err != nil {
		log.Fatalln("Server could not be created:", err)
	}

	select {}
}
