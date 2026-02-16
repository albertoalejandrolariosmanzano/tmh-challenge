# Build Stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copiar archivos de dependencias
COPY go.mod .
COPY go.sum* .
RUN go mod download

# Copiar código fuente
COPY *.go .
COPY scripts/loader.go ./scripts/loader.go

# Build del binario (optimizado)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o api-server \
    main.go

# Build del script de carga masiva
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o loader \
    scripts/loader.go

# Run Stage (imagen mínima)
FROM alpine:3.18

# Instalar herramientas necesarias (wget para healthcheck)
RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copiar binario desde builder
COPY --chmod=755 --from=builder /app/api-server .
COPY --chmod=755 --from=builder /app/loader ./scripts/loader

# DAR PERMISOS DE EJECUCIÓN
RUN chmod +x /app/scripts/loader

# Crear usuario no-root (REQUERIDO por el challenge)
RUN adduser -D -u 1001 nonroot && \
    chown -R nonroot:nonroot /app

USER nonroot

EXPOSE 8080

# Health check interno
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./api-server"]