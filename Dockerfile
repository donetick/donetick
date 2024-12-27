# Stage 1: Build the application
FROM alpine:latest AS builder

WORKDIR /usr/src/app

RUN apk --no-cache add curl jq

RUN latest_release=$(curl --silent "https://api.github.com/repos/dkhalife/donetick/releases/latest" | jq -r .tag_name) && \
    set -ex; \
    apkArch="$(apk --print-arch)"; \
    case "$apkArch" in \
    armhf) arch='armv6' ;; \
    armv7) arch='armv7' ;; \
    aarch64) arch='arm64' ;; \
    x86_64) arch='x86_64' ;; \
    *) echo >&2 "error: unsupported architecture: $apkArch"; exit 1 ;; \
    esac; \
    curl -fL "https://github.com/dkhalife/donetick/releases/download/${latest_release}/donetick_Linux_$arch.tar.gz" | tar -xz -C .

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