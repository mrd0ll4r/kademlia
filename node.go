package kademlia

import (
	"net"
	"sync"
	"time"
)

const (
	k     = 3
	alpha = 2
)

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

// Peer is a remote node.
type Peer struct {
	ID   NodeID
	IP   net.IP
	Port uint16
}

// Node is the functionality a Kademlia node provides.
type Node interface {
	Ping(NodeID) error
	Store(NodeID, Key, []byte) error
	FindNode(NodeID) ([]Peer, error)
	FindValue(Key) ([]byte, error)
}

/*func NewNode() (Node,error) {

return &node{},nil
}*/

type node struct {
	// TODO fix this
	Addr     string
	ip       net.IP
	port     uint16
	laddr    *net.UDPAddr
	conns    map[NodeID]*net.UDPConn
	requests map[rpcID]chan []byte

	wg *sync.WaitGroup

	socket *net.UDPConn
	sync.RWMutex
}

func (n *node) bootsrap() error {
	return nil
}

func (n *node) mainLoop() error {
	udpAddr, err := net.ResolveUDPAddr("udp", n.Addr)
	if err != nil {
		return err
	}

	n.socket, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer n.socket.Close()

	for {
		buffer := make([]byte, 2048)
		n.socket.SetReadDeadline(time.Now().Add(time.Second))
		k, addr, err := n.socket.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				// A temporary failure is not fatal; just pretend it never happened.
				continue
			}
			return err
		}

		// We got nothin'
		if k == 0 {
			continue
		}

		n.wg.Add(1)
		go func() {
			defer n.wg.Done()

			if ip := addr.IP.To4(); ip != nil {
				addr.IP = ip
			}

			// check if this is a response packet
			header, err := decodePacketHeader(buffer)
			if err != nil {
				// TODO handle correctly
				panic(err)
			}

			switch header.packetType {
			case pingResponse:
				fallthrough
			case storeResponse:
				fallthrough
			case findNodeResponse:
				fallthrough
			case findValueResponse:
				// TODO check
				n.requests[header.rpcID] <- buffer
				return
			default:
			}

			// Handle the request.
			start := time.Now()
			// TODO handle request

			// TODO update kbuckets

			// TODO update n.conns
			_ = start
		}()
	}
}

func (n *node) doRPC(id NodeID, rpcID rpcID, packet []byte) ([]byte, error) {
	conn, ok := n.conns[id]
	if !ok {
		panic("problem!")
	}

	c := make(chan []byte)
	n.requests[rpcID] = c

	_, err := conn.Write(packet)
	if err != nil {
		return nil, err
	}

	resp := <-c

	return resp, nil
}
