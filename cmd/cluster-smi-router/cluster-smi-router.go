package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/mehu-ukko/cluster-smi/internal/config"
	"github.com/mehu-ukko/cluster-smi/internal/exporter"
	"github.com/mehu-ukko/cluster-smi/internal/web"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vmihailenco/msgpack/v5"
	"log"
	"net/http"
	"sync"
	"time"
)

// set at build time via -ldflags "-X main.version=..."
var version = "dev"

// intermediate struct (under mutex lock)
var allNodes map[string]cluster.Node

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

	// HTTP handlers by port, so metrics and web page may share one port
	muxes := make(map[string]*http.ServeMux)
	muxFor := func(port string) *http.ServeMux {
		if muxes[port] == nil {
			muxes[port] = http.NewServeMux()
		}
		return muxes[port]
	}

	// serving Prometheus metrics (optional)
	if cfg.Ports.Metrics != "" {
		registry := prometheus.NewRegistry()
		registry.MustRegister(exporter.New(snapshot, time.Duration(cfg.Timeout)*time.Second))

		muxFor(cfg.Ports.Metrics).Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		log.Println("Serving Prometheus metrics at", ":"+cfg.Ports.Metrics+"/metrics")
	}

	// serving the web page (optional)
	if cfg.Ports.Web != "" {
		page := web.New(cfg.Timeout)
		muxFor(cfg.Ports.Web).Handle("/", page.Handler())
		log.Println("Serving the web page at", ":"+cfg.Ports.Web+"/")

		// pass the current state to the page every tick, like clients request it
		go func() {
			for {
				if err := page.Update(cluster.Cluster{Nodes: snapshot()}); err != nil {
					log.Println("web encode error:", err)
				}
				time.Sleep(time.Duration(cfg.Tick) * time.Second)
			}
		}()
	}

	for port, mux := range muxes {
		server := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
		go func() {
			if err := server.ListenAndServe(); err != nil {
				log.Fatalf("Failed to serve HTTP at %s: %s\n", server.Addr, err.Error())
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
