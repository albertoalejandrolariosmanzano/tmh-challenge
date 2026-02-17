package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

// Métricas Prometheus
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total de requests HTTP por endpoint, método y status",
		},
		[]string{"method", "endpoint", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duración de requests HTTP en segundos",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "endpoint"},
	)
	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Duración de queries a DB",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5},
		},
		[]string{"query_type"},
	)
	cacheHitRate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total de cache hits/misses",
		},
		[]string{"status"}, // hit o miss
	)
	ordersCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_created_total",
			Help: "Total de órdenes creadas por sucursal",
		},
		[]string{"branch_id"},
	)
)

const (
	orderStream   = "orders_stream"
	orderGroup    = "order_group"
	orderConsumer = "worker-1"
)

type Order struct {
	ID         int       `json:"id"`
	BranchID   int       `json:"branch_id"`
	CustomerID *int      `json:"customer_id,omitempty"`
	Total      float64   `json:"total"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AnalyticsStats struct {
	TotalOrders    int            `json:"total_orders"`
	TotalRevenue   float64        `json:"total_revenue"`
	AvgOrderValue  float64        `json:"avg_order_value"`
	OrdersByStatus map[string]int `json:"orders_by_status"`
}

var (
	db  *sql.DB
	rdb *redis.Client
	ctx = context.Background()
)

var orderQueueKey = "orders_queue" // buffer interno

func ensureStreamGroup() {
	if rdb == nil {
		return
	}

	err := rdb.XGroupCreateMkStream(ctx, orderStream, orderGroup, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		log.Fatalf("Error creando consumer group: %v", err)
	}

	log.Println("✅ Redis Stream y Consumer Group listos")
}

func initDB() {
	var err error
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_HOST", "postgres"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "tmh_db"),
	)

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error conectando a DB: %v", err)
	}

	// Pool de conexiones
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verificar conexión
	if err = db.Ping(); err != nil {
		log.Fatalf("Error al hacer ping a DB: %v", err)
	}
	log.Println("✅ Conectado a PostgreSQL")
}

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_HOST", "redis") + ":" + getEnv("REDIS_PORT", "6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("⚠️ Redis no disponible: %v (continuando sin cache)", err)
		rdb = nil
	} else {
		log.Println("✅ Conectado a Redis")
	}
}

func main() {
	// Cargar variables desde un archivo .env si existe (útil en desarrollo)
	if err := godotenv.Load("./.env"); err != nil {
		log.Println("No .env file found, continuing with environment variables or defaults")
	} else {
		log.Println("Loaded .env file")
	}
	initDB()
	defer db.Close()
	initRedis()
	ensureStreamGroup()
	startRedisStreamWorker()

	r := mux.NewRouter()
	r.Use(metricsMiddleware)

	// Endpoints de Negocio (TODOS los requeridos)
	r.HandleFunc("/api/v1/orders", createOrder).Methods("POST")
	r.HandleFunc("/api/v1/orders/{id}", getOrder).Methods("GET")
	r.HandleFunc("/api/v1/orders", listOrders).Methods("GET")
	r.HandleFunc("/api/v1/orders/{id}/status", updateOrderStatus).Methods("PATCH")
	r.HandleFunc("/api/v1/orders/{id}", deleteOrder).Methods("DELETE")
	r.HandleFunc("/api/v1/analytics/stats", getAnalyticsStats).Methods("GET")

	// Health Checks
	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/ready", readinessCheck).Methods("GET")

	// Metrics
	r.Handle("/metrics", promhttp.Handler())

	port := getEnv("PORT", "8088")
	log.Printf("🚀 Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// Middleware para métricas
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrapper para capturar status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		endpoint := r.URL.Path
		method := r.Method
		status := strconv.Itoa(rw.statusCode)

		httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
		httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// POST /api/v1/orders - Crear orden
func createOrder(w http.ResponseWriter, r *http.Request) {
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if rdb == nil {
		respondError(w, http.StatusServiceUnavailable, "Queue unavailable")
		return
	}

	data, _ := json.Marshal(order)

	err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: orderStream,
		Values: map[string]interface{}{
			"data": data,
		},
	}).Err()

	if err != nil {
		log.Printf("Error agregando a stream: %v", err)
		respondError(w, http.StatusInternalServerError, "Queue error")
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]string{
		"status": "Order accepted",
	})
}

// GET /api/v1/orders/:id - Obtener orden por ID (con cache)
func getOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	cacheKey := fmt.Sprintf("order:%d", id)

	// Intentar cache
	if rdb != nil {
		val, err := rdb.Get(ctx, cacheKey).Result()
		if err == nil {
			cacheHitRate.WithLabelValues("hit").Inc()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write([]byte(val))
			return
		}
		cacheHitRate.WithLabelValues("miss").Inc()
	}

	// Query a DB
	start := time.Now()
	var order Order
	query := `SELECT id, branch_id, customer_id, total, status, created_at, updated_at 
              FROM orders WHERE id = $1`

	err := db.QueryRow(query, id).Scan(
		&order.ID, &order.BranchID, &order.CustomerID, &order.Total,
		&order.Status, &order.CreatedAt, &order.UpdatedAt,
	)
	dbQueryDuration.WithLabelValues("select").Observe(time.Since(start).Seconds())

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Order not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Guardar en cache
	if rdb != nil {
		data, _ := json.Marshal(order)
		rdb.Set(ctx, cacheKey, data, 60*time.Second)
	}

	w.Header().Set("X-Cache", "MISS")
	respondJSON(w, http.StatusOK, order)
}

// GET /api/v1/orders - Listar órdenes (paginado)
func listOrders(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	start := time.Now()
	query := `SELECT id, branch_id, customer_id, total, status, created_at, updated_at 
              FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		var o Order
		rows.Scan(&o.ID, &o.BranchID, &o.CustomerID, &o.Total, &o.Status, &o.CreatedAt, &o.UpdatedAt)
		orders = append(orders, o)
	}
	dbQueryDuration.WithLabelValues("select_list").Observe(time.Since(start).Seconds())

	respondJSON(w, http.StatusOK, orders)
}

// PATCH /api/v1/orders/:id/status - Actualizar estado
func updateOrderStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	start := time.Now()
	query := `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`
	result, err := db.Exec(query, payload.Status, id)
	dbQueryDuration.WithLabelValues("update").Observe(time.Since(start).Seconds())

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		respondError(w, http.StatusNotFound, "Order not found")
		return
	}

	// Invalidar cache
	if rdb != nil {
		rdb.Del(ctx, fmt.Sprintf("order:%d", id))
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE /api/v1/orders/:id - Cancelar orden
func deleteOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	start := time.Now()
	query := `UPDATE orders SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1`
	result, err := db.Exec(query, id)
	dbQueryDuration.WithLabelValues("delete").Observe(time.Since(start).Seconds())

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		respondError(w, http.StatusNotFound, "Order not found")
		return
	}

	if rdb != nil {
		rdb.Del(ctx, fmt.Sprintf("order:%d", id))
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/analytics/stats - Métricas de negocio
func getAnalyticsStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var stats AnalyticsStats
	query := `SELECT 
        COUNT(*) as total_orders,
        COALESCE(SUM(total), 0) as total_revenue,
        COALESCE(AVG(total), 0) as avg_order_value
    FROM orders WHERE created_at >= NOW() - INTERVAL '24 hours'`

	db.QueryRow(query).Scan(&stats.TotalOrders, &stats.TotalRevenue, &stats.AvgOrderValue)

	// Órdenes por status
	stats.OrdersByStatus = make(map[string]int)
	rows, _ := db.Query(`SELECT status, COUNT(*) FROM orders 
                         WHERE created_at >= NOW() - INTERVAL '24 hours' 
                         GROUP BY status`)
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		stats.OrdersByStatus[status] = count
	}

	dbQueryDuration.WithLabelValues("analytics").Observe(time.Since(start).Seconds())
	respondJSON(w, http.StatusOK, stats)
}

// GET /health - Health check básico
func healthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /ready - Readiness probe (verifica DB)
func readinessCheck(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		respondError(w, http.StatusServiceUnavailable, "Database not ready")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// Helpers
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// Worker para procesar órdenes desde Redis Stream
func startRedisStreamWorker() {
	go func() {
		log.Println("🔄 Redis Stream Worker iniciado")

		for {
			if rdb == nil {
				time.Sleep(3 * time.Second)
				continue
			}

			streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    orderGroup,
				Consumer: orderConsumer,
				Streams:  []string{orderStream, ">"},
				Count:    1,
				Block:    0,
			}).Result()

			if err != nil {
				log.Printf("Error leyendo stream: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {

					raw := message.Values["data"].(string)

					var order Order
					if err := json.Unmarshal([]byte(raw), &order); err != nil {
						log.Printf("Error deserializando orden: %v", err)
						continue
					}

					err = insertOrderToDB(&order)
					if err != nil {
						log.Printf("⚠️ DB caída, no se hace ACK. Reintentará...")
						continue
					}

					// ACK solo si insertó bien
					rdb.XAck(ctx, orderStream, orderGroup, message.ID)
					log.Printf("✅ Orden procesada y ACK enviada (%s)", message.ID)
				}
			}
		}
	}()
}

// Inserta la orden en la DB y actualiza el struct con ID y timestamps generados
func insertOrderToDB(order *Order) error {
	query := `INSERT INTO orders 
		(branch_id, customer_id, total, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'PENDING', NOW(), NOW())
		RETURNING id, created_at, updated_at`

	return db.QueryRow(
		query,
		order.BranchID,
		order.CustomerID,
		order.Total,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
}

// Helpers para respuestas JSON
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Respuesta de error con formato JSON
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
