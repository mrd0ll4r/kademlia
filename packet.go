package kademlia

import (
	"errors"
	"fmt"
	"net"
)

var errPacketTooShort = errors.New("packet too short")

type packetType byte

const (
	pingRequest packetType = iota
	pingResponse
	storeRequest
	storeResponse
	findNodeRequest
	findNodeResponse
	findValueRequest
	findValueResponse
)

const (
	findValueWithValue byte = iota
	findValueWithNodes
)

const headerLength = 61

type rpcID [20]byte

func rpcIDFromBytes(b []byte) rpcID {
	if len(b) != 20 {
		panic("len")
	}

	var buf [20]byte
	copy(buf[:], b)
	return rpcID(buf)
}

func (r rpcID) bytes() []byte {
	return r[:]
}

type packet interface {
	packetType() packetType
	origin() NodeID
	destination() NodeID
	id() rpcID
	payload() []byte
}

func serializePacket(p packet) []byte {
	toReturn := make([]byte, 0, 1024)
	toReturn = append(toReturn, byte(p.packetType()))
	toReturn = append(toReturn, p.id().bytes()...)
	toReturn = append(toReturn, p.origin().bytes()...)
	toReturn = append(toReturn, p.destination().bytes()...)
	toReturn = append(toReturn, p.payload()...)

	if len(toReturn) > 1024 {
		return toReturn[:1024]
	}

	return toReturn
}

func parsePacket(b []byte) (packet, error) {
	if len(b) < headerLength {
		return nil, errors.New("packet too short")
	}

	header, err := decodePacketHeader(b[:headerLength])
	if err != nil {
		return nil, err
	}

	switch header.packetType() {
	case pingRequest:
		return pingRequestPacket{packetHeader: header}, nil
	case pingResponse:
		return pingResponsePacket{packetHeader: header}, nil
	case storeRequest:
		toReturn := storeRequestPacket{packetHeader: header}
		err := toReturn.parseFromPayload(b[headerLength:])
		if err != nil {
			return nil, err
		}
		return toReturn, nil
	case storeResponse:
		toReturn := storeResponsePacket{packetHeader: header}
		err := toReturn.parseFromPayload(b[headerLength:])
		if err != nil {
			return nil, err
		}
		return toReturn, nil
	case findNodeRequest:
		toReturn := findNodeRequestPacket{packetHeader: header}
		err := toReturn.parseFromPayload(b[headerLength:])
		if err != nil {
			return nil, err
		}
		return toReturn, nil
	case findNodeResponse:
		toReturn := findNodeResponsePacket{packetHeader: header}
		err := toReturn.parseFromPayload(b[headerLength:])
		if err != nil {
			return nil, err
		}
		return toReturn, nil
	case findValueRequest:
		toReturn := findValueRequestPacket{packetHeader: header}
		err := toReturn.parseFromPayload(b[headerLength:])
		if err != nil {
			return nil, err
		}
		return toReturn, nil
	case findValueResponse:
		toReturn := findValueResponsePacket{packetHeader: header}
		err := toReturn.parseFromPayload(b[headerLength:])
		if err != nil {
			return nil, err
		}
		return toReturn, nil
	default:
		return nil, fmt.Errorf("unknown packet type: %d", header.packetType())
	}
}

type packetHeader struct {
	typ   packetType // 1 byte
	rpcID rpcID      // 20 byte
	orig  NodeID     // 20 byte
	dest  NodeID     // 20 byte
}

func (p packetHeader) packetType() packetType {
	return p.typ
}

func (p packetHeader) id() rpcID {
	return p.rpcID
}

func (p packetHeader) origin() NodeID {
	return p.orig
}

func (p packetHeader) destination() NodeID {
	return p.dest
}

func decodePacketHeader(b []byte) (packetHeader, error) {
	if len(b) < headerLength {
		return packetHeader{}, errors.New("packet too short")
	}
	return packetHeader{
		typ:   packetType(b[0]),
		rpcID: rpcIDFromBytes(b[1:21]),
		orig:  NodeIDFromBytes(b[21:41]),
		dest:  NodeIDFromBytes(b[41:61]),
	}, nil
}

type pingRequestPacket struct{ packetHeader }

func (p pingRequestPacket) payload() []byte {
	return []byte{}
}

type pingResponsePacket struct{ packetHeader }

func (p pingResponsePacket) payload() []byte {
	return []byte{}
}
func newPingRequest(origin NodeID, destination NodeID, rpcID rpcID) pingRequestPacket {
	return pingRequestPacket{packetHeader: packetHeader{typ: pingRequest, orig: origin, dest: destination, rpcID: rpcID}}
}

func newPingResponse(origin NodeID, destination NodeID, rpcID rpcID) pingResponsePacket {
	return pingResponsePacket{packetHeader: packetHeader{typ: pingResponse, orig: origin, dest: destination, rpcID: rpcID}}
}

type storeRequestPacket struct {
	packetHeader
	key   Key    // 20 byte
	value []byte // "opaque"
}

func newStoreRequestPacket(origin NodeID, destination NodeID, rpcID rpcID, key Key, value []byte) storeRequestPacket {
	p := storeRequestPacket{
		packetHeader: packetHeader{
			typ:   storeRequest,
			orig:  origin,
			dest:  destination,
			rpcID: rpcID,
		},
		key:   key,
		value: make([]byte, len(value)),
	}
	copy(p.value, value)

	return p
}

func (p storeRequestPacket) payload() []byte {
	toReturn := make([]byte, 0, 1024)
	toReturn = append(toReturn, p.key[:]...)
	if len(p.value) > 1024 {
		toReturn = append(toReturn, p.value[:1024]...)
	} else {
		toReturn = append(toReturn, p.value...)
	}
	return toReturn
}

func (p *storeRequestPacket) parseFromPayload(b []byte) error {
	if len(b) < 20 {
		return errPacketTooShort
	}
	copy(p.key[:], b[:20])
	p.value = make([]byte, len(b)-20)
	copy(p.value, b[20:])

	return nil
}

type storeResponsePacket struct {
	packetHeader
	err byte
}

func newStoreResponsePacket(origin NodeID, destination NodeID, rpcID rpcID, err byte) storeResponsePacket {
	return storeResponsePacket{
		packetHeader: packetHeader{
			typ:   storeResponse,
			orig:  origin,
			dest:  destination,
			rpcID: rpcID,
		},
		err: err,
	}
}

func (p storeResponsePacket) payload() []byte {
	return []byte{p.err}
}

func (p *storeResponsePacket) parseFromPayload(b []byte) error {
	if len(b) < 1 {
		return errPacketTooShort
	}
	p.err = b[0]

	return nil
}

type findNodeRequestPacket struct {
	packetHeader
	nodeID NodeID
}

func newFindNodeRequestPacket(origin NodeID, destination NodeID, rpcID rpcID, nodeID NodeID) findNodeRequestPacket {
	p := findNodeRequestPacket{
		packetHeader: packetHeader{
			typ:   findNodeRequest,
			orig:  origin,
			dest:  destination,
			rpcID: rpcID,
		},
		nodeID: [20]byte{},
	}
	copy(p.nodeID[:], nodeID[:])

	return p
}

func (p findNodeRequestPacket) payload() []byte {
	toReturn := make([]byte, len(p.nodeID))
	copy(toReturn, p.nodeID[:])

	return toReturn
}

func (p *findNodeRequestPacket) parseFromPayload(b []byte) error {
	if len(b) < 20 {
		return errPacketTooShort
	}
	copy(p.nodeID[:], b[:20])

	return nil
}

type findNodeResponsePacket struct {
	packetHeader
	peers []Peer
}

func newFindNodeResponsePacket(origin NodeID, destination NodeID, rpcID rpcID, peers []Peer) findNodeResponsePacket {
	p := findNodeResponsePacket{
		packetHeader: packetHeader{
			typ:   findNodeResponse,
			orig:  origin,
			dest:  destination,
			rpcID: rpcID,
		},
		peers: make([]Peer, 0, len(peers)),
	}

	for _, peer := range peers {
		cpeer := Peer{
			Port: peer.Port,
			IP:   make(net.IP, len(peer.IP)),
			ID:   [20]byte{},
		}
		copy(cpeer.IP, peer.IP)
		copy(cpeer.ID[:], peer.ID[:])
		p.peers = append(p.peers, cpeer)
	}

	return p
}

func (p findNodeResponsePacket) payload() []byte {
	toReturn := make([]byte, 0, k*(22+net.IPv6len))

	for _, peer := range p.peers {
		toReturn = append(toReturn, peer.bytes()...)
	}

	return toReturn
}

func (p *findNodeResponsePacket) parseFromPayload(b []byte) error {
	peers, err := decodePeers(b)
	if err != nil {
		return err
	}
	p.peers = peers

	return nil
}

func decodePeers(b []byte) (p []Peer, err error) {
	defer func() {
		// We needthis because PeerFromBytes can panic.
		// This is not optimal, defers are slow.
		if k := recover(); k != nil {
			p = nil
			err = fmt.Errorf("decodePeers: recovered: %s", k)
		}
	}()

	for i := 0; i < len(b); i += 22 + net.IPv6len {
		if i+(22+net.IPv6len) > len(b) {
			return nil, errors.New("malformed Peers")
		}

		p = append(p, peerFromBytes(b[i:i+22+net.IPv6len]))
	}

	return
}

type findValueRequestPacket struct {
	packetHeader
	key Key
}

func newFindValueRequestPacket(origin NodeID, destination NodeID, rpcID rpcID, key Key) findValueRequestPacket {
	p := findValueRequestPacket{
		packetHeader: packetHeader{
			typ:   findValueRequest,
			orig:  origin,
			dest:  destination,
			rpcID: rpcID,
		},
		key: [20]byte{},
	}
	copy(p.key[:], key[:])

	return p
}

func (p findValueRequestPacket) payload() []byte {
	toReturn := make([]byte, len(p.key))
	copy(toReturn, p.key[:])

	return toReturn
}

func (p *findValueRequestPacket) parseFromPayload(b []byte) error {
	if len(b) < 20 {
		return errPacketTooShort
	}
	copy(p.key[:], b[:20])

	return nil
}

type findValueResponsePacket struct {
	packetHeader
	// Either FindValueWithNodes or FindValueWithValue
	findValueResponseType byte

	// If FindValueWithNodes
	peers []Peer
	// If FindValueWithValue
	value []byte
}

func newFindValueResponsePacketWithValue(origin NodeID, destination NodeID, rpcID rpcID, value []byte) findValueResponsePacket {
	p := findValueResponsePacket{
		packetHeader: packetHeader{
			typ:   findValueResponse,
			orig:  origin,
			dest:  destination,
			rpcID: rpcID,
		},
		findValueResponseType: findValueWithValue,
		value: make([]byte, len(value)),
	}
	copy(p.value, value)

	return p
}

func newFindValueResponsePacketWithNodes(origin NodeID, destination NodeID, rpcID rpcID, peers []Peer) findValueResponsePacket {
	p := findValueResponsePacket{
		packetHeader: packetHeader{
			typ:   findValueResponse,
			orig:  origin,
			dest:  destination,
			rpcID: rpcID,
		},
		findValueResponseType: findValueWithNodes,
		peers: make([]Peer, 0, len(peers)),
	}

	for _, peer := range peers {
		cpeer := Peer{
			Port: peer.Port,
			IP:   make(net.IP, len(peer.IP)),
			ID:   [20]byte{},
		}
		copy(cpeer.IP, peer.IP)
		copy(cpeer.ID[:], peer.ID[:])
		p.peers = append(p.peers, cpeer)
	}

	return p
}

func (p findValueResponsePacket) payload() []byte {
	toReturn := make([]byte, 1, 1024)
	toReturn[0] = p.findValueResponseType

	switch p.findValueResponseType {
	case findValueWithNodes:
		for _, peer := range p.peers {
			toReturn = append(toReturn, peer.bytes()...)
		}
	case findValueWithValue:
		if len(p.value) > 1024 {
			toReturn = append(toReturn, p.value[:1024]...)
		} else {
			toReturn = append(toReturn, p.value...)
		}
	default:
		panic(fmt.Sprintf("invalid findValueResponseType: %d", p.findValueResponseType))
	}

	return toReturn
}

func (p *findValueResponsePacket) parseFromPayload(b []byte) error {
	if len(b) < 1 {
		return errPacketTooShort
	}
	p.findValueResponseType = b[0]
	switch p.findValueResponseType {
	case findValueWithNodes:
		peers, err := decodePeers(b[1:])
		if err != nil {
			return err
		}
		p.peers = peers
	case findValueWithValue:
		p.value = make([]byte, len(b)-1)
		copy(p.value, b[1:])
	default:
		return fmt.Errorf("invalid findValueResponseType: %d", p.findValueResponseType)
	}
	return nil
}
