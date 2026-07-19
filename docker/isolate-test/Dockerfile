# Use an official Golang image as a base
FROM cskulab/isolate-with-compilers:1.1.1

# Install dependencies
RUN apt-get update && apt-get install -y curl tar git

# Install Go
ENV GO_VERSION=1.24.2
RUN curl -OL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz && \
    rm go${GO_VERSION}.linux-amd64.tar.gz

# Set Go environment variables
ENV PATH="/usr/local/go/bin:${PATH}"

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire Go application to the container
COPY . .

# Set environment variables for cross-compilation (Linux, AMD64)
ENV GOARCH=amd64
ENV GOOS=linux

ENTRYPOINT ["go" ,"test", "-v" ,"./tests/integration"]
