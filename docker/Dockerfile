FROM golang:1.25.10-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY internal internal
COPY cmd cmd

RUN CGO_ENABLED=1 go build -o /slurmtack ./cmd/

FROM alpine:3.23

RUN apk add --no-cache ca-certificates openssh-client shadow su-exec sqlite && \
    adduser -D -h /home/slurmtack slurmtack && \
    mkdir -p /data /home/slurmtack/.ssh /run/secrets && \
    chown -R slurmtack:slurmtack /data /home/slurmtack && \
    chmod 700 /home/slurmtack/.ssh

COPY --from=build /slurmtack /usr/local/bin/slurmtack
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENV HOME=/home/slurmtack
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
