package kademlia

import "sync"

// valueStore is a storage for (Key, []byte) tuples.
type valueStore interface {
	put(Key, []byte) error
	get(Key) ([]byte, error)
}

type valueStoreImpl struct {
	sync.RWMutex
	values map[Key][]byte
}

func newValueStore() valueStore {
	// TODO implement GC
	return &valueStoreImpl{
		values: make(map[Key][]byte),
	}
}

func (s *valueStoreImpl) put(k Key, v []byte) error {
	s.Lock()
	s.values[k] = v
	s.Unlock()

	return nil
}

func (s *valueStoreImpl) get(k Key) ([]byte, error) {
	s.RLock()
	v, ok := s.values[k]
	s.RUnlock()

	if !ok {
		return nil, errDNE
	}

	return v, nil
}
