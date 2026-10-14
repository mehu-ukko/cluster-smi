package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/mehu-ukko/cluster-smi/internal/clusterapp"
	"github.com/mehu-ukko/cluster-smi/internal/config"
	"github.com/vmihailenco/msgpack/v5"
	"log"
	"time"
)

// set at build time via -ldflags "-X main.version=..."
var version = "dev"

// dialPush returns a PUSH socket connected to addr.
func dialPush(addr string) (zmq4.Socket, error) {
	socket := zmq4.NewPush(context.Background())
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil, err
	}
	return socket, nil
}

func main() {

	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// load ports and ip-address
	cfg := config.Load()
	cfg.Print()

	if err := clusterapp.InitNVML(); err != nil {
		log.Fatalf("Failed initializing NVML: %s\n", err.Error())
	}
	defer clusterapp.ShutdownNVML()

	// sending messages (PUSH-PULL)
	SocketAddr := "tcp://" + cfg.RouterIp + ":" + cfg.Ports.Nodes
	tick := time.Duration(cfg.Tick) * time.Second
	log.Println("Now pushing to", SocketAddr)

	// the router might not be up yet: keep retrying instead of giving up
	var socket zmq4.Socket
	for socket == nil {
		s, err := dialPush(SocketAddr)
		if err != nil {
			log.Printf("Failed to dial %s: %s (retrying in %v)\n", SocketAddr, err.Error(), tick)
			time.Sleep(tick)
			continue
		}
		socket = s
	}
	defer func() { socket.Close() }()

	node := &cluster.Node{}
	clusterapp.InitNode(node)

	log.Println("Cluster-SMI-Node is active. Press CTRL+C to shut down.")
	for range time.Tick(tick) {
		clusterapp.FetchNode(node)

		// encode data
		buf, err := msgpack.Marshal(&node)
		if err != nil {
			log.Fatal("encode error:", err)
		}

		// re-establish the connection dropped by a previous send error
		if socket == nil {
			s, err := dialPush(SocketAddr)
			if err != nil {
				log.Println("redial error:", err)
				continue
			}
			log.Println("Reconnected to", SocketAddr)
			socket = s
		}

		// send data
		if err := socket.Send(zmq4.NewMsg(buf)); err != nil {
			// zmq4 does not reconnect a PUSH socket on its own: once the router
			// closed the connection, every further send is silently dropped.
			// Drop the socket here and dial a fresh one on the next tick.
			log.Println("send error:", err)
			socket.Close()
			socket = nil
		}
	}

}
