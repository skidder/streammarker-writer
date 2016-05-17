GO15VENDOREXPERIMENT=1

COVERAGEDIR = ./coverage
all: clean build test cover

clean: 
	if [ -d $(COVERAGEDIR) ]; then rm -rf $(COVERAGEDIR); fi
	if [ -e streammarker-writer ]; then rm -f streammarker-writer; fi

all: build test

build:
	go build -v -o streammarker-writer

static-build:
	CGO_ENABLED=0 go build -a -ldflags '-s' -installsuffix cgo -v -o streammarker-writer

fmt:
	go fmt ./...

test:
	if [ ! -d $(COVERAGEDIR) ]; then mkdir $(COVERAGEDIR); fi

cover:

bench:
	go test ./... -cpu 2 -bench .

run: build
	$(CURDIR)/streammarker-writer

docker-build:
	docker info
	docker build -t skidder/streammarker-writer:latest .

docker-deploy:
	docker login -e ${DOCKER_EMAIL} -u ${DOCKER_USER} -p ${DOCKER_PASS}
	docker push skidder/streammarker-writer:latest
