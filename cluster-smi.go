package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/patwie/cluster-smi/cluster"
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

	request_attempts := 0

	// load ports and ip-address
	cfg := LoadConfig()

	// ask for updates messages (REQ-ROUTER)
	request_socket := zmq4.NewReq(context.Background())
	defer request_socket.Close()

	SocketAddr := "tcp://" + cfg.RouterIp + ":" + cfg.Ports.Clients
	if err := request_socket.Dial(SocketAddr); err != nil {
		log.Fatalf("Failed to dial %s: %s\n", SocketAddr, err.Error())
	}
	for {

		// request new update
		buf, err := RequestUpdateMessage()
		if err != nil {
			log.Fatal("request messsage error:", err)
		}
		if err := request_socket.Send(zmq4.NewMsg(buf)); err != nil {
			log.Fatal("sending request messsage error:", err)
		}

		// response from cluster-smi-server
		msg, err := request_socket.Recv()
		if err != nil {
			log.Println(err)

			time.Sleep(10 * time.Second)
			request_attempts += 1

			if request_attempts == 0 {
				panic("too many request attempts yielding an error")
			}
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
		time.Sleep(time.Duration(cfg.Tick) * time.Second)
	}

}
