package kademlia

import (
	"errors"
	"net"
	"sync"
)

// errDNE is returned for a missing value.
var errDNE = errors.New("does not exist")

// connStore is a storage for (NodeID, *net.UDPAddr) tuples.
type connStore interface {
	put(NodeID, *net.UDPAddr) error
	get(NodeID) (*net.UDPAddr, error)
}

type connStoreImpl struct {
	sync.RWMutex
	conns map[NodeID]*net.UDPAddr
}

func newConnStore() connStore {
	// TODO maybe implement some form of GC? Otherwise this is gonna grow forever...
	return &connStoreImpl{
		conns: make(map[NodeID]*net.UDPAddr),
	}
}

func (s *connStoreImpl) put(id NodeID, addr *net.UDPAddr) error {
	s.Lock()
	s.conns[id] = addr
	s.Unlock()
	return nil
}

func (s *connStoreImpl) get(id NodeID) (*net.UDPAddr, error) {
	s.RLock()
	addr, ok := s.conns[id]
	s.RUnlock()

	if !ok {
		return nil, errDNE
	}

	return addr, nil
}
