FROM golang:1.26.2 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -o tiros ./cmd/tiros

# Create lightweight container
FROM alpine:latest

RUN adduser -D -H tiros
WORKDIR /home/tiros
USER tiros

COPY --from=builder /build/tiros /usr/local/bin/tiros

CMD tiros