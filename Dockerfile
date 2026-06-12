FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./src/cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o /oracle ./src/cmd/oracle

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /gateway /gateway
COPY --from=builder /oracle /oracle
EXPOSE 8080
CMD ["/gateway"]
