FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /tmp/ssvirt-api-server ./cmd/api-server

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

WORKDIR /root/

COPY --from=builder /tmp/ssvirt-api-server /usr/local/bin/

EXPOSE 8080

CMD ["ssvirt-api-server"]