package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mehu-ukko/cluster-smi/cluster"
)

func testCluster(name string) cluster.Cluster {
	return cluster.Cluster{Nodes: []cluster.Node{{
		Name: name,
		Time: time.Unix(1700000000, 0),
		Devices: []cluster.Device{{
			Id:          0,
			Name:        "TITAN Xp",
			Utilization: 39,
			Processes:   []cluster.Process{{Pid: 1, Username: "john", UsedGpuMemory: 1000}},
		}},
	}}}
}

func TestState(t *testing.T) {
	s := New(180)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/cluster")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("before the first update: status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	if err = s.Update(testCluster("node01")); err != nil {
		t.Fatal(err)
	}

	resp, err = http.Get(ts.URL + "/api/cluster")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var got struct {
		Timeout int
		Nodes   []cluster.Node
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Timeout != 180 || len(got.Nodes) != 1 || got.Nodes[0].Name != "node01" ||
		got.Nodes[0].Devices[0].Processes[0].Username != "john" {
		t.Errorf("got %+v", got)
	}
}

func TestEvents(t *testing.T) {
	s := New(180)
	if err := s.Update(testCluster("node01")); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}

	events := bufio.NewReader(resp.Body)
	next := func() string {
		t.Helper()
		for {
			line, err := events.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if data, ok := strings.CutPrefix(line, "data: "); ok {
				return data
			}
		}
	}

	// the latest state is sent right away, later ones as they arrive
	if got := next(); !strings.Contains(got, `"name":"node01"`) {
		t.Errorf("first event = %s, want node01", got)
	}
	if err := s.Update(testCluster("node02")); err != nil {
		t.Fatal(err)
	}
	if got := next(); !strings.Contains(got, `"name":"node02"`) {
		t.Errorf("second event = %s, want node02", got)
	}
}

func TestPage(t *testing.T) {
	ts := httptest.NewServer(New(180).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "<title>cluster-smi</title>") {
		t.Errorf("status %d, body %.100s", resp.StatusCode, body)
	}
}
