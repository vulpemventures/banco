# Use the official Go image as the base image for building
FROM golang:1.21-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download and install the Go dependencies
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the Go application
RUN go build -o ./bin/banco .

# Create a new stage for the final image
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /app


# Copy the web folder with HTML templates
COPY web/ /app/web/

# Copy the built binary from the previous stage
COPY --from=builder /app/bin/banco /app/banco
COPY --from=builder /app/web/. /app/web


# Expose the port that the server listens on
EXPOSE 8080

# Set the command to run the server when the container starts
ENTRYPOINT ["/app/banco"]
