all:
	go get -d
	go-bindata -o fixtures/bindata.go -prefix="fixtures/ein" -pkg="fixtures" fixtures/ein
	go build

install:
	go-bindata -o fixtures/bindata.go -prefix="fixtures/ein" -pkg="fixtures" fixtures/ein
	go install ./cmd/...
