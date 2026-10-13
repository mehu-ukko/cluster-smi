BIN_DIR := /usr/local/bin
SYSTEMD_DIR := /etc/systemd/system
CONFIG_DIR := /etc/cluster-smi

all:
	go build -o cluster-smi ./cmd/cluster-smi
	go build -o cluster-smi-router ./cmd/cluster-smi-router
	go build -o cluster-smi-node ./cmd/cluster-smi-node
	go build -o cluster-smi-local ./cmd/cluster-smi-local

clean:
	rm -f cluster-smi cluster-smi-router cluster-smi-node cluster-smi-local

install:
	install -v -D -m 755 cluster-smi-local cluster-smi-node cluster-smi-router cluster-smi $(BIN_DIR)
	install -v -D -m 644 cluster-smi.example.yml ${CONFIG_DIR}/cluster-smi.yaml
	install -v -d -m 755 ${CONFIG_DIR}/cluster-smi.d
	install -v -D -m 644 service/cluster-smi-node.service $(SYSTEMD_DIR)
	systemctl enable --now cluster-smi-node.service

uninstall:
	systemctl disable --now cluster-smi-node.service
	rm -f ${CONFIG_DIR}/cluster-smi.yaml
	rm -f $(SYSTEMD_DIR)/cluster-smi-node.service
	rm -f $(BIN_DIR)/cluster-smi-local
	rm -f $(BIN_DIR)/cluster-smi-node
	rm -f $(BIN_DIR)/cluster-smi-router
	rm -f $(BIN_DIR)/cluster-smi

install-router:
	install -v -D -m 644 service/cluster-smi-router.service $(SYSTEMD_DIR)
	systemctl enable --now cluster-smi-router.service

uninstall-router:
	systemctl disable --now cluster-smi-router.service
	rm -f $(SYSTEMD_DIR)/cluster-smi-router.service
