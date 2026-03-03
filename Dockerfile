# Build stage
FROM golang:latest AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o chough ./cmd/chough

# Runtime stage
FROM debian:stable-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /build/chough /usr/local/bin/chough
COPY --from=builder /go/pkg/mod/github.com/k2-fsa/sherpa-onnx-go-linux@v1.12.26/lib/x86_64-unknown-linux-gnu/*.so /usr/local/lib/
ENV LD_LIBRARY_PATH=/usr/local/lib
EXPOSE 8080
ENTRYPOINT ["chough"]
CMD ["--server", "--host", "0.0.0.0", "--port", "8080"]
