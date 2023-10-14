package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
)

var HANDSHAKE = "\x13" + "BitTorrent protocol" + "00000000"
const HANDSHAKE_LEN = 68

type MessageType uint32 

const (
	ChokeMessage MessageType = iota
	UnchokeMessage
	InterestedMessage
	NotInterestedMessage
	HaveMessage
	BitfieldMessage
	RequestMessage
	PieceMessage
	CancelMessage
	PortMessage
)

type Peer struct {
	host     string
	port     string
	infoHash string
	pieceLength    int32
	c              net.Conn
	blocks         chan Block 
	pieces         chan Piece
	peerChoked     bool
	peerInterested bool
	amChoked       bool
	amInterested   bool
	bitfield       []byte
	donwloading    bool
}

func (p *Peer) Connect() error {
	log.Println("Connecting")
	c, err := net.Dial("tcp", fmt.Sprintf("%s:%s", p.host, p.port))
	if err != nil {
		return err
	}
	p.c = c
	log.Println("Connected")
	return nil
}

func (p *Peer) Handshake() error {
	log.Println("Handshaking")
	hs := fmt.Sprintf("%s%s%s", HANDSHAKE, p.infoHash, PEER_ID)
	fmt.Fprintf(p.c, hs)
	r, _ := p.ReadHandshake()
	if string(r[len(HANDSHAKE):len(HANDSHAKE)+20]) != hs[len(HANDSHAKE):len(HANDSHAKE)+20] {
		return fmt.Errorf("handshakes not equal")
	}
	log.Println("Handshake OK")
	return nil
}

func (p *Peer) ReadNBytes(n uint32) ([]byte, error) {
	buf := make([]byte, n) // big buffer
	_, err := io.ReadFull(p.c, buf)
	if err != nil {
		return []byte{}, err
	}
	return buf, nil
}

func (p *Peer) ReadHandshake() ([]byte, error) {
	return p.ReadNBytes(HANDSHAKE_LEN)
}

func (p *Peer) ReadLength() (uint32, error) {
	data, err := p.ReadNBytes(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(data), nil
}

func (p *Peer) ReadType() (MessageType, error) {
	data, err := p.ReadNBytes(1)
	if err != nil {
		return 0, err
	}
	return MessageType(data[0]), nil
}

func (p *Peer) Listen() {
	for {
		l, err := p.ReadLength()
		if err != nil {
			// Keep Alive
			log.Println("KeepAlive")
			return
		}
		if l == 0 {
			continue
		}
		t, err := p.ReadType()
		if err != nil {
			return
		}
		payload, err := p.ReadNBytes(l-1)
		switch t {
		case BitfieldMessage:
			p.bitfield = payload
		case PieceMessage:
			p.blocks <- Block{pieceIndex: int(binary.BigEndian.Uint32(payload[0:4])), offset: int(binary.BigEndian.Uint32(payload[4:8])), data:payload[8:]}
		case UnchokeMessage:
			p.amChoked = false
		default:
			log.Println("message not understood")
		}

	}
}

func (p *Peer) SendInterested() {
	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, 1)
	m := fmt.Sprintf("%s%s", lengthPrefix, []byte{byte(InterestedMessage)}) 
	fmt.Fprintf(p.c, m)
}

func (p *Peer) SendUnchoke() {
	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, 1)
	m := fmt.Sprintf("%s%s", lengthPrefix, []byte{byte(UnchokeMessage)}) 
	fmt.Fprintf(p.c, m)
}

func (p *Peer) Send(index, l , blockSize int) {
	pieceIndex := make([]byte, 4)
	lengthPrefix := make([]byte, 4)
	begin := make([]byte, 4)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, 13)
	offset := 0
	sentBlocks := 0
	for {
		if !p.amChoked {
			binary.BigEndian.PutUint32(pieceIndex, uint32(index))
			binary.BigEndian.PutUint32(begin, uint32(offset))
			binary.BigEndian.PutUint32(length, uint32(blockSize))
			m := fmt.Sprintf("%s%s%s%s%s", lengthPrefix, []byte{byte(RequestMessage)}, pieceIndex, begin, length) 
			fmt.Fprint(p.c, m)
			sentBlocks += 1
			if sentBlocks == l/blockSize {
				break 
			}
			offset += blockSize
		}
	}
}

