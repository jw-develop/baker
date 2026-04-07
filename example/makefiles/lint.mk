.PHONY: lint

lint::
	@go vet ./...
