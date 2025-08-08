FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /tmp/api-server ./cmd/api-server
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /tmp/controller ./cmd/controller

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

WORKDIR /root/

COPY --from=builder /tmp/api-server .
COPY --from=builder /tmp/controller .

EXPOSE 8080

CMD ["./api-server"]