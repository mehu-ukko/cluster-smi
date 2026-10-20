package routerclient

import (
	"context"
	"testing"
	"time"

	"github.com/go-zeromq/zmq4"
	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/vmihailenco/msgpack/v5"
)

func TestPoll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// a minimal cluster-smi-router answering every request with one node
	router := zmq4.NewRouter(ctx)
	defer router.Close()
	if err := router.Listen("tcp://127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	body, err := msgpack.Marshal(&cluster.Cluster{Nodes: []cluster.Node{{Name: "node01"}}})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			req, err := router.Recv()
			if err != nil {
				return
			}
			_ = router.SendMulti(zmq4.NewMsgFrom(req.Frames[0], req.Frames[1], body))
		}
	}()

	got := make(chan cluster.Cluster, 10)
	done := make(chan struct{})
	go func() {
		Poll(ctx, "tcp://"+router.Addr().String(), 10*time.Millisecond, func(c cluster.Cluster) { got <- c })
		close(done)
	}()

	for i := 0; i < 2; i++ {
		select {
		case c := <-got:
			if len(c.Nodes) != 1 || c.Nodes[0].Name != "node01" {
				t.Errorf("update %d = %+v", i, c)
			}
		case <-ctx.Done():
			t.Fatal("no update received")
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Poll did not return after cancel")
	}
}
