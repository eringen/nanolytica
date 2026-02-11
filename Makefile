.PHONY: build run clean clean-data test docker templ assets assets-dev npm-install typecheck sqlc singlebinary

BINARY_NAME=nanolytica
DOCKER_IMAGE=nanolytica

# Regenerate sqlc type-safe query code
sqlc:
	cd analytics/sqlcgen && sqlc generate

# Default build target
build: templ assets
	go build -ldflags="-s -w" -o $(BINARY_NAME) .

# Generate templ files
templ:
	go run github.com/a-h/templ/cmd/templ@latest generate

# Install npm dependencies
npm-install:
	npm install

# TypeScript type check (no emit)
typecheck:
	npm run typecheck

# Build all static assets (CSS and JS)
assets: npm-install
	npm run build

# Build assets in watch mode for development
assets-dev:
	npm run build:css:dev

# Watch TypeScript for changes
watch-ts:
	npm run watch:ts

# Run the application
run: build
	./$(BINARY_NAME)

# Run with live reload (requires air)
dev:
	go run github.com/air-verse/air@latest -c .air.toml
# Run tests
test:
	go test -v ./...

# Clean build artifacts (preserves database)
clean:
	rm -f $(BINARY_NAME)
	npm run clean

# Clean everything including database (destructive!)
clean-data:
	rm -rf data/

# Docker commands
docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker run -p 8080:8080 -v $(PWD)/data:/app/data $(DOCKER_IMAGE)

# Cross-compilation
build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY_NAME)-linux-amd64 .

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY_NAME)-darwin-amd64 .

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY_NAME)-windows-amd64.exe .

build-all: build-linux build-darwin build-windows

# Full production build
prod: clean templ assets build
	@echo "Production build complete: $(BINARY_NAME)"

# Single binary build with embedded static assets (no external files needed)
singlebinary: templ assets
	go build -tags=embed -ldflags="-s -w" -o $(BINARY_NAME) .
	@echo "Single binary build complete: $(BINARY_NAME)"
	@echo "All assets (CSS, JS) are embedded in the binary"

# Single binary cross-compilation
singlebinary-linux:
	GOOS=linux GOARCH=amd64 go build -tags=embed -ldflags="-s -w" -o $(BINARY_NAME)-linux-amd64 .

singlebinary-darwin:
	GOOS=darwin GOARCH=amd64 go build -tags=embed -ldflags="-s -w" -o $(BINARY_NAME)-darwin-amd64 .

singlebinary-windows:
	GOOS=windows GOARCH=amd64 go build -tags=embed -ldflags="-s -w" -o $(BINARY_NAME)-windows-amd64.exe .

singlebinary-all: singlebinary-linux singlebinary-darwin singlebinary-windows

# Development setup
setup: npm-install
	go mod download
	go run github.com/a-h/templ/cmd/templ@latest generate
	@echo "Development setup complete!"

# Lint / check everything
check: typecheck templ
	go vet ./...
	@echo "All checks passed!"
