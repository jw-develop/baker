.PHONY: build clean

build::
	@echo "Building $(NAME)..."
	@go build -o $(NAME) .

clean::
	@rm -f $(NAME)
