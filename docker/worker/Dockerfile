# Use an official Golang image as a base
FROM golang:1.24-alpine AS build

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules and download dependencies
COPY go.mod go.sum ./
RUN go mod tidy

# Copy the entire Go application to the container
COPY . .

# Set environment variables for cross-compilation (Linux, AMD64)
ENV GOARCH=amd64
ENV GOOS=linux

# Build the Go app
RUN go build -o worker cmd/worker/worker.go

FROM sornchaithedev/all-isolate:latest

# Copy the binary from the build container to the final container
COPY --from=build /app/worker /app/worker

ENTRYPOINT ["/app/worker"]
