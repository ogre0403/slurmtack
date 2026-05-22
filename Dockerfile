FROM golang:1.22-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /slurmtack ./cmd/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    adduser -D -H slurmtack && \
    mkdir -p /data && chown slurmtack /data

COPY --from=build /slurmtack /usr/local/bin/slurmtack

USER slurmtack
ENTRYPOINT ["slurmtack"]
