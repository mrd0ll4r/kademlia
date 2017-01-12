package kademlia

import (
	"bytes"
	"errors"
	"sync"
)

// max length of bucket-prefix in bit
const m = 160

// peerStore is a storage for Peers.
// This is the whole k-bucket thing.
type peerStore interface {
	put(Peer) error
	getClose(Key, int) ([]Peer, error)
}

type bucket struct {
	// bin [0/1]
	prefix []byte
	// use when leaf = true is ordered: [k - 1] is least recently seen, [0] not seen for longest time
	peers []Peer
	// use when leaf = false
	buckets []bucket
	leaf    bool
}

type peerStoreImpl struct {
	buckets []bucket
	sync.RWMutex
	pingFunc func(NodeID) error
	// Own node's ID. We need this to split buckets
	own NodeID
}

func newPeerStore(pingFunc func(NodeID) error, id NodeID) peerStore {
	ps := peerStoreImpl{
		buckets:  make([]bucket, 2),
		pingFunc: pingFunc,
		own:      id,
	}

	var bits = bytesToBits(ps.own.bytes())

	p0 := []byte{0}
	p1 := []byte{1}

	/* Create buckets with prefix '0' and '1' and other recursive */
	ps.buckets[0].init(p0, bits)
	ps.buckets[1].init(p1, bits)

	return &ps
}

func (b *bucket) init(myPrefix []byte, nodePrefix []byte) error {
	b.prefix = myPrefix

	// Check if myPrefix is in nodePrefix
	var neighbor = bytes.Equal(myPrefix, nodePrefix[0:len(myPrefix)])

	// If max prefix length or k-bucket is far away from node
	if (len(myPrefix) == len(nodePrefix)) || (!neighbor) {
		b.leaf = true
		b.peers = make([]Peer, 0, k)
	} else {
		// k-bucket is close to node and has not the max prefix length
		b.leaf = false
		b.buckets = make([]bucket, 2)
		tmp0 := make([]byte, len(myPrefix)+1)
		tmp1 := make([]byte, len(myPrefix)+1)
		copy(tmp0, myPrefix)
		copy(tmp1, myPrefix)
		tmp0[len(myPrefix)] = 0
		tmp1[len(myPrefix)] = 1
		b.buckets[0].init(tmp0, nodePrefix)
		b.buckets[1].init(tmp1, nodePrefix)
	}

	return nil
}

func bytesToBits(b []byte) []byte {
	/* Convert prefix bytes to bits */
	bits := make([]byte, m)
	var mask byte = 128

	for i := 0; i < m; i++ {
		if (i % 8) == 0 {
			mask = 128
		}
		if (b[i/8] & mask) == 0 {
			bits[i] = 0
		} else {
			bits[i] = 1
		}
		mask >>= 1
	}

	return bits
}

func (b *bucket) findBucket(prefix []byte) *bucket {
	if b.leaf {
		return b
	}
	// If bucket prefix is not matching searched prefix
	if b.prefix[len(b.prefix)-1] != prefix[len(b.prefix)-1] {
		// Comparing only 1 bit is enough, because all bits before already got checked
		panic("bucket.prefix does not match the searched for prefix")
	}
	if prefix[len(b.prefix)] > 1 {
		panic("expected a binary, but found something else")
	}

	// Recursive lookup
	nextBit := prefix[len(b.prefix)]
	return b.buckets[nextBit].findBucket(prefix)
}

func (s *peerStoreImpl) put(p Peer) error {
	s.Lock()

	prefixBits := bytesToBits(p.ID.bytes())
	b := s.buckets[prefixBits[0]].findBucket(prefixBits)

	if b == nil {
		return errors.New("Could not find a bucket for peers prefix")
	}

	if len(b.peers) == k {
		// Bucket is full
		indexPeer := -1
		for i, curP := range b.peers {
			if curP.ID.Equal(p.ID) {
				indexPeer = i
			}
		}
		if indexPeer > -1 {
			// Bucket contains p
			b.peers = append(b.peers[:indexPeer], b.peers[indexPeer+1:]...) // Remove peer from slice
			b.peers = append(b.peers, p)                                    // Append peer
		} else {
			// Bucket does not contain p: Ping last peer in bucket
			if s.pingFunc(b.peers[0].ID) != nil {
				// Peer does not answer -> kick and move new peer to tail
				b.peers = append(b.peers[1:], p)
			} else {
				// Peer answered -> move to tail
				b.peers = append(b.peers[1:], b.peers[0])
			}
		}
	} else {
		// Bucket is not full
		if !containsPeer(b.peers, p) {
			b.peers = append(b.peers, p)
		}
	}

	s.Unlock()
	return nil
}

func (b *bucket) getClose(bits []byte, n int) []Peer {
	if b.leaf {
		return b.peers
	}
	nextBit := bits[len(b.prefix)]
	peers := b.buckets[nextBit].getClose(bits, n)

	missingPeers := n - len(peers)
	// Dept-first search
	if missingPeers > 0 {
		if nextBit == 0 {
			nextBit = 1
		} else {
			nextBit = 0
		}
		morePeers := b.buckets[nextBit].peers

		for _, p := range morePeers {
			peers = append(peers, p)
			missingPeers--
			if missingPeers == 0 {
				break
			}
		}
	}

	return peers
}

func (s *peerStoreImpl) getClose(k Key, n int) ([]Peer, error) {
	if n < 0 {
		n = 0
	}
	toReturn := make([]Peer, 0, n)

	if n > 0 {
		s.RLock()

		prefixBits := bytesToBits(k.bytes())
		toReturn = s.buckets[prefixBits[0]].getClose(prefixBits, n)

		s.RUnlock()
	}

	return toReturn, nil
}
