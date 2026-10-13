# syntax=docker/dockerfile:1

# ---------- tahap build ----------
FROM golang:1.26-alpine AS build
WORKDIR /src

# cache dependency dulu
COPY go.mod go.sum ./
RUN go mod download

# salin source lalu build binary statis (tanpa CGO)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dbprobe ./cmd/dbprobe

# ---------- tahap final ----------
FROM alpine:3.20

# ca-certificates: untuk TLS (mis. PostgreSQL sslmode=require / Oracle wallet)
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 dbprobe

COPY --from=build /out/dbprobe /usr/local/bin/dbprobe

USER dbprobe
WORKDIR /work

ENTRYPOINT ["dbprobe"]
CMD ["engines"]
