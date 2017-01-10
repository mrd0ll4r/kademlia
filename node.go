package kademlia

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

// Node is the functionality a Kademlia node provides.
type Node interface {
	Ping(NodeID) error
	Store(NodeID, Key, []byte) error
	FindNode(NodeID) ([]Peer, error)
	FindValue(Key) ([]byte, error)
	Addr() string
	ID() NodeID
	Stop()
	Connect(id NodeID, addr string) error
}

// TODO make contructors accept a config
// TODO config must contain: NodeID, Addr, bootstrap nodes, possibly other settings

// NewNode creates a new Node with a random ID.
func NewNode(addr string) (Node, error) {
	return newNodeWithFixedID(addr, NewNodeID())
}

// NewNodeWithFixedID creates a new Node with the given ID.
func newNodeWithFixedID(addr string, id NodeID) (Node, error) {
	toReturn := &node{
		addr:         addr,
		own:          id,
		connStore:    newConnStore(),
		valueStore:   newValueStore(),
		requestStore: newRequestStore(),
		wg:           &sync.WaitGroup{},
		startErr:     make(chan error),
		started:      make(chan struct{}),
		shutdown:     make(chan struct{}),
		shutdownDone: make(chan struct{}),
	}

	toReturn.peerStore = newPeerStore(toReturn.Ping)

	go func() {
		err := toReturn.mainLoop()
		if err != nil {
			log.Errorln("main loop failed:", err)
			return
		}
		log.Debugln("main loop shut down.")
	}()

	// we need to wait until the main loop is ready to receive responses
	// before we bootstrap.
	select {
	case <-toReturn.started:
		// all good
	case err := <-toReturn.startErr:
		return nil, err
	}

	err := toReturn.bootsrap()
	if err != nil {
		return nil, err
	}

	return toReturn, nil
}

type node struct {
	addr         string
	connStore    connStore
	peerStore    peerStore
	requestStore requestStore
	valueStore   valueStore
	wg           *sync.WaitGroup
	own          NodeID
	socket       *net.UDPConn

	startErr chan error
	started  chan struct{}

	shutdown     chan struct{}
	shutdownDone chan struct{}
}

// Addr returns the address (=endpoint) of the node.
func (n *node) Addr() string {
	return n.addr
}

// ID returns the ID of the node.
func (n *node) ID() NodeID {
	return n.own
}

func (n *node) Stop() {
	select {
	case <-n.shutdown:
		return
	default:
	}

	close(n.shutdown)
	<-n.shutdownDone
}

func (n *node) Connect(id NodeID, addr string) error {
	uaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	err = n.connStore.put(id, uaddr)
	if err != nil {
		return err
	}
	return n.Ping(id)
}

func (n *node) Ping(id NodeID) error {
	p := newPingRequest(n.own, id, n.nextRPCID())

retry:
	before := time.Now()
	resp, err := n.doRPC(p)
	if err != nil && err == errRPCIDInUse {
		p.rpcID = n.nextRPCID()
		goto retry
	} else if err != nil {
		return err
	}

	if resp.packetType() != pingResponse {
		return fmt.Errorf("Expected ping response, got %d", resp.packetType())
	}
	log.Debugf("Ping took %s", time.Since(before))

	return nil
}

func (n *node) Store(NodeID, Key, []byte) error {
	return nil
}
func (n *node) FindNode(NodeID) ([]Peer, error) {
	return []Peer{}, nil
}
func (n *node) FindValue(Key) ([]byte, error) {
	return []byte{}, nil
}

func (n *node) bootsrap() error {
	// TODO contact boostrap nodes
	// TODO look up own ID
	// TODO populate peerStore
	return nil
}

func (n *node) nextRPCID() rpcID {
	buf := [20]byte{}
	read, err := rand.Read(buf[:])
	if err != nil {
		panic(err)
	}
	if read != 20 {
		panic("insufficient randomness")
	}
	return rpcID(buf)
}

func (n *node) mainLoop() error {
	udpAddr, err := net.ResolveUDPAddr("udp", n.addr)
	if err != nil {
		n.startErr <- err
		return err
	}

	n.socket, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		n.startErr <- err
		return err
	}
	defer n.socket.Close()

	close(n.started)

	for {
		select {
		case <-n.shutdown:
			n.wg.Wait()
			close(n.shutdownDone)
			return nil
		default:
		}

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
		buffer = buffer[:k]

		log.Debugf("Got packet. Length: %d, content: %x", k, buffer)

		n.wg.Add(1)
		go func() {
			defer n.wg.Done()

			if ip := addr.IP.To4(); ip != nil {
				addr.IP = ip
			}

			p, err := parsePacket(buffer)
			if err != nil {
				log.Errorln("unable to parse incoming packet:", err)
				return
			}

			switch p.packetType() {
			case pingResponse:
				fallthrough
			case storeResponse:
				fallthrough
			case findNodeResponse:
				fallthrough
			case findValueResponse:
				err = n.requestStore.fulfill(p.id(), p)
				if err != nil {
					log.Errorf("Got response of type %d for unknown rpcID %x", p.packetType(), p.id())
					return
				}

				// update kbuckets
				err = n.peerStore.put(Peer{ID: p.origin(), IP: addr.IP, Port: uint16(addr.Port)})
				if err != nil {
					log.Errorln("Unable to update peer store:", err)
				}

				// update n.conns
				err = n.connStore.put(p.origin(), addr)
				if err != nil {
					log.Errorln("Unable to update connections:", err)
				}
				return
			default:
			}

			// Handle the request.
			start := time.Now()
			var resp packet

			switch p.packetType() {
			case pingRequest:
				resp = newPingResponse(n.own, p.origin(), p.id())
			case storeRequest:
				sp, ok := p.(storeRequestPacket)
				if !ok {
					panic("store request packet does not coerce to storeRequestPacket")
				}
				err := n.valueStore.put(sp.key, sp.value)
				if err != nil {
					log.Errorln("Unable to store value:", err)
					resp = newStoreResponsePacket(n.own, p.origin(), p.id(), 1)
					break
				}
				resp = newStoreResponsePacket(n.own, p.origin(), p.id(), 0)
			case findNodeRequest:
				fp, ok := p.(findNodeRequestPacket)
				if !ok {
					panic("find node request packet does not coerce to findNodeRequestPacket")
				}
				nodes, err := n.peerStore.getClose(Key(fp.nodeID), k)
				if err != nil {
					log.Errorln("Unable to get close peers:", err)
					resp = newFindNodeResponsePacket(n.own, p.origin(), p.id(), []Peer{})
					break
				}
				resp = newFindNodeResponsePacket(n.own, p.origin(), p.id(), nodes)
			case findValueRequest:
				fp, ok := p.(findValueRequestPacket)
				if !ok {
					panic("find value request packet does not coerce to findValudRequestPacket")
				}

				val, err := n.valueStore.get(fp.key)
				if err != nil && err != errDNE {
					log.Errorln("Unable to get value:", err)
					resp = newFindValueResponsePacketWithNodes(n.own, p.origin(), p.id(), []Peer{})
					break
				} else if err == errDNE {
					// don't have value, return peers
					nodes, err := n.peerStore.getClose(fp.key, k)
					if err != nil {
						log.Errorln("Unableto get close peers:", err)
						resp = newFindValueResponsePacketWithNodes(n.own, p.origin(), p.id(), []Peer{})
						break
					}
					resp = newFindValueResponsePacketWithNodes(n.own, p.origin(), p.id(), nodes)
					break
				}
				// have value, return that
				resp = newFindValueResponsePacketWithValue(n.own, p.origin(), p.id(), val)
			default:
				// unknown
				log.Errorf("Received packet with unknown type %d", p.packetType())
				return
			}

			if resp == nil {
				panic("main loop: no response generated!")
			}

			_, err = n.socket.WriteToUDP(serializePacket(resp), addr)
			if err != nil {
				log.Errorln("Unable to send response:", err)
				// do not abort, we can still update our buckets and connections
			} else {
				log.Debugf("Responded to packet of type %d, took %s", p.packetType(), time.Since(start))
			}

			// update kbuckets
			err = n.peerStore.put(Peer{ID: p.origin(), IP: addr.IP, Port: uint16(addr.Port)})
			if err != nil {
				log.Errorln("Unable to update peer store:", err)
			}

			// update n.conns
			err = n.connStore.put(p.origin(), addr)
			if err != nil {
				log.Errorln("Unable to update connections:", err)
			}
		}()
	}
}

func (n *node) doRPC(p packet) (packet, error) {
	conn, err := n.connStore.get(p.destination())
	if err != nil {
		return nil, err
	}

	err = n.requestStore.prepare(p.id())
	if err != nil {
		return nil, err
	}

	_, err = n.socket.WriteToUDP(serializePacket(p), conn)
	if err != nil {
		n.requestStore.abort(p.id())
		return nil, err
	}

	log.Debugf("Sent RPC with rpcID=%x to node %x", p.id(), p.destination())

	return n.requestStore.await(p.id(), time.Second)
}
