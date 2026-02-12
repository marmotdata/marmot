FROM node:20-alpine AS frontend

RUN corepack enable && corepack prepare pnpm@9 --activate

WORKDIR /app/web/marmot
COPY web/marmot/package.json web/marmot/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/marmot/ ./
RUN pnpm build

FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /app/web/marmot/build ./internal/staticfiles/build
RUN CGO_ENABLED=0 go build -tags production -o marmot ./cmd/main.go

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 10001 marmot

WORKDIR /app

COPY --from=builder /app/marmot /usr/local/bin/
RUN chmod +x /usr/local/bin/marmot && \
    chown marmot:marmot /usr/local/bin/marmot

USER marmot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/marmot"]
CMD ["run"]
