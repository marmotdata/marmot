FROM golang:1.24 as builder

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 go build -o marmot ./cmd/main.go

FROM alpine:3.18

COPY --from=builder /app/marmot /usr/local/bin/

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/marmot"]
CMD ["run"]
