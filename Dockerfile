# Stage 1: Build frontend (consumer app)
FROM node:20.18-alpine3.20 AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 1b: Build brand portal frontend
FROM node:20.18-alpine3.20 AS frontend-brand
WORKDIR /app/frontend-brand
COPY frontend-brand/package.json frontend-brand/package-lock.json* ./
RUN npm ci
COPY frontend-brand/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o thriftly .

# Stage 3: Final image with Litestream for continuous SQLite replication
FROM alpine:3.20
RUN apk add --no-cache ca-certificates

# Install Litestream for continuous SQLite backup to S3/R2
ADD https://github.com/benbjohnson/litestream/releases/download/v0.3.13/litestream-v0.3.13-linux-amd64.tar.gz /tmp/litestream.tar.gz
RUN tar -xzf /tmp/litestream.tar.gz -C /usr/local/bin && rm /tmp/litestream.tar.gz

WORKDIR /app
COPY --from=backend /app/thriftly .
COPY --from=frontend /app/static ./static
COPY --from=frontend-brand /app/static-brand ./static-brand
COPY litestream.yml /etc/litestream.yml
COPY run.sh .
RUN chmod +x run.sh
EXPOSE 8080

# run.sh conditionally starts litestream if replica env vars are set
CMD ["./run.sh"]
