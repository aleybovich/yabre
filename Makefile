.PHONY: test

up-dep:
#	Update project dependencies
	go get -t -u ./...

test:
#	Run tests
	go test -v ./...