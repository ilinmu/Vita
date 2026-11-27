FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/vita-server ./cmd/vita-server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/vita-server /vita-server
VOLUME ["/data"]
EXPOSE 8420
ENV VITA_ADDR=":8420" \
    VITA_DATA_DIR="/data" \
    VITA_ALLOW_PUBLIC_REGISTER="true"
ENTRYPOINT ["/vita-server"]
