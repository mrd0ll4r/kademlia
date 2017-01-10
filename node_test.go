package kademlia

import (
	"testing"

	log "github.com/Sirupsen/logrus"
)

func TestNode_Ping(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	id1 := NodeIDFromBytes([]byte("aaaaaaaaaaaaaaaaaaaa"))
	addr1 := "localhost:1337"

	id2 := NodeIDFromBytes([]byte("bbbbbbbbbbbbbbbbbbbb"))
	addr2 := "localhost:1338"

	node1, err := newNodeWithFixedID(addr1, id1)
	if err != nil {
		t.Error(err)
	}
	defer node1.Stop()

	node2, err := newNodeWithFixedID(addr2, id2)
	if err != nil {
		t.Error(err)
	}
	defer node2.Stop()

	err = node1.Connect(id2, addr2)
	if err != nil {
		t.Error(err)
	}

	err = node1.Ping(id2)
	if err != nil {
		t.Error(err)
	}

	err = node2.Ping(id1)
	if err != nil {
		t.Error(err)
	}
}
