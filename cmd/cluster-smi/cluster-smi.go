package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/mehu-ukko/cluster-smi/internal/config"
	"github.com/mehu-ukko/cluster-smi/internal/routerclient"
	"time"
)

// set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {

	showTimePtr := flag.Bool("t", false, "show time of events")
	showExtendedPtr := flag.Bool("e", false, "extended view")
	showProcessesPtr := flag.Bool("p", false, "verbose process information")
	showDetailPtr := flag.Bool("d", false, "detail view with fan, temp, and power info")
	nodeRegex := flag.String("n", ".", "match node-names with regex for display information "+
		"(if not specified, all nodes will be shown)")
	usernameFilter := flag.String("u", "", "show all information only for specific user")
	useColor := flag.Bool("color", true, "use colored output")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// load ports and ip-address
	cfg := config.Load()

	SocketAddr := "tcp://" + cfg.RouterIp + ":" + cfg.Ports.Clients
	tick := time.Duration(cfg.Tick) * time.Second

	// ask for updates messages (REQ-ROUTER)
	routerclient.Poll(context.Background(), SocketAddr, tick, func(clus cluster.Cluster) {
		if *usernameFilter != "" {
			clus = cluster.FilterByUser(clus, *usernameFilter)
		}

		clus.Sort()
		clus.FilterNodes(*nodeRegex)
		clus.Print(*showProcessesPtr, *showTimePtr, cfg.Timeout, *useColor, *showExtendedPtr, *showDetailPtr)
	})

}
