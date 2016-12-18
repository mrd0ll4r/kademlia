package kademlia

import "errors"

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
	findValueWithNode
)

type rpcID [20]byte

func rpcIDFromBytes(b []byte) rpcID {
	if len(b) != 20 {
		panic("len")
	}

	var buf [20]byte
	copy(buf[:], b)
	return rpcID(buf)
}

type packetHeader struct {
	packetType // 1 byte
	rpcID      // 20 byte
	NodeID     // 20 byte
}

type pingPacket struct {
	packetHeader
}

type storeRequestPacket struct {
	packetHeader
	key   Key    // 20 byte
	value []byte // "opaque"
}

type storeResponsePacket struct {
	packetHeader
	err byte
}

type findNodeRequestPacket struct {
	packetHeader
	id NodeID
}

type findNodeResponsePacket struct {
	packetHeader
	peers []Peer
}

type findValueRequestPacket struct {
	packetHeader
	key Key
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

func (p storeRequestPacket) Bytes() []byte {
	toReturn := make([]byte, 0, 61+1024)
	toReturn = append(toReturn, byte(storeRequest))
	toReturn = append(toReturn, p.packetHeader.rpcID[:]...)
	toReturn = append(toReturn, p.packetHeader.NodeID[:]...)
	toReturn = append(toReturn, p.key[:]...)
	toReturn = append(toReturn, p.value[:1024]...)

	return toReturn
}

func decodePacketHeader(b []byte) (packetHeader, error) {
	if len(b) < 41 {
		return packetHeader{}, errors.New("packet too short")
	}
	return packetHeader{
		packetType(b[0]),
		rpcIDFromBytes(b[1:21]),
		NodeIDFromBytes(b[21:41]),
	}, nil
}

func decodeStoreResponsePacket(b []byte) (storeResponsePacket, error) {
	if len(b) < 42 {
		return storeResponsePacket{}, errors.New("packet too short")
	}
	header, err := decodePacketHeader(b)
	if err != nil {
		return storeResponsePacket{}, err
	}

	return storeResponsePacket{
		packetHeader: header,
		err:          b[42],
	}, nil
}
