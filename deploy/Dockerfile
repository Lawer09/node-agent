FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/node-agent ./cmd/agent

FROM debian:stable-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/node-agent /usr/local/bin/node-agent
WORKDIR /app
COPY configs/config.yaml /app/config.yaml
ENV CONFIG_PATH=/app/config.yaml
EXPOSE 2112
CMD ["/usr/local/bin/node-agent"]
