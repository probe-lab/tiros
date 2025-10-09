FROM golang:1.25 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -o tiros github.com/probe-lab/tiros

# Create lightweight container
FROM alpine:latest

RUN adduser -D -H tiros
WORKDIR /home/tiros
USER tiros

COPY --from=builder /build/tiros /usr/local/bin/tiros

CMD tiros