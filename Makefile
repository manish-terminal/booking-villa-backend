.PHONY: build clean deploy test local

# Build the Lambda binary
build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap ./cmd/main.go

# Clean build artifacts
clean:
	rm -f bootstrap
	rm -rf .aws-sam

# Run go mod tidy
tidy:
	go mod tidy

# Download dependencies
deps:
	go mod download

# Run tests
test:
	go test ./...

# Validate SAM template
validate:
	sam validate

# Build with SAM
sam-build: tidy
	sam build

# Deploy to AWS
deploy: sam-build
	sam deploy --guided

# Start local API
local: sam-build
	sam local start-api

# Invoke function locally
invoke:
	sam local invoke BookingFunction --event events/test-event.json

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run ./...

# Development build (faster, not optimized)
dev-build:
	go build -o bootstrap ./cmd/main.go
