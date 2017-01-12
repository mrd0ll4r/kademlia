package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/mrd0ll4r/kademlia"
)

var addr string
var bootstrapNodes string
var debug bool
var pauseTimeString string
var pauseTime time.Duration

func init() {
	flag.StringVar(&pauseTimeString, "pause", "1s", "time to pause between requests")
	flag.BoolVar(&debug, "debug", false, "run in debug mode")
	flag.StringVar(&addr, "a", "localhost:1337", "address to bind to")
	flag.StringVar(&bootstrapNodes, "bootstrap", "localhost:1338,localhost:1339", "comma-separated addresses of bootstrap nodes")
}

func main() {
	flag.Parse()
	if debug {
		log.SetLevel(log.DebugLevel)
	}
	var err error
	pauseTime, err = time.ParseDuration(pauseTimeString)
	if err != nil {
		log.Fatalln(err)
	}

	storeKey := kademlia.KeyFromBytes([]byte("abababababababababab"))

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
	for i := 1; ; i++ {
		storeVal := fmt.Sprintf("%d - %d - %d", i, i, i)

		before := time.Now()
		err = node.Store(storeKey, []byte(storeVal))
		if err != nil {
			log.Errorln("Unable to store:", err)
			time.Sleep(pauseTime)
			continue
		}
		log.Infof("Store OK, stored %s, took %s", storeVal, time.Since(before))

		time.Sleep(pauseTime)

		before = time.Now()
		val, err := node.FindValue(storeKey)
		if err != nil {
			log.Errorln("Unable to find value:", err)
			time.Sleep(pauseTime)
			continue
		}

		log.Infof("FindValue OK, got: %s, took %s", string(val), time.Since(before))

		time.Sleep(pauseTime)
	}
}
