#FROM golang:1.24.1-alpine3.21 AS build-stage
#
#WORKDIR /app
#
#COPY go.mod go.sum ./
#
#RUN go mod download
#
#COPY . .
#
#RUN CGO_ENABLED=0 GOOS=linux go build -o ./main.exe ./main.go
#
#FROM build-stage AS dev-stage
#
#WORKDIR /app
#
#COPY --from=build-stage /app/main.exe /main
#
#ENTRYPOINT ["./main.exe"]

# Base stage with Go
FROM golang:1.25-alpine3.21 AS base

WORKDIR /app

# Install air and other dependencies
RUN apk add --no-cache git curl && \
    go install github.com/air-verse/air@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Development stage with hot reload
FROM base AS dev-stage

WORKDIR /app

# Copy air config
COPY ./.air.toml ./

# Copy source code
COPY . .

# Expose port
EXPOSE 5001

# Run air for hot reload
CMD ["air", "-c", ".air.toml"]

# Production build stage
FROM golang:1.24.1-alpine3.21 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./main.go

# Production stage
FROM alpine:latest AS prod-stage

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from build stage
COPY --from=build-stage /app/main .

# Expose port
EXPOSE 5001

# Run binary
CMD ["./main"]