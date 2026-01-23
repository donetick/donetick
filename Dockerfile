# Stage 1: Build the application
FROM alpine:latest AS builder

WORKDIR /usr/src/app

RUN apk --no-cache add curl jq tzdata

# Accept VERSION as build argument, fallback to latest stable release if not provided
ARG VERSION
RUN if [ -z "$VERSION" ]; then \
        VERSION=$(curl --silent "https://api.github.com/repos/donetick/donetick/releases/latest" | jq -r .tag_name); \
    fi && \
    echo "Downloading version: $VERSION" && \
    set -ex; \
    apkArch="$(apk --print-arch)"; \
    case "$apkArch" in \
    armhf) arch='armv6' ;; \
    armv7) arch='armv7' ;; \
    aarch64) arch='arm64' ;; \
    x86_64) arch='x86_64' ;; \
    *) echo >&2 "error: unsupported architecture: $apkArch"; exit 1 ;; \
    esac; \
    curl -fL "https://github.com/donetick/donetick/releases/download/${VERSION}/donetick_Linux_$arch.tar.gz" | tar -xz -C .

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