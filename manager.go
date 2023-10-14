package main

import (
	"log"
)

type Job struct {
	pieceIndex uint32
	offset     uint32
	length     uint32
}

type Manager struct {
	peers   []*Peer
	torrent *torrent
	file    *File
	blocks  chan Block
	concurrentDownloads chan struct{}
}

func NewManager(clients []*client, torrent *torrent) (*Manager, error) {
	var peers []*Peer
	infohash, err := torrent.rawInfoHash()
	if err != nil {
		return &Manager{}, err
	}
	b := make(chan Block, 100)
	for _, c := range clients {
		p := &Peer{host: c.host, port: c.port, infoHash: infohash, pieceLength: int32(torrent.Info.PieceLength), blocks: b,
		amChoked: true, peerChoked: true, amInterested: false, peerInterested: true}
		peers = append(peers, p)
	}
	
	lengthLastFile := (int(torrent.Info.PieceLength) * len(torrent.Info.Pieces) / 20)
	log.Println(lengthLastFile)
	numPieces := len(torrent.Info.Pieces) / 20
	log.Println("Num Pieces", numPieces)
	f := newFile(numPieces, int(torrent.Info.PieceLength), int(torrent.Info.Length), torrent.Info.Pieces, torrent.Info.Name) 
	return &Manager{peers, torrent, f, b, make(chan struct{}, 1)}, nil
}

func (m *Manager) Download() {

	go m.Receive()
	for _, p := range m.peers {
		if err := p.Connect(); err != nil {
			panic(err)
		}
		if err := p.Handshake(); err != nil {
			panic(err)
		}
		go p.Listen()
		p.SendUnchoke()
		p.SendInterested()
	}
	for _, piece := range m.file.pieces {
		m.concurrentDownloads <- struct{}{}
		for _, p := range m.peers {
			go p.Send(piece.index, piece.length, piece.blockSize)
		}
	}
}

func (m *Manager) Receive() {
	for block := range m.blocks {
		piece := m.file.pieces[block.pieceIndex]
		piece.addBlock(block)
		if piece.isFinal {
			if piece.check() {
				m.file.storePiece(piece)
				m.file.markPieceFinished(block.pieceIndex)
				log.Println(piece.index, len(m.file.missingPieces), float64(len(m.file.missingPieces)) / float64(m.file.numPieces), "%")
				<- m.concurrentDownloads
			}
		}
		if len(piece.blocks) == piece.length / piece.blockSize {
			if !piece.check() {
				panic(1)
			}
			m.file.storePiece(piece)
			piece.Free()
			m.file.markPieceFinished(block.pieceIndex)
			log.Println(piece.index, len(m.file.missingPieces), float64(len(m.file.missingPieces)) / float64(m.file.numPieces), "%")
			<- m.concurrentDownloads
		}
		if len(m.file.missingPieces) == 0 {
			break
		}
	}
	log.Println("Finished Downloading file")
	m.file.Finish()
}
