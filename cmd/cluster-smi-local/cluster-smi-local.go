package main

/*
This is simply reproducing "nvidia-smi" without networking.
*/

import (
	"flag"
	"fmt"
	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/mehu-ukko/cluster-smi/internal/clusterapp"
	"github.com/mehu-ukko/cluster-smi/internal/config"
	"log"
	"time"
)

// set at build time via -ldflags "-X main.version=..."
var version = "dev"

var clus cluster.Cluster

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

	if err := clusterapp.InitNVML(); err != nil {
		log.Fatalf("Failed initializing NVML: %s\n", err.Error())
	}
	defer clusterapp.ShutdownNVML()

	node := cluster.Node{}
	clusterapp.InitNode(&node)

	clus.Nodes = append(clus.Nodes, node)

	log.Println("Cluster-SMI-Local is active. Press CTRL+C to shut down.")

	for {
		clusterapp.FetchNode(&clus.Nodes[0])

		if *usernameFilter != "" {
			clus = cluster.FilterByUser(clus, *usernameFilter)
		}

		clus.FilterNodes(*nodeRegex)
		clus.Print(*showProcessesPtr, *showTimePtr, cfg.Timeout, *useColor, *showExtendedPtr, *showDetailPtr)
		time.Sleep(time.Duration(cfg.Tick) * time.Second)
	}

}
