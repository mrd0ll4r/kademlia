package kademlia

import "sync"

// peerStore is a storage for Peers.
// This is the whole k-bucket thing.
type peerStore interface {
	put(Peer) error
	getClose(Key, int) ([]Peer, error)
}

type bucket struct {
	prefix []byte
	peers  []Peer
}

type peerStoreImpl struct {
	buckets []bucket
	sync.RWMutex
	peers    map[NodeID]Peer
	pingFunc func(NodeID) error

	// Own node's ID. We need this to split buckets.
	own NodeID
}

func newPeerStore(pingFunc func(NodeID) error) peerStore {
	return &peerStoreImpl{
		peers:    make(map[NodeID]Peer),
		pingFunc: pingFunc,
	}
}

func (s *peerStoreImpl) put(p Peer) error {
	s.Lock()
	s.peers[p.ID] = p
	s.Unlock()

	return nil
}

func (s *peerStoreImpl) getClose(k Key, n int) ([]Peer, error) {
	if n < 0 {
		n = 0
	}
	toReturn := make([]Peer, 0, n)
	s.RLock()
	for _, p := range s.peers {
		if n == 0 {
			break
		}
		toReturn = append(toReturn, p)
		n--
	}
	s.RUnlock()
	return toReturn, nil
}
