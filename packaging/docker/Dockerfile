# Stage 1: Build Stage
FROM golang:alpine AS go-builder

# Install build tools
RUN apk add --no-cache make git gcc g++

ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Download swaggo for generating swagger docs
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Generate swagger docs
RUN make docs

# Build the Go app using the Makefile
RUN make build

####################################################################
FROM node:alpine AS node-builder

WORKDIR /ui

# copy the UI source code from the host machine folder ottermq_ui to the root of the container
COPY ottermq_ui ./
RUN npm install
RUN npm run build

####################################################################
# Stage 3: Final Stage
FROM alpine:latest

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the built binaries from the builder stage
COPY --from=go-builder /app/bin/ottermq .
COPY --from=node-builder /ui/dist/spa ./ui/
# Expose ports 3000 for the web admin, 5672 for the broker and 9090 for prometheus exporter
EXPOSE 3000
EXPOSE 5672
EXPOSE 9090

# Command to run both broker and web admin binaries
CMD ["./ottermq"]
