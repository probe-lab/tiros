FROM golang:1.19 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o tiros github.com/probe-lab/tiros

# Create lightweight container
FROM alpine:latest

RUN adduser -D -H tiros
WORKDIR /home/tiros
USER tiros

COPY --from=builder /build/tiros /usr/local/bin/tiros
COPY --from=builder /build/udgerdb_v3.dat /home/tiros/udgerdb_v3.dat

CMD tiros