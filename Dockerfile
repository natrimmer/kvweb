# Stage 1: Build frontend
FROM node:24-alpine AS frontend
WORKDIR /app/web
RUN corepack enable && corepack prepare pnpm@10 --activate
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY static/ static/
COPY --from=frontend /app/web/dist/ static/dist/
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=$VERSION -X main.commit=$COMMIT" -o /kvweb ./cmd/kvweb

# Stage 3: Minimal runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D kvweb
USER kvweb
COPY --from=builder /kvweb /kvweb
EXPOSE 8080
ENTRYPOINT ["/kvweb", "-host", "0.0.0.0"]
