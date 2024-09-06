# Stage 1: Build the application
FROM golang:1.22-alpine AS builder

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -buildvcs=false -o /donetick

# Stage 2: Create a smaller runtime image
FROM alpine:latest

# Install necessary CA certificates
RUN apk --no-cache add ca-certificates libc6-compat

# Copy the binary and config folder from the builder stage
COPY --from=builder /donetick /donetick
COPY --from=builder /usr/src/app/config /config

# Set environment variables
ENV DT_ENV="selfhosted"

# Expose the application port
EXPOSE 2021

# Command to run the application
CMD ["/donetick"]