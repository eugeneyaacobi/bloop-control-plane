FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bloop-control-plane ./cmd/bloop-control-plane

FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/bloop-control-plane /bloop-control-plane
ENTRYPOINT ["/bloop-control-plane"]
