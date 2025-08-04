FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api-server ./cmd/api-server
RUN CGO_ENABLED=0 GOOS=linux go build -o controller ./cmd/controller

FROM registry.access.redhat.com/ubi9/ubi:latest

WORKDIR /root/

COPY --from=builder /app/api-server .
COPY --from=builder /app/controller .

EXPOSE 8080

CMD ["./api-server"]