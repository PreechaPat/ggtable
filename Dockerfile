# Builder image
FROM golang:1.25.9-alpine3.23 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY static ./static
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ggtable .

# Runtime image
FROM debian:trixie-slim AS prod

ARG VERSION
LABEL org.opencontainers.image.version=${VERSION}
LABEL maintainer="Preecha Patumcharoenpol"

# Prep the environment
ENV GGTABLE_DATA=/data
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
     && apt-get -y install --no-install-recommends --no-install-suggests curl ncbi-blast+ \
     && apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/* /tmp/library-scripts /tmp/downloaded_packages 

WORKDIR /app
COPY --from=builder /app/ggtable .
COPY --from=builder /app/static ./static

# Expose the port that the app listens on (if needed)
EXPOSE 8080

# Healthcheck configuration
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/api/v1/health || exit 1

# Command to run the binary
CMD ["/app/ggtable"]
