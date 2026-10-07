package main

import (
	"context"
	"github.com/go-zeromq/zmq4"
	"github.com/patwie/cluster-smi/cluster"
	"github.com/patwie/cluster-smi/internal/config"
	"github.com/patwie/cluster-smi/internal/exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vmihailenco/msgpack/v5"
	"log"
	"net/http"
	"sync"
	"time"
)

// intermediate struct (under mutex lock)
var allNodes map[string]cluster.Node

func main() {

	// load ports and ip-address
	cfg := config.Load()
	cfg.Print()

	allNodes = make(map[string]cluster.Node)
	var mutex = &sync.Mutex{}

	// copy the current nodes out of the map (under mutex lock)
	snapshot := func() []cluster.Node {
		mutex.Lock()
		defer mutex.Unlock()
		nodes := make([]cluster.Node, 0, len(allNodes))
		for _, n := range allNodes {
			nodes = append(nodes, n)
		}
		return nodes
	}

	// serving Prometheus metrics (optional)
	if cfg.Ports.Metrics != "" {
		registry := prometheus.NewRegistry()
		registry.MustRegister(exporter.New(snapshot, time.Duration(cfg.Timeout)*time.Second))

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

		go func() {
			MetricsAddr := ":" + cfg.Ports.Metrics
			log.Println("Serving Prometheus metrics at", MetricsAddr+"/metrics")
			if err := http.ListenAndServe(MetricsAddr, mux); err != nil {
				log.Fatalf("Failed to serve metrics at %s: %s\n", MetricsAddr, err.Error())
			}
		}()
	}

	// message loop
	log.Println("Cluster-SMI-Router is active. Press CTRL+C to shut down.")

	// receiving messages in extra thread
	go func() {
		// incoming messages (PUSH-PULL)
		SocketAddr := "tcp://*:" + cfg.Ports.Nodes
		log.Println("waiting for nodes connecting to ", SocketAddr)
		node_socket := zmq4.NewPull(context.Background())
		defer node_socket.Close()
		if err := node_socket.Listen(SocketAddr); err != nil {
			log.Fatalf("Failed to bind %s: %s\n", SocketAddr, err.Error())
		}

		for {
			// read node information
			msg, err := node_socket.Recv()
			if err != nil {
				log.Println(err)
				continue
			}

			var node cluster.Node
			if err := msgpack.Unmarshal(msg.Bytes(), &node); err != nil {
				log.Println(err)
				continue
			}

			mutex.Lock()
			if _, ok := allNodes[node.Name]; !ok {
				log.Printf("A new node \"%v\" connected\n", node.Name)
			}

			allNodes[node.Name] = node
			mutex.Unlock()

		}
	}()

	// outgoing messages (REQ-ROUTER)
	SocketAddr := "tcp://*:" + cfg.Ports.Clients
	log.Println("Waiting for clients connecting to", SocketAddr)
	router_socket := zmq4.NewRouter(context.Background())
	defer router_socket.Close()
	if err := router_socket.Listen(SocketAddr); err != nil {
		log.Fatalf("Failed to bind %s: %s\n", SocketAddr, err.Error())
	}

	for {

		// read request: ROUTER prepends identity, REQ adds empty delimiter, then body
		req, err := router_socket.Recv()
		if err != nil {
			log.Println("recv error:", err)
			continue
		}

		clus := cluster.Cluster{Nodes: snapshot()}

		body, err := msgpack.Marshal(&clus)
		if err != nil {
			log.Println("encode error:", err)
			continue
		}

		// reply must echo the identity + empty delimiter the REQ side expects
		if err := router_socket.SendMulti(zmq4.NewMsgFrom(req.Frames[0], req.Frames[1], body)); err != nil {
			log.Println("send error:", err)
		}

	}

}
