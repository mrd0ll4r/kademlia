package main

import (
	"flag"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/mrd0ll4r/kademlia"
)

var addr string
var bootstrapNodes string
var debug bool

func init() {
	flag.BoolVar(&debug, "debug", false, "run in debug mode")
	flag.StringVar(&addr, "a", "localhost:1337", "address to bind to")
	flag.StringVar(&bootstrapNodes, "bootstrap", "localhost:1338,localhost:1339", "comma-separated addresses of bootstrap nodes")
}

func main() {
	flag.Parse()
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	node, err := kademlia.NewNode(kademlia.NodeConfig{
		ID:             kademlia.NodeIDFromBytes([]byte("00000000000000000000")),
		Addr:           addr,
		BootstrapNodes: strings.Split(bootstrapNodes, ","),
	})
	if err != nil {
		log.Fatalln(err)
	}
	defer node.Stop()

	log.Printf("Own ID: %x", node.ID())

	// We now have a node and can use it.

	time.Sleep(3 * time.Second)
	for {
		before := time.Now()
		err = node.Ping(kademlia.NodeIDFromBytes([]byte("11111111111111111111")))
		if err != nil {
			log.Errorln("Unable to ping:", err)
		} else {
			log.Infof("Ping OK, took %s", time.Since(before))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
