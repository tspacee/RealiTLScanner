FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
# Build with optimizations: disable debug info and symbol table
RUN go build -ldflags="-s -w" -o RealiTLScanner .

FROM alpine:latest
# ca-certificates needed for TLS verification, tzdata for correct timezone logging
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /src/RealiTLScanner .
ENTRYPOINT ["./RealiTLScanner"]
