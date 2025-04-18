# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder
# Set the working directory inside the container
WORKDIR /app
# Copy the Go modules manifests
COPY go.mod go.sum ./
# Download the Go modules
RUN go mod download
# Copy the source code
COPY . .
# Build the Go application
RUN go build -o main .



# Stage 2: Create the final image
FROM alpine:latest
# Set the working directory inside the container
WORKDIR /root/
# Copy the built Go application from the builder stage
COPY --from=builder /app/main .
# Expose the port the application runs on
EXPOSE 8080
# Command to run the application
CMD ["./main"]