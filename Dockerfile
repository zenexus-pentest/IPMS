# ── Stage 1: Build React Frontend ──────────────────────────────────────
FROM node:20-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ── Stage 2: Build Go Backend ─────────────────────────────────────────
FROM golang:1.21-alpine AS backend-build
# CGO is required for go-sqlite3
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go-backend/go.mod go-backend/go.sum ./
RUN go mod download
COPY go-backend/ ./
RUN CGO_ENABLED=1 go build -o server ./cmd

# ── Stage 3: Production Image ─────────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app

# Copy the compiled Go binary
COPY --from=backend-build /app/server ./server

# Copy the built frontend dist into the expected path
COPY --from=frontend-build /app/frontend/dist ./frontend/dist

# Render sets PORT env var automatically
ENV PORT=8080
ENV DEMO_MODE=true
ENV NODE_ENV=production

EXPOSE 8080

CMD ["./server"]
