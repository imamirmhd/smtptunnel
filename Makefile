.PHONY: all build clean install

all: build

build:
	go build -o smtptunnel-server ./cmd/server
	go build -o smtptunnel-client ./cmd/client

clean:
	rm -f smtptunnel-server smtptunnel-client

install: build
	cp smtptunnel-server /usr/local/bin/
	cp smtptunnel-client /usr/local/bin/
	mkdir -p /etc/smtptunnel
	@if [ ! -f /etc/smtptunnel/config.toml ]; then \
		cp config.toml /etc/smtptunnel/config.toml; \
		echo "Installed default config to /etc/smtptunnel/config.toml"; \
	fi

install-services:
	cp dist/smtptunnel-server.service /etc/systemd/system/
	cp dist/smtptunnel-client.service /etc/systemd/system/
	systemctl daemon-reload
	@echo "Services installed. Enable with:"
	@echo "  systemctl enable --now smtptunnel-server"
	@echo "  systemctl enable --now smtptunnel-client"

uninstall:
	systemctl stop smtptunnel-server 2>/dev/null || true
	systemctl stop smtptunnel-client 2>/dev/null || true
	systemctl disable smtptunnel-server 2>/dev/null || true
	systemctl disable smtptunnel-client 2>/dev/null || true
	rm -f /usr/local/bin/smtptunnel-server /usr/local/bin/smtptunnel-client
	rm -f /etc/systemd/system/smtptunnel-server.service
	rm -f /etc/systemd/system/smtptunnel-client.service
	systemctl daemon-reload
	@echo "Note: /etc/smtptunnel was NOT removed. Delete manually if needed."
