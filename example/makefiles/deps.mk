.PHONY: install tidy

install::
	@go mod download

tidy::
	@go mod tidy
