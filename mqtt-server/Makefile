build:
	go build -o mqtt-server .

install:
	mkdir -p /opt/mqtt-server
	cp mqtt-server /opt/mqtt-server/
	cp mqtt-server.service /etc/systemd/system/
	systemctl daemon-reload
	systemctl enable mqtt-server
	systemctl start mqtt-server

uninstall:
	systemctl stop mqtt-server
	rm -r /opt/mqtt-server
	rm /etc/systemd/system/mqtt-server.service
	systemctl daemon-reload
