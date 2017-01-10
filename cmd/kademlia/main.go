package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"

	"github.com/mrd0ll4r/kademlia"
)

var addr string

func init() {
	flag.StringVar(&addr, "a", "localhost:1337", "address to bind to")
}

func main() {
	flag.Parse()
	node, err := kademlia.NewNode(addr)
	if err != nil {
		log.Fatalln(err)
	}
	defer node.Stop()

	log.Println("Own ID:", node.ID())

	// We now have a node and can use it.
}
