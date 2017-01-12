package kademlia

import (
	"errors"
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
	Store(Key, []byte) error
	FindNode(NodeID) ([]Peer, error)
	FindValue(Key) ([]byte, error)
	Addr() string
	ID() NodeID
	Stop()
	Connect(id NodeID, addr string) error
}

// NodeConfig is the configuration for a node.
type NodeConfig struct {
	// ID is the Node's ID.
	ID NodeID

	// Addr is the Node's UDP endpoint to listen on, e.g. 0.0.0.0:1234
	Addr string

	// BootstrapNodes is a list of UDP endpoints to use as bootstrap Nodes.
	BootstrapNodes []string
}

// NewNode creates a new Node from the given config.
func NewNode(cfg NodeConfig) (Node, error) {
	toReturn := &node{
		addr:           cfg.Addr,
		own:            cfg.ID,
		connStore:      newConnStore(),
		valueStore:     newValueStore(),
		requestStore:   newRequestStore(),
		wg:             &sync.WaitGroup{},
		startErr:       make(chan error),
		started:        make(chan struct{}),
		shutdown:       make(chan struct{}),
		shutdownDone:   make(chan struct{}),
		bootstrapNodes: cfg.BootstrapNodes,
	}

	toReturn.peerStore = newPeerStore(toReturn.Ping, toReturn.own)

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

	bootstrapNodes []string

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

func (n *node) Store(key Key, val []byte) error {
	closest, _, err := n.findClosePeers(key)
	if err != nil {
		return err
	}

	for _, peer := range closest {
		p := newStoreRequestPacket(n.own, peer.ID, n.nextRPCID(), key, val)
		addr := fmt.Sprintf("%s:%d", peer.IP, peer.Port)
		uaddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return err
		}
		resp, err := n.doRequest(p, uaddr)
		if err != nil {
			return err
		}
		if resp.packetType() != storeResponse {
			return errors.New("response to store request was not of type storeResponse")
		}

		sp, ok := resp.(storeResponsePacket)
		if !ok {
			return errors.New("storeResponse packet does not coerce to storeResponsePacket")
		}

		if sp.err != 0 {
			log.Infoln("Node %x responded to a store request with error code %d, ignoring...", peer.ID, sp.err)
		}
	}

	return nil
}

func (n *node) FindNode(id NodeID) ([]Peer, error) {
	closest, _, err := n.findClosePeers(Key(id))
	if err != nil {
		return nil, err
	}
	return closest, nil
}

// findClosePeers performs a node lookup.
// It returns the k closest peers, alongside with any other peers it has learned
// about during the lookup, or an error.
func (n *node) findClosePeers(key Key) ([]Peer, []Peer, error) {
	queried := make([]Peer, 0)
	findFunc := func(toCheck Peer) ([]Peer, error) {
		p := newFindNodeRequestPacket(n.own, toCheck.ID, n.nextRPCID(), NodeID(key))
		addr := fmt.Sprintf("%s:%d", toCheck.IP, toCheck.Port)
		uaddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return nil, err
		}
		resp, err := n.doRequest(p, uaddr)
		if err != nil {
			return nil, err
		}
		if resp.packetType() != findNodeResponse {
			return nil, errors.New("response to FIND_NODE packet did not have findNodeResponse type")
		}
		fp, ok := resp.(findNodeResponsePacket)
		if !ok {
			return nil, errors.New("findNodeResponse packet does not coerce to findNodeResponsePacket")
		}
		return fp.peers, nil
	}

	peers, err := n.peerStore.getClose(key, alpha)
	if err != nil {
		return nil, nil, err
	}
	if len(peers) == 0 {
		return nil, nil, errors.New("not enough peers")
	}

	peers = sortPeers(peers, key)
	currentBest := XorDist(peers[0].ID, key)
	for _, peer := range peers {
		val := XorDist(peer.ID, key)
		if val.Cmp(currentBest) < 0 {
			panic("peer sorting failed")
		}
	}

	roundRemaining := alpha - 1
	for ; roundRemaining >= 0 && len(peers) > 0; roundRemaining-- {
		// do alpha lookups, not parallel
		var currentPeer Peer

		// find a peer we have not yet queried
		for {
			currentPeer = peers[0]
			peers = peers[1:]
			if !containsPeer(queried, currentPeer) {
				break
			}
		}
		queried = append(queried, currentPeer)

		// FIND_NODE on that peer
		newPeers, err := findFunc(currentPeer)
		if err != nil {
			roundRemaining++
			continue
		}

		// dedup, sort, check if we have an improvement
		newPeers = dedupPeers(sortPeers(peers, key))
		if len(newPeers) == 0 {
			continue
		}

		newBest := XorDist(newPeers[0].ID, key)
		if newBest.Cmp(currentBest) < 0 {
			currentBest = newBest
			roundRemaining = alpha
		}
		// we will, if we do alpha lookups without receiving a better
		// peer, automatically fall out of the loop

		peers = dedupPeers(sortPeers(append(peers, newPeers...), key))
	}

	// query k best
	toQuery := peers
	if len(peers) > k {
		toQuery = peers[:k]
	}
	for _, p := range toQuery {
		newPeers, err := findFunc(p)
		if err != nil {
			log.Println("error on the k best nodes:", err)
			continue
		}
		// no need to sort here
		peers = append(peers, newPeers...)
	}

	union := dedupPeers(sortPeers(append(queried, peers...), key))

	//optional: determine all the nodes we never sent anything to and at least ping them?
	// no. We just return them. Whoever wants to can do whatever they want with them

	if len(union) < k {
		return union, []Peer{}, nil
	}

	return union[:k], union[k:], nil
}

func (n *node) FindValue(Key) ([]byte, error) {
	return []byte{}, nil
}

func (n *node) bootsrap() error {
	// contact bootstrap nodes
	for _, addr := range n.bootstrapNodes {
		uaddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return err
		}

		// just manually ping the node - the response will trigger an
		// update for our kbuckets and connections.
		p := newPingRequest(n.own, NodeID([20]byte{}), n.nextRPCID())

	retry:
		resp, err := n.doRequest(p, uaddr)
		if err != nil && err == errRPCIDInUse {
			p.rpcID = n.nextRPCID()
			goto retry
		} else if err != nil && err == errTimeout {
			// node unresponsive, ignore.
			log.Warnf("bootstrap: node at %s unresponsive", addr)
			continue
		} else if err != nil {
			return err
		}
		log.Debugf("bootstrap: connected to %x at %s", resp.origin(), addr)
	}

	// look up own node's ID
	closest, notClosest, err := n.findClosePeers(Key(n.own))
	if err != nil {
		// ignore, we probably don't know about any peers yet
		log.Warnln("bootstrapping error, probably have no nodes:", err)
		return nil
	}

	// ping all the nodes we learnt about
	for _, p := range append(closest, notClosest...) {
		uaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", p.IP, p.Port))
		if err != nil {
			return err
		}

		p := newPingRequest(n.own, p.ID, n.nextRPCID())
		_, err = n.doRequest(p, uaddr)
		if err != nil {
			log.Infoln("bootstrap: pinging one of the nodes we learned about failed:", err)
		}
	}

	return nil
}

func (n *node) nextRPCID() rpcID {
	//TODO make this request an ID from the request store, or something, to
	// lower the chance of collisions even more
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

	return n.doRequest(p, conn)
}

func (n *node) doRequest(p packet, addr *net.UDPAddr) (packet, error) {
	err := n.requestStore.prepare(p.id())
	if err != nil {
		return nil, err
	}

	_, err = n.socket.WriteToUDP(serializePacket(p), addr)
	if err != nil {
		n.requestStore.abort(p.id())
		return nil, err
	}

	log.Debugf("Sent RPC with rpcID=%x to node %x", p.id(), p.destination())

	return n.requestStore.await(p.id(), time.Second)
}
