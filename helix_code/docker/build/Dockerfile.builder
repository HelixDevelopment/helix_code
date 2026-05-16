# HelixCode Builder Container
# Provides ALL build dependencies so nothing needs to be installed on the host.
# Usage: docker compose -f docker-compose.builder.yml run --rm builder make build
#
# Authority: CONST-035 — End-User Usability Mandate
# All builds MUST be reproducible and containerized.

FROM docker.io/golang:1.26-alpine

# Install ALL build dependencies — host machine needs nothing but Docker/Podman
RUN apk add --no-cache \
    git \
    bash \
    make \
    coreutils \
    ca-certificates \
    tzdata \
    gcc \
    g++ \
    musl-dev \
    linux-headers \
    postgresql-client \
    redis \
    openssh-client \
    docker-cli \
    docker-compose \
    curl \
    jq \
    sed \
    findutils

# Install golangci-lint for linting inside container
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin v1.64.5

# Set Go environment
ENV CGO_ENABLED=1
ENV GO111MODULE=on

# Working directory matches project layout inside container
WORKDIR /workspace/HelixCode

# Pre-download dependencies for faster incremental builds
COPY HelixCode/go.mod HelixCode/go.sum ./
RUN go mod download

# Default entrypoint is a bash shell for interactive use
ENTRYPOINT ["/bin/bash"]
CMD ["-c", "echo 'HelixCode Builder Ready. Run: make build' && /bin/bash"]
