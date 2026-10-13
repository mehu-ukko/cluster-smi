package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/mehu-ukko/cluster-smi/internal/config"
	"github.com/vmihailenco/msgpack/v5"
	"log"
	"os"
	"time"
)

// dummy request for REQ-ROUTER pattern
type Request struct {
	Identity string
}

func RequestUpdateMessage() ([]byte, error) {
	id := fmt.Sprintf("REQ %v", os.Getpid())
	req := Request{id}
	return msgpack.Marshal(&req)
}

// dialReq returns a REQ socket connected to addr.
func dialReq(addr string) (zmq4.Socket, error) {
	socket := zmq4.NewReq(context.Background())
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil, err
	}
	return socket, nil
}

func main() {

	showTimePtr := flag.Bool("t", false, "show time of events")
	showExtendedPtr := flag.Bool("e", false, "extended view")
	showProcessesPtr := flag.Bool("p", false, "verbose process information")
	showDetailPtr := flag.Bool("d", false, "detail view with fan, temp, and power info")
	nodeRegex := flag.String("n", ".", "match node-names with regex for display information "+
		"(if not specified, all nodes will be shown)")
	usernameFilter := flag.String("u", "", "show all information only for specific user")
	useColor := flag.Bool("color", true, "use colored output")
	flag.Parse()

	// load ports and ip-address
	cfg := config.Load()

	SocketAddr := "tcp://" + cfg.RouterIp + ":" + cfg.Ports.Clients
	tick := time.Duration(cfg.Tick) * time.Second

	// ask for updates messages (REQ-ROUTER)
	// the router might not be up yet: keep retrying instead of giving up
	var request_socket zmq4.Socket
	for request_socket == nil {
		s, err := dialReq(SocketAddr)
		if err != nil {
			log.Printf("Failed to dial %s: %s (retrying in %v)\n", SocketAddr, err.Error(), tick)
			time.Sleep(tick)
			continue
		}
		request_socket = s
	}
	defer func() { request_socket.Close() }()

	for {

		// a REQ socket cannot resume a broken request-reply cycle: whenever one
		// fails the socket is dropped and a fresh one is dialed here
		if request_socket == nil {
			s, err := dialReq(SocketAddr)
			if err != nil {
				log.Println("redial error:", err)
				time.Sleep(tick)
				continue
			}
			log.Println("Reconnected to", SocketAddr)
			request_socket = s
		}

		// request new update
		buf, err := RequestUpdateMessage()
		if err != nil {
			log.Fatal("request messsage error:", err)
		}
		if err = request_socket.Send(zmq4.NewMsg(buf)); err != nil {
			log.Println("sending request messsage error:", err)
			request_socket.Close()
			request_socket = nil
			time.Sleep(tick)
			continue
		}

		// response from cluster-smi-server
		msg, err := request_socket.Recv()
		if err != nil {
			log.Println(err)
			request_socket.Close()
			request_socket = nil
			time.Sleep(tick)
			continue
		}

		var clus cluster.Cluster
		err = msgpack.Unmarshal(msg.Bytes(), &clus)
		if err != nil {
			log.Println("decode error:", err)
			continue
		}

		if *usernameFilter != "" {
			clus = cluster.FilterByUser(clus, *usernameFilter)
		}

		clus.Sort()
		clus.FilterNodes(*nodeRegex)
		clus.Print(*showProcessesPtr, *showTimePtr, cfg.Timeout, *useColor, *showExtendedPtr, *showDetailPtr)
		time.Sleep(tick)
	}

}
