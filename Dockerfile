# Stage 1: Build the application
FROM alpine:latest AS builder

WORKDIR /usr/src/app

RUN apk --no-cache add curl jq

RUN latest_release=$(curl --silent "https://api.github.com/repos/donetick/donetick/releases/latest" | jq -r .tag_name) && \
curl -fL "https://github.com/donetick/donetick/releases/download/${latest_release}/donetick_Linux_x86_64.tar.gz" | tar -xz -C .

# Stage 2: Create a smaller runtime image
FROM alpine:latest

# Install necessary CA certificates
RUN apk --no-cache add ca-certificates libc6-compat

# Copy the binary and config folder from the builder stage
COPY --from=builder /usr/src/app/donetick /donetick
COPY --from=builder /usr/src/app/config /config

# Set environment variables
ENV DT_ENV="selfhosted"

# Expose the application port
EXPOSE 2021

# Command to run the application
CMD ["/donetick"]