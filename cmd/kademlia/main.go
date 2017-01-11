package main

import (
	"flag"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/mrd0ll4r/kademlia"
)

var addr string
var bootstrapNodes string

func init() {
	flag.StringVar(&addr, "a", "localhost:1337", "address to bind to")
	flag.StringVar(&bootstrapNodes, "bootstrap", "localhost:1338,localhost:1339", "comma-separated addresses of bootstrap nodes")
}

func main() {
	flag.Parse()
	node, err := kademlia.NewNode(kademlia.NodeConfig{
		ID:             kademlia.NewNodeID(),
		Addr:           addr,
		BootstrapNodes: strings.Split(bootstrapNodes, ","),
	})
	if err != nil {
		log.Fatalln(err)
	}
	defer node.Stop()

	log.Printf("Own ID: %x", node.ID())

	// We now have a node and can use it.
}
