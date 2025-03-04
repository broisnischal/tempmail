# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o tempmail

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/tempmail .
EXPOSE 25 8080
RUN adduser -D tempmail
USER tempmail

COPY .env .

HEALTHCHECK --interval=5s --timeout=5s CMD curl -f http://localhost:8080/health || exit 1

CMD ["./tempmail"]