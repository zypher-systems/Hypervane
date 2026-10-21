FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/api /app/api
ARG VERSION=dev
LABEL org.opencontainers.image.title="hypervane-api" \
      org.opencontainers.image.version="${VERSION}"
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/api"]
