.PHONY: test test-nocache

test::
	@go test ./...

test-nocache::
	@go test -count=1 ./...
