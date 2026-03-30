FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bloop-control-plane ./cmd/bloop-control-plane

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/bloop-control-plane /app/bloop-control-plane
ENTRYPOINT ["/app/bloop-control-plane"]
