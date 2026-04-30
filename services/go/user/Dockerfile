FROM golang:1.21-alpine as builder
WORKDIR /app
RUN apk add --no-cache git protobuf protobuf-dev
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /user-service

FROM gcr.io/distroless/static-debian12
COPY --from=builder /user-service /usr/bin/user-service
EXPOSE 8082
ENTRYPOINT ["/usr/bin/user-service"]
