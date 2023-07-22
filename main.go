package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/extintor/bencode"
)

type info struct {
	Length uint64 `bencode:"length"`
	Name string `bencode:"name"`
	PieceLength uint64 `bencode:"piece length"`
	Pieces string `bencode:"pieces"`
}

type torrent struct {
	Announce string `bencode:"announce"`
	Comment string `bencode:"comment"`
	AnnounceList [][]string `bencode:"announce-list"`
	Info info `bencode:"info"`
}

type trackerResponse struct {
	Complete uint64 `bencode:"complete"`
	Incomplete uint64 `bencode:"incomplete"`
	Iterval uint64 `bencode:"interval"`
	Peers string `bencode:"peers"`
}

type client struct {
	host string
	port string
}

func (t *torrent) infoHash() (string, error) {
	c, err := bencode.Encode(t.Info)
	if err != nil {
		return "", err
	}
	e := sha1.Sum(c)
	return url.QueryEscape(string(e[:])), nil
}

func contactBroker(t *torrent) error {
	infoHash, err := t.infoHash()
	if err != nil {
		return err
	}
	brokerUrl := fmt.Sprintf("%s?info_hash=%s&peer_id=ABCDEFGHIJKLMNOPQRST&port=6881&uploaded=0&downloaded=0&left=727955456&event=started&numwant=100&no_peer_id=1&compact=1", t.Announce, infoHash)
	res, err := http.Get(brokerUrl)
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	tr := &trackerResponse{}
	bencode.Decode(resBody, tr)
	fmt.Println(getClients([]byte(tr.Peers)))
	return nil
}

func getClients(cp []byte) []*client {
	r := make([]*client, 0)
	for i := 0; i < len(cp); i += 6 {
		ip := fmt.Sprintf("%d:%d:%d:%d", cp[i], cp[i+1], cp[i+2], cp[i+3])
		port := fmt.Sprintf("%d", binary.BigEndian.Uint16(cp[i+4:i+6]))
		c := &client{host: ip, port: port}
		r = append(r, c)
	}
	return r
}

func getCreateHandler() http.HandlerFunc {
	return func (w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(0); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, fileHeader, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		fileContent, err := fileHeader.Open()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		bs, err := ioutil.ReadAll(fileContent)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		torrentFile := &torrent{}
		bencode.Decode(bs, torrentFile)
		log.Println(torrentFile.Announce, torrentFile.Info.Name, torrentFile.Info.PieceLength)

		err = contactBroker(torrentFile)
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	http.HandleFunc("/api/v1/create", getCreateHandler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
