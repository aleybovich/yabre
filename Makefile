.PHONY: test
test:
#	Run tests
	go test -v -race -cover -coverprofile=coverage.out ./...

.PHONY: up-dep
up-dep:
#	Update project dependencies
	go get -t -u ./...

.PHONY: cover
cover: test
# Show test coverage
	go tool cover -html=coverage.out
