# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/front
COPY front/package*.json ./
RUN npm ci --no-audit --no-fund
COPY front/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.22-alpine AS backend
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /z-ui .

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend /z-ui /opt/z-ui/backend/z-ui
COPY --from=frontend /app/front/dist /opt/z-ui/front/dist
WORKDIR /opt/z-ui/backend
EXPOSE 8081
VOLUME ["/opt/z-ui/data", "/opt/z-ui/runtime"]
CMD ["/opt/z-ui/backend/z-ui"]
