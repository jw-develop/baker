.PHONY: build test lint clean

build:
	@echo "Building $(NAME)..."
	@go build -o $(NAME) .

test:
	@go test ./...

lint:
	@go vet ./...

clean:
	@rm -f $(NAME)
