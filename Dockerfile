# Multi-stage build : binaire Go statique pour deploiement Docker / k3s.
#
# Build :   docker build -t graphthulhu-vault:dev .
# Run HTTP : docker run --rm -p 7878:7878 \
#             -v "/path/to/vault:/vault:ro" \
#             graphthulhu-vault:dev serve --backend obsidian --vault /vault --http :7878
#
# Note : volume mount macOS degrade les performances fsnotify. Sur macOS,
# preferer le binaire natif via launchd (cf. config/launchd/ du repo hermes).
# Docker reste pertinent pour deploiement k3s ou serveur Linux dedie.

FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/graphthulhu-vault .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/graphthulhu-vault /usr/local/bin/graphthulhu-vault

EXPOSE 7878
ENTRYPOINT ["/usr/local/bin/graphthulhu-vault"]
CMD ["serve", "--backend", "obsidian", "--vault", "/vault", "--http", ":7878"]
