FROM golang:1.21-alpine as builder
WORKDIR /app
RUN apk add --no-cache git protobuf protobuf-dev
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /payment-service

FROM gcr.io/distroless/static-debian12
COPY --from=builder /payment-service /usr/bin/payment-service
EXPOSE 8083
ENTRYPOINT ["/usr/bin/payment-service"]
