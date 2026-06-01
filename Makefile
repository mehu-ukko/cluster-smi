BIN_DIR := /usr/local/bin
SYSTEMD_DIR := /etc/systemd/system

all:
	go build -o cluster-smi cluster-smi.go config.go
	go build -o cluster-smi-router cluster-smi-router.go config.go
	go build -o cluster-smi-node cluster-smi-node.go config.go cluster.go
	go build -o cluster-smi-local cluster-smi-local.go config.go cluster.go
clean:
	rm -f cluster-smi cluster-smi-router cluster-smi-node cluster-smi-local
install:
	install -v cluster-smi-local cluster-smi-node cluster-smi-router cluster-smi $(BIN_DIR)
	install docs/cluster-smi-node.service $(SYSTEMD_DIR)
	systemctl enable cluster-smi-node.service
	systemctl start cluster-smi-node.service
uninstall:
	systemctl disable cluster-smi-node.service
	systemctl stop cluster-smi-node.service
	rm -f $(SYSTEMD_DIR)/cluster-smi-node.service 
	rm -f $(BIN_DIR)/cluster-smi-local
	rm -f $(BIN_DIR)/cluster-smi-node
	rm -f $(BIN_DIR)/cluster-smi-router
	rm -f $(BIN_DIR)/cluster-smi


install-router:
	install docs/cluster-smi-router.service $(SYSTEMD_DIR)
	systemctl enable cluster-smi-router.service
	systemctl start cluster-smi-router.service

uninstall-router:
	systemctl disable cluster-smi-router.service
	systemctl stop cluster-smi-router.service
	rm -f $(SYSTEMD_DIR)/cluster-smi-router.service
