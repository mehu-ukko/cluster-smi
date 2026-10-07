package main

import (
	"context"
	"github.com/go-zeromq/zmq4"
	"github.com/patwie/cluster-smi/cluster"
	"github.com/patwie/cluster-smi/internal/clusterapp"
	"github.com/patwie/cluster-smi/internal/config"
	"github.com/vmihailenco/msgpack/v5"
	"log"
	"time"
)

func main() {

	// load ports and ip-address
	cfg := config.Load()
	cfg.Print()

	if err := clusterapp.InitNVML(); err != nil {
		log.Fatalf("Failed initializing NVML: %s\n", err.Error())
	}
	defer clusterapp.ShutdownNVML()

	// sending messages (PUSH-PULL)
	SocketAddr := "tcp://" + cfg.RouterIp + ":" + cfg.Ports.Nodes
	log.Println("Now pushing to", SocketAddr)
	socket := zmq4.NewPush(context.Background())
	defer socket.Close()

	if err := socket.Dial(SocketAddr); err != nil {
		log.Fatalf("Failed to dial %s: %s\n", SocketAddr, err.Error())
	}

	node := &cluster.Node{}
	clusterapp.InitNode(node)

	log.Println("Cluster-SMI-Node is active. Press CTRL+C to shut down.")
	for range time.Tick(time.Duration(cfg.Tick) * time.Second) {
		clusterapp.FetchNode(node)

		// encode data
		buf, err := msgpack.Marshal(&node)
		if err != nil {
			log.Fatal("encode error:", err)
		}

		// send data
		if err := socket.Send(zmq4.NewMsg(buf)); err != nil {
			log.Println("send error:", err)
		}
	}

}
