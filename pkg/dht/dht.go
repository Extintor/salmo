package dht

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)
var DHT_ID = "\x62\x0a\x57\x2e\xd7\xc3\x3b\xf7\x70\x1f\x2f\xef\x5e\xa8\x69\x94\x47\x30\xdb\x4f"
var defaultPeers = []string{"router.utorrent.com", "router.bittorrent.com", "dht.transmissionbt.com","router.bitcomet.com", "dht.aelitis.com"}
const (
  defaultPort = 6881
  peerBuffer = 10_000
)

type DHT struct {
  m *sync.Mutex
	peers    map[string]*peer
  infoHash string
  newPeers chan *peer
  ReturnChan chan *client
}

func NewDHT (infoHash string) *DHT {
  return &DHT{m: &sync.Mutex{}, infoHash: infoHash, peers: make(map[string]*peer), newPeers: make(chan *peer, peerBuffer), ReturnChan: make(chan *client, 1000000)}
}

func (d *DHT) Bootstrap() {
  slog.Info("Bootstrapping")
  for _, h := range defaultPeers {
	  ips, _ := net.LookupIP(h)
	  for _, ip := range ips {
		  if ipv4 := ip.To4(); ipv4 != nil {
			  ipString := fmt.Sprintf("%d.%d.%d.%d", ipv4[0], ipv4[1], ipv4[2], ipv4[3])
			  go d.addPeer(ipString, defaultPort)
		  }
	  }
  }
}

func (d *DHT) Start() {
  go d.processNewPeers()
	d.Bootstrap()
  select{}
}

func (d *DHT) addPeer(host string, port int) {
	p := newPeer(host, port)
	id, responded, err := p.Ping()
	if err != nil {
		slog.Warn("Could not contact with peer", "host", p.host, "port", p.port)
		return
	}
	if responded == false {
		slog.Warn("Could not contact with peer", "host", p.host, "port", p.port)
		return
	}
	slog.Warn("Contacted with peer", "host", p.host, "port", p.port, "id", id)
	p.id = id
	p.pinged = true
	p.lastMessage = time.Now()

  d.newPeers <- p
}

func (d *DHT) processNewPeers() {
  slog.Info("Processing new peers")
  jobs := make(chan struct{}, 100)
  for {
    select {
    case p := <- d.newPeers: 
      jobs <- struct{}{} 
      go func() {
        defer func() {
          <-jobs
        }()
        d.getAddPeers(p)
      }()
    }
  }
}

func (d *DHT) getAddPeers(p *peer) {
  peers, values := p.getPeers(d.infoHash)
  for _, v := range values {
    d.ReturnChan <- v
    slog.Info("Returned values", "Len", len(d.ReturnChan))
  }
  defer d.m.Unlock()
  d.m.Lock()
  for _, p := range peers {
    if _, ok := d.peers[p.id]; ok {
      d.peers[p.id] = p
      continue
    }
    d.peers[p.id] = p
    if len(d.newPeers) >= peerBuffer - 1 {
      continue // Prevent Deadlock if channel is full
    }
    d.newPeers <- p
  }
}
