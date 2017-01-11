package kademlia

import (
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestNode_Ping(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cfg1 := NodeConfig{
		ID:             NodeIDFromBytes([]byte("aaaaaaaaaaaaaaaaaaaa")),
		Addr:           "localhost:1337",
		BootstrapNodes: []string{"localhost:1338"},
	}
	cfg2 := NodeConfig{
		ID:             NodeIDFromBytes([]byte("bbbbbbbbbbbbbbbbbbbb")),
		Addr:           "localhost:1338",
		BootstrapNodes: []string{"localhost:1337"},
	}

	node1, err := NewNode(cfg1)
	require.Nil(t, err)
	defer node1.Stop()

	node2, err := NewNode(cfg2)
	require.Nil(t, err)
	defer node2.Stop()

	err = node1.Connect(cfg2.ID, cfg2.Addr)
	require.Nil(t, err)

	err = node1.Ping(cfg2.ID)
	require.Nil(t, err)

	err = node2.Ping(cfg1.ID)
	require.Nil(t, err)
}
