package kademlia

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"math/rand"
	"net"
	"sort"
)

const (
	k     = 3
	alpha = 2
)

// XorDist returns the XOR distance from a to b.
func XorDist(a [20]byte, b [20]byte) *big.Int {
	toReturn := big.NewInt(0)
	for i := 0; i < 20; i++ {
		res := a[19-i] ^ b[19-i]
		tmp := big.NewInt(int64(res))
		toReturn.Add(toReturn, tmp.Lsh(tmp, uint(8*i)))
	}
	return toReturn
}

// NodeID is the 160-bit identifier of a Node.
type NodeID [20]byte

// NodeIDFromBytes constructs a NodeID from a given byte-slice.
// It panics if the length of the slice is != 20.
func NodeIDFromBytes(b []byte) NodeID {
	if len(b) != 20 {
		panic("len")
	}

	var buf [20]byte
	copy(buf[:], b)
	return NodeID(buf)
}

func (n NodeID) bytes() []byte {
	return n[:]
}

// Dist returns the XOR distance from n to other.
func (n NodeID) Dist(other NodeID) *big.Int {
	return XorDist([20]byte(n), [20]byte(other))
}

// NewNodeID creates a new, random NodeID.
func NewNodeID() NodeID {
	toReturn := [20]byte{}
	n, err := rand.Read(toReturn[:])
	if err != nil {
		panic(err)
	}
	if n != 20 {
		panic("insufficient randomness")
	}
	return NodeID(toReturn)
}

// Key is a key of a key-value pair.
type Key [20]byte

// KeyFromBytes constructs a Key from a given byte-slice.
// It panics if the length of the slice is != 20.
func KeyFromBytes(b []byte) Key {
	if len(b) != 20 {
		panic("len")
	}

	var buf [20]byte
	copy(buf[:], b)
	return Key(buf)
}

// Dist returns the XOR distance from k to other.
func (k Key) Dist(other Key) *big.Int {
	return XorDist([20]byte(k), [20]byte(other))
}

func (k Key) bytes() []byte {
	return k[:]
}

// Peer is a remote node.
type Peer struct {
	ID   NodeID
	IP   net.IP
	Port uint16
}

func (p Peer) bytes() []byte {
	toReturn := make([]byte, 0, 22+net.IPv6len)
	toReturn = append(toReturn, p.ID[:]...)
	toReturn = append(toReturn, 0, 0)
	binary.BigEndian.PutUint16(toReturn[20:22], p.Port)
	toReturn = append(toReturn, p.IP.To16()...)

	return toReturn
}

func peerFromBytes(b []byte) Peer {
	if len(b) < 22+net.IPv4len {
		panic("len")
	}

	return Peer{
		ID:   NodeIDFromBytes(b[:20]),
		Port: binary.BigEndian.Uint16(b[20:22]),
		IP:   net.IP(b[22:]),
	}
}

func dedupPeers(peers []Peer) []Peer {
	toReturn := make([]Peer, 0, len(peers))
	for _, p := range peers {
		contained := false
		for i := range toReturn {
			if bytes.Equal(toReturn[i].ID.bytes(), p.ID.bytes()) {
				contained = true
				break
			}
		}
		if !contained {
			toReturn = append(toReturn, p)
		}
	}
	return toReturn
}

// sortablePeers is a wrapper to quicksort Peers by closeness to a key.
// It implements sorting.Interface
type sortablePeers struct {
	peers []Peer
	key   Key
}

func (s *sortablePeers) Len() int { return len(s.peers) }
func (s *sortablePeers) Less(i, j int) bool {
	return XorDist(s.peers[i].ID, s.key).Cmp(XorDist(s.peers[j].ID, s.key)) < 0
}
func (s *sortablePeers) Swap(i, j int) { tmp := s.peers[j]; s.peers[j] = s.peers[i]; s.peers[i] = tmp }

func sortPeers(peers []Peer, key Key) []Peer {
	sort.Sort(&sortablePeers{
		peers: peers,
		key:   key,
	})
	return peers
}

func containsPeer(peers []Peer, peer Peer) bool {
	pos := peerIndex(peers, peer)
	if pos >= len(peers) || !bytes.Equal(peers[pos].ID.bytes(), peer.ID.bytes()) {
		return false
	}
	return true
}

func peerIndex(peers []Peer, peer Peer) int {
	pos := sort.Search(len(peers), func(n int) bool {
		return bytes.Equal(peers[n].ID.bytes(), peer.ID.bytes())
	})

	return pos
}
