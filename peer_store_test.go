package kademlia

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func pingMockSuccess(_ NodeID) error {
	return nil
}

func pingMockFail(NodeID) error {
	return errors.New("PingMockFail")
}

func TestPeer_Storage_Put_GetClose(t *testing.T) {
	// Create a new PeerStore
	id1 := NewNodeID()
	ps := newPeerStore(pingMockSuccess, id1)

	// Put new peer in PeerStore
	id2 := NewNodeID()
	p2 := Peer{ID: id2,
		IP:   net.IP{},
		Port: 1338,
	}

	err := ps.put(p2)
	require.Nil(t, err)

	// Get all closeBy Nodes for id2 (so itself)
	closeBy, err := ps.getClose(KeyFromBytes(id2.bytes()), 3)
	require.Nil(t, err)

	// Find id2 in returned peers
	found := false
	for _, p := range closeBy {
		if id2.Equal(p.ID) {
			found = true
		}
	}
	require.True(t, found, "node2 should be returned from closeBy(node2.ID)")

	// Put the same peer in the PeerStore again
	err = ps.put(p2)
	require.Nil(t, err)

	closeBy, err = ps.getClose(KeyFromBytes(id2.bytes()), 3)
	require.Nil(t, err)

	if len(closeBy) > 1 {
		t.Error("There should only be one returned peer")
		fmt.Println("Peers from close by:")
		for key, value := range closeBy {
			fmt.Printf("%d: %d\n", key, value.ID.bytes())
		}
	}

	/*
	 * Put 4 peers in one bucket and getClose for ID.
	 * 4th peer should not be in the bucket, since PingMock always return nil (success)
	 */

	// flip first byte of PeerStore ID and add 4 peers, 4th should not be in it (if ping works)
	var idFirstByte byte = 128
	if id1.bytes()[0] >= 128 {
		// 1st bit is 1
		idFirstByte = 0
	}

	idLayout := append(make([]byte, 0, 20), idFirstByte)
	idLayout = append(idLayout, id1.bytes()[1:]...)

	peers := make([]Peer, 0)
	peers = append(peers, Peer{ID: NodeIDFromBytes(idLayout),
		IP:   net.IP{},
		Port: 1339,
	})
	idLayout[0]++
	peers = append(peers, Peer{ID: NodeIDFromBytes(idLayout),
		IP:   net.IP{},
		Port: 1340,
	})
	idLayout[0]++
	peers = append(peers, Peer{ID: NodeIDFromBytes(idLayout),
		IP:   net.IP{},
		Port: 1341,
	})
	idLayout[0]++
	peers = append(peers, Peer{ID: NodeIDFromBytes(idLayout),
		IP:   net.IP{},
		Port: 1342,
	})

	for _, p := range peers {
		err = ps.put(p)
		require.Nil(t, err)
	}

	closeBy, err = ps.getClose(KeyFromBytes(idLayout), 4)
	require.Nil(t, err)

	require.True(t, len(closeBy) == 3)

	for _, p := range closeBy {
		if p.ID.Equal(peers[3].ID) {
			t.Error("PeerStore should not contain the 1st peer, because it should be droped, " +
				"since ping does not do anything right now")
		}
	}

	/*
	 * Put peer with ID close to PeerStore's.
	 * Use PingMockFail, so first peer gets kicked when 4th is put in
	 */
	ps = newPeerStore(pingMockFail, id1)

	id3 := id1
	// Invert 1st bit of last byte
	var mask byte = 1 << 7
	if id3[19]&mask == 0 {
		id3[19] &= mask
	} else {
		id3[19] &= (mask - 1)
	}
	peers = make([]Peer, 0)
	peers = append(peers, Peer{ID: NodeIDFromBytes(id3.bytes()),
		IP:   net.IP{},
		Port: 1339,
	})
	id3[19]++
	peers = append(peers, Peer{ID: NodeIDFromBytes(id3.bytes()),
		IP:   net.IP{},
		Port: 1340,
	})
	id3[19]++
	peers = append(peers, Peer{ID: NodeIDFromBytes(id3.bytes()),
		IP:   net.IP{},
		Port: 1341,
	})
	id3[19]++
	peers = append(peers, Peer{ID: NodeIDFromBytes(id3.bytes()),
		IP:   net.IP{},
		Port: 1342,
	})

	for _, p := range peers {
		err = ps.put(p)
		require.Nil(t, err)
	}

	// add the last peer a 2nd time
	err = ps.put(peers[3])
	require.Nil(t, err)

	// Get closeBy
	closeBy, err = ps.getClose(KeyFromBytes(id3.bytes()), 4)
	require.Nil(t, err)

	if len(closeBy) != 3 {
		t.Error("getClose returned to many peers (should return 3)")
	}

	found2ndPeer := false
	for _, p := range closeBy {
		if p.ID.Equal(peers[0].ID) {
			t.Error("PeerStore should not contain the 1st peer, because it should be droped, since ping does not do anything right now")
		}
		if p.ID.Equal(peers[1].ID) {
			found2ndPeer = true
		}
	}
	require.True(t, found2ndPeer, "2nd peer should not be droped, because last peer was added 2 times")
}
