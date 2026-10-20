// Package routerclient requests the cluster state from cluster-smi-router
// over ZMQ, as the cluster-smi client does.
package routerclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-zeromq/zmq4"
	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/vmihailenco/msgpack/v5"
)

// dummy request for REQ-ROUTER pattern
type request struct {
	Identity string
}

func requestUpdateMessage() ([]byte, error) {
	id := fmt.Sprintf("REQ %v", os.Getpid())
	req := request{id}
	return msgpack.Marshal(&req)
}

// dialReq returns a REQ socket connected to addr.
func dialReq(ctx context.Context, addr string) (zmq4.Socket, error) {
	socket := zmq4.NewReq(ctx)
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil, err
	}
	return socket, nil
}

// Poll requests the cluster state from the router at addr every tick and
// passes each received state to handle, until ctx is canceled. Connection
// errors are logged and retried, so the router does not need to be up yet.
func Poll(ctx context.Context, addr string, tick time.Duration, handle func(cluster.Cluster)) {
	var socket zmq4.Socket
	defer func() {
		if socket != nil {
			socket.Close()
		}
	}()

	// sleep returns false once ctx is canceled
	sleep := func() bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(tick):
			return true
		}
	}

	for ctx.Err() == nil {

		// a REQ socket cannot resume a broken request-reply cycle: whenever one
		// fails the socket is dropped and a fresh one is dialed here
		if socket == nil {
			s, err := dialReq(ctx, addr)
			if err != nil {
				log.Printf("Failed to dial %s: %s (retrying in %v)\n", addr, err.Error(), tick)
				if !sleep() {
					return
				}
				continue
			}
			socket = s
		}

		// request new update
		buf, err := requestUpdateMessage()
		if err != nil {
			log.Fatal("request messsage error:", err)
		}
		if err = socket.Send(zmq4.NewMsg(buf)); err != nil {
			log.Println("sending request messsage error:", err)
			socket.Close()
			socket = nil
			if !sleep() {
				return
			}
			continue
		}

		// response from cluster-smi-router
		msg, err := socket.Recv()
		if err != nil {
			log.Println(err)
			socket.Close()
			socket = nil
			if !sleep() {
				return
			}
			continue
		}

		var clus cluster.Cluster
		if err := msgpack.Unmarshal(msg.Bytes(), &clus); err != nil {
			log.Println("decode error:", err)
		} else {
			handle(clus)
		}

		if !sleep() {
			return
		}
	}
}
