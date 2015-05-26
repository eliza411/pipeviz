all:
	go get -d ./...
	go build ./...

dev:
	go get github.com/jteeuwen/go-bindata...
	go-bindata -o fixtures/bindata.go -prefix="fixtures/ein" -pkg="fixtures" fixtures/ein

install:
	go install
	go install ./cmd/...
