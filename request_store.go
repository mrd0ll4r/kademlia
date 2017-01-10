package kademlia

import (
	"errors"
	"sync"
	"time"
)

// errRPCIDInUse is returned if the rpcID is already in use.
var errRPCIDInUse = errors.New("rpcID already in use")

// errNoRPCID is returned for an unknown rpcID.
var errNoRPCID = errors.New("rpcID not prepared for a response")

// errTimeout is returned if await times out.
var errTimeout = errors.New("timed out")

// requestStore is a storage for open requests.
type requestStore interface {
	prepare(rpcID) error
	await(rpcID, time.Duration) (packet, error)
	abort(rpcID) error
	fulfill(rpcID, packet) error
}

type requestStoreImpl struct {
	sync.RWMutex
	requests map[rpcID]chan packet
}

func newRequestStore() requestStore {
	return &requestStoreImpl{
		requests: make(map[rpcID]chan packet, 1),
	}
}

func (s *requestStoreImpl) prepare(id rpcID) error {
	s.Lock()
	_, ok := s.requests[id]
	if ok {
		s.Unlock()
		return errRPCIDInUse
	}
	s.requests[id] = make(chan packet)
	s.Unlock()

	return nil
}

func (s *requestStoreImpl) abort(id rpcID) error {
	s.Lock()
	_, ok := s.requests[id]
	if !ok {
		s.Unlock()
		return errNoRPCID
	}
	delete(s.requests, id)
	s.Unlock()

	return nil
}

func (s *requestStoreImpl) await(id rpcID, timeout time.Duration) (packet, error) {
	s.RLock()
	c, ok := s.requests[id]
	s.RUnlock()
	if !ok {
		return nil, errNoRPCID
	}

	select {
	case p := <-c:
		s.Lock()
		delete(s.requests, id)
		s.Unlock()
		return p, nil
	case <-time.After(timeout):
		s.Lock()
		delete(s.requests, id)
		s.Unlock()

		// we have to check if there has been a response
		select {
		case p := <-c:
			return p, nil
		default:
		}

		return nil, errTimeout
	}
}

func (s *requestStoreImpl) fulfill(id rpcID, p packet) error {
	s.RLock()
	c, ok := s.requests[id]
	if !ok {
		return errNoRPCID
	}
	c <- p
	s.RUnlock()

	return nil
}
