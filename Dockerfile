FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bin/coordinator main.go

FROM alpine
COPY --from=builder /app/bin/coordinator /coordinator
ENTRYPOINT ["/coordinator"]
