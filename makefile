build-server:
	go build -ldflags "-w -s" -o ./build/sp-server ./server.go ./base.go ./conf.go
build-client:
	go build -ldflags "-w -s" -o ./build/sp-client ./client.go ./base.go ./conf.go