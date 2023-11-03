package dht

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/extintor/bencode"
)

type pingArgument struct {
	ID string `bencode:"id"`
}

type getPeersArgument struct {
	ID string `bencode:"id"`
	InfoHash string `bencode:"info_hash"`
}

type query[T any] struct {
	TransactionID string `bencode:"t"`
	Key string `bencode:"y"`
	Argument T `bencode:"a"`
	QueryType string `bencode:"q"`
}

type getPeersResponse struct {
	ID string `bencode:"id"`
	Token string `bencode:"token"`
	Nodes string `bencode:"nodes"`
	Values []string `bencode:"values"`
}

type pingResponse struct {
	ID string `bencode:"id"`
}

type queryResponse[T any] struct {
	TransactionID string `bencode:"t"`
	Key string `bencode:"y"`
	Response T `bencode:"r"`
}

type client struct {
	host string
	port string
}

type peer struct {
	host   string
	port   int
	id     string
	pinged bool
	lastMessage time.Time
}

func newPeer(host string, port int) *peer {
  return &peer{host: host, port: port, id: "", pinged: false, lastMessage: time.Time{}}
}

func (p *peer) Ping() (string, bool, error) {
	pingMessage := query[pingArgument]{Argument: pingArgument{ID: DHT_ID }, TransactionID: "aa", Key: "q", QueryType: "ping"}
	encodedMessage, err := bencode.Encode(&pingMessage)
	if err != nil {
		return "", false, err
	}

  d := net.Dialer{Timeout: 1 * time.Second}
	conn, err := d.Dial("udp", fmt.Sprintf("%s:%d", p.host, p.port ))
	if err != nil {
    return "", false, err
	}
	_, _ = conn.Write(encodedMessage)

	go func() {
		select {
		case <-time.After(1 * time.Second):
			conn.Close()
		}
	}()

	buf := make([]byte, 500)
	_, err = bufio.NewReader(conn).Read(buf)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			return "", false, nil
		}
		return "", false, err
	}

	resp := &queryResponse[pingResponse]{}
	bencode.Decode(buf, resp)
	return resp.Response.ID, true, nil
}

func (p *peer) getPeers(infoHash string) ([]*peer, []*client) {
	message := query[getPeersArgument]{Argument: getPeersArgument{ID: DHT_ID, InfoHash: infoHash}, TransactionID: "aa", Key: "q", QueryType: "get_peers"}
	encodedMessage, err := bencode.Encode(&message)
	if err != nil {
		panic(err)
	}
  d := net.Dialer{Timeout: 1 * time.Second}
	conn, err := d.Dial("udp", fmt.Sprintf("%s:%d", p.host, p.port ))
	if err != nil {
		panic(err)
	}
	go func() {
    select {
    case <-time.After(1 * time.Second):
      conn.Close()
    }
	}()
	
  _, _ = conn.Write(encodedMessage)

	buf := make([]byte, 1024)
	_, err = bufio.NewReader(conn).Read(buf)
	if err != nil {
		return nil, nil
	}

  peers := make([]*peer, 0)
  clients := make([]*client, 0)
	resp := &queryResponse[getPeersResponse]{}
	bencode.Decode(buf, resp)

  for _, value := range resp.Response.Values {
    encodedIP := []byte(value[0:4])
    encodedPort := []byte(value[4:6])
    ip, port := decodeIPPort(encodedIP, encodedPort)
    clients = append(clients, &client{host: ip, port: strconv.Itoa(port)})
  }
	for offset := 0; offset < len(resp.Response.Nodes); offset += 26{
    id := resp.Response.Nodes[offset:offset+20]
    encodedIP := []byte(resp.Response.Nodes[offset+20:offset+24])
    encodedPort := []byte(resp.Response.Nodes[offset+24:offset+26])
    ip, port := decodeIPPort(encodedIP, encodedPort)
    p := newPeer(ip, port)
    p.id = id
    peers = append(peers, p)
	} 
	return peers, clients
}

func decodeIPPort(encodedIP, encodedPort []byte) (string, int) {
  ip := fmt.Sprintf("%d.%d.%d.%d", encodedIP[0], encodedIP[1], encodedIP[2], encodedIP[3])
  port := int(binary.BigEndian.Uint16(encodedPort))
  return ip, port
}
