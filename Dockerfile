FROM golang:1.22.4-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /slurmtack ./cmd/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates openssh-client && \
    adduser -D -h /home/slurmtack slurmtack && \
    mkdir -p /data /home/slurmtack/.ssh /run/secrets && \
    chown -R slurmtack:slurmtack /data /home/slurmtack && \
    chmod 700 /home/slurmtack/.ssh

COPY --from=build /slurmtack /usr/local/bin/slurmtack

ENV HOME=/home/slurmtack
USER slurmtack
ENTRYPOINT ["slurmtack"]
