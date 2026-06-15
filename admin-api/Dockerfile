FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o cont-admin-api .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/cont-admin-api .
COPY --from=builder /app/docs ./docs
EXPOSE 8001
ENTRYPOINT ["./cont-admin-api"]
