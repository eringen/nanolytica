# Build stage
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

# Install dependencies for templ
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

# Install templ CLI
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy Go source files
COPY . .

# Generate templ files
RUN templ generate

# Build Go binary
RUN go build -ldflags="-s -w" -o nanolytica .

# Node build stage for assets
FROM node:20-alpine AS node-builder

WORKDIR /app

# Copy package files
COPY package*.json ./
RUN npm install

# Copy source files for Tailwind content scanning and TypeScript compilation
COPY fe_src/ ./fe_src/
COPY analytics/templates/ ./analytics/templates/
COPY scripts/ ./scripts/
COPY tailwind.config.js ./

# Build assets
RUN npm run build

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -S nanolytica && adduser -S nanolytica -G nanolytica

WORKDIR /app

# Copy Go binary
COPY --from=go-builder /app/nanolytica .

# Copy built assets
COPY --from=node-builder /app/static/css/ ./static/css/
COPY --from=node-builder /app/static/js/ ./static/js/

# Create data directory and set ownership
RUN mkdir -p data && chown -R nanolytica:nanolytica /app

# Switch to non-root user
USER nanolytica

# Expose port
EXPOSE 8080

# Run
CMD ["./nanolytica"]
