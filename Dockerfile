# Use the official Go image as the base image for building
FROM golang:1.21-alpine AS builder

# Set the working directory inside the container
WORKDIR /builder

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

# Copy the built binary from the previous stage
COPY --from=builder /builder/bin/banco /app/banco


# Set a new directory for the web files
WORKDIR /web
COPY --from=builder /builder/web/ . 


# Go back to the app directory
WORKDIR /app

# Expose the port that the server listens on
EXPOSE 8080

# Use the absolute path for the /web folder to have two separate volumes
ENV WEB_DIR /web

# Declare a volume for the database
VOLUME /app

# Declare a volume for the web files
VOLUME /web

# Set the command to run the server when the container starts
ENTRYPOINT ["/app/banco"]
