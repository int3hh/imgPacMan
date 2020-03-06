GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean

all: build-debug

build-debug:
	$(GOBUILD) -v
clean:
	$(GOCLEAN)
build-release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -trimpath -ldflags="-s -w -X main.isDebug=false" -a -v 
install: build-release
	sudo cp -r ./etc /
	sudo cp imgPacMan /usr/bin
	sudo chmod +x /usr/bin/imgPacMan
	@echo "Service installed"