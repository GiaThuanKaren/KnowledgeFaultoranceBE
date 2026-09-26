# Stage 1: Build binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates git tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/server ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/internal/database/migrations /app/migrations

USER appuser

EXPOSE 8080

ENV PORT=8080
ENV ENVIRONMENT=production

ENTRYPOINT ["/app/server"]
