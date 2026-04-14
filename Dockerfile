# Stage 1: Build frontend (consumer app)
FROM node:20.18-alpine3.20 AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ ./
RUN npm run build
# Verify frontend build produced valid output (catches missing JS bundles)
RUN set -e && \
    test -f /app/static/index.html || (echo "ERROR: index.html missing" && exit 1) && \
    test -d /app/static/assets || (echo "ERROR: assets/ directory missing" && exit 1) && \
    JS_COUNT=$(ls /app/static/assets/*.js 2>/dev/null | wc -l) && \
    test "$JS_COUNT" -gt 0 || (echo "ERROR: No JS bundles in assets/" && exit 1) && \
    for asset in $(grep -oE '/assets/[^"]+' /app/static/index.html); do \
        test -f "/app/static${asset}" || (echo "ERROR: Missing referenced asset: ${asset}" && exit 1); \
    done && \
    echo "Frontend build OK: ${JS_COUNT} JS bundles, all references valid"

# Stage 1b: Build brand portal frontend
FROM node:20.18-alpine3.20 AS frontend-brand
WORKDIR /app/frontend-brand
COPY frontend-brand/package.json frontend-brand/package-lock.json* ./
RUN npm ci
COPY frontend-brand/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o groflo .

# Stage 3: Final image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=backend /app/groflo .
COPY --from=frontend /app/static ./static
COPY --from=frontend-brand /app/static-brand ./static-brand
EXPOSE 8080

CMD ["./groflo"]
