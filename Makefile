.PHONY: build run clean install

build:
	@echo "Building analyze-pt-stalk..."
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o analyze-pt-stalk main.go
	@echo "Build successful! Binary created: ./analyze-pt-stalk"

run: build
	@./analyze-pt-stalk $(ARGS)

clean:
	@rm -f analyze-pt-stalk
	@echo "Cleaned build artifacts"

install:
	@CGO_ENABLED=0 go install -ldflags="-s -w" .

