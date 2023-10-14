package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/extintor/bencode"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	PEER_ID = fmt.Sprintf("CS%s-%013d", VERSION, rand.Int63n(int64(math.Pow10(13))))
}

const VERSION = "0100"
const PORT = "6881"
var PEER_ID string

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

type download struct {
	clients []*client
	torrent *torrent
}

func (c *client) String() string {
	return fmt.Sprintf("%s:%s", c.host, c.port)
}

func (t *torrent) escapedInfoHash() (string, error) {
	r, err := t.rawInfoHash()
	if err != nil {
		return "", err
	}
	return url.QueryEscape(r), nil
}

func (t *torrent) rawInfoHash() (string, error) {
	c, err := bencode.Encode(t.Info)
	if err != nil {
		return "", err
	}
	e := sha1.Sum(c)
	return string(e[:]), nil
}

func contactBroker(t *torrent) ([]*client, error) {
	infoHash, err := t.escapedInfoHash()
	if err != nil {
		return []*client{}, err
	}
	brokerUrl := fmt.Sprintf("%s?info_hash=%s&peer_id=ABCDEFGHIJKLMNOPQRST&port=%s&uploaded=0&downloaded=0&left=727955456&event=started&numwant=100&no_peer_id=1&compact=1", t.Announce, infoHash, PORT)
	res, err := http.Get(brokerUrl)
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return []*client{}, err
	}
	tr := &trackerResponse{}
	bencode.Decode(resBody, tr)
	return getClients([]byte(tr.Peers)), nil
}

func getClients(cp []byte) []*client {
	r := make([]*client, 0)
	for i := 0; i < len(cp); i += 6 {
		ip := fmt.Sprintf("%d.%d.%d.%d", cp[i], cp[i+1], cp[i+2], cp[i+3])
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

		clients, err := contactBroker(torrentFile)
		log.Println(clients)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m, err := NewManager(clients, torrentFile)
		go m.Download()
		if err != nil {
			panic(err)
		}
	}
}

func handleConnection(c net.Conn) {
        fmt.Printf("Serving %s\n", c.RemoteAddr().String())
        for {
                netData, err := bufio.NewReader(c).ReadString('\n')
                if err != nil {
                        fmt.Println(err)
						log.Printf("Connection Closed")
                        return
                }

                temp := strings.TrimSpace(string(netData))
				log.Printf("Received: %s", temp)
        }
}

func Listen() {
	l, err := net.Listen("tcp4", ":"+PORT)
        if err != nil {
                fmt.Println(err)
                return
        }
        defer l.Close()

        fmt.Printf("Listening TCP PORT %s\n", PORT)
        for {
                c, err := l.Accept()
                if err != nil {
                        fmt.Println(err)
                        return
                }
                go handleConnection(c)
        }
	}

func main() {
	fmt.Println("Peer_id:", PEER_ID)
	http.HandleFunc("/api/v1/create", getCreateHandler())
	go Listen()
	log.Fatal(http.ListenAndServe(":8080", nil))
}
