FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY main.go .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o ai-gateway .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/ai-gateway .
EXPOSE 3000
CMD ["./ai-gateway"]
