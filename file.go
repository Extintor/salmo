package main

import (
	"crypto/sha1"
	"log"
	"os"
)


const BLOCK_SIZE = 16384 / 2

type File struct {
	name string
	pieces []*Piece
	numPieces int
	missingPieces map[int]struct{}
	handler *os.File 
	size int
}

func newFile(n, l, size int, hashes, name string) *File {
	p := make([]*Piece, n)
	mp := make(map[int]struct{}, n)
	for i := range p {
		p[i] = &Piece{make([]Block, 0), [20]byte{}, i, false, l, false, 0, BLOCK_SIZE}
		copy(p[i].hash[:], hashes[i*20:(i*20)+20])
		mp[i] = struct{}{}
	}
	p[len(p) - 1].isFinal = true
	p[len(p) - 1].blockSize = BLOCK_SIZE / 4
	fh, err := os.Create(name + ".bin")
	if err != nil {
		panic(1)
	}
	return &File{name, p, n, mp, fh, size}
}

func (f *File) Finish() {
	os.Truncate(f.name + ".bin", int64(f.size))
	e := os.Rename(f.name + ".bin", f.name) 
    if e != nil { 
        log.Fatal(e) 
    }
}

func (f *File) markPieceFinished(i int) {
	delete(f.missingPieces, i)
	if len(f.missingPieces) < 10 {
		log.Println("Missing Pieces", f.missingPieces)
	}
}

func (f *File) storePiece(p *Piece) error {
	data := make([]byte, 0)
	for _, b := range p.blocks {
		data = append(data, b.data...)
	}
	f.handler.Seek(int64(p.index) * int64(p.length), 0)
	f.handler.Write(data)
	return nil
}

type Piece struct {
	blocks []Block
	hash   [20]byte 
	index  int
	completed bool
	length int
	isFinal bool
	size    int
	blockSize int
}

func newPiece(i, l int) *Piece {
	return &Piece{make([]Block, 0), [20]byte{}, i, false, l, false, 0, BLOCK_SIZE}
}

func (p *Piece) addBlock(b Block) {
	// p.data = append(p.data[:b.offset], append(b.data, p.data[len(b.data)+b.offset:]...)...)
	if p.size < b.offset + len(b.data) {
		p.size = b.offset + len(b.data)
	}
	for i, cb := range p.blocks {
		if b.offset == cb.offset { // same offset, overwrite
			p.blocks[i] = b
			return
		}
		if b.offset < cb.offset {
			p.blocks = append(p.blocks[:i+1], p.blocks[i:]...)
			p.blocks[i] = b
			return
		}
	}
    p.blocks = append(p.blocks, b)
}

func (p *Piece) isDownloaded() bool {
	return true
}

func (p *Piece) Free() {
	p.blocks = nil
}

func (p *Piece) check() bool {
	data := make([]byte, 0)
	for _, b := range p.blocks {
		data = append(data, b.data...)
	}
	if sha1.Sum(data) == p.hash {
		return true
	}

	return false
}

type Block struct {
	data       []byte
	offset     int
	pieceIndex int
}
