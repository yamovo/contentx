# Build stage
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /contentx ./cmd/server/main.go

# Frontend build stage
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ .
RUN npm run build

# Production stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata

# Run as non-root user
RUN adduser -D -h /app contentx
WORKDIR /app

# Copy binary
COPY --from=backend-builder /contentx .

# Copy frontend
COPY --from=frontend-builder /app/web/dist ./web/dist

# Create directories
RUN mkdir -p uploads backups logs plugins themes && chown -R contentx:contentx /app

# Copy config template
COPY deploy/docker/.env.example .env

EXPOSE 8080

VOLUME ["/app/uploads", "/app/backups", "/app/logs"]

USER contentx

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:8080/api/v1/system/health || exit 1

ENTRYPOINT ["./contentx"]
