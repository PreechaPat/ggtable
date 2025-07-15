FROM golang:1.23-alpine3.20 AS builder

# Compile
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY static ./static
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ggtable .

# Copy and run
FROM debian:bookworm-slim AS prod

# Prep the environment
ENV GGTABLE_DATA=/data
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
     && apt-get -y install --no-install-recommends --no-install-suggests ncbi-blast+ samtools \
     && apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/* /tmp/library-scripts /tmp/downloaded_packages 

WORKDIR /app
COPY --from=builder /app/ggtable .
COPY --from=builder /app/static ./static

# Expose the port that the app listens on (if needed)
EXPOSE 8080

# Command to run the binary
CMD ["/app/ggtable"]
