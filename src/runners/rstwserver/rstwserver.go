// DO NOT MODIFY!

package main

import (
	"flag"
	"log"
	"net"
	"strconv"

	"stwserver"
)

var (
	port = flag.Int("port", 9010, "port number to listen on")
	masterServer = flag.String("master", "", "master appserver host port (if non-empty then this server is a slave)")
	masterStorageServer = flag.String("storageMaster", "", "master storage host port")
	numNodes       = flag.Int("N", 1, "the number of nodes in the cluster (including the master)")
)

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
}

func main() {
	flag.Parse()

	hostPort := net.JoinHostPort("localhost", strconv.Itoa(*port))
	_, err := stwserver.NewStwServer(hostPort, *masterServer, *masterStorageServer, *numNodes)
	if err != nil {
		log.Fatalln("Server could not be created:", err)
	}

	select {}
}
