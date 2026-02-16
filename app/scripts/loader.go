package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var (
	db *sql.DB
)

// Patrones de tráfico
var hourlyTrafficPattern = map[int]int{
	0:  2,
	1:  1,
	2:  1,
	3:  1,
	4:  1,
	5:  1,
	6:  5,
	7:  15,
	8:  30, // Hora desayuno
	9:  25,
	10: 15,
	11: 20,
	12: 50,
	13: 60, // Hora comida
	14: 45,
	15: 20,
	16: 15,
	17: 25,
	18: 40,
	19: 70, // Hora cena
	20: 80, // Hora pico
	21: 60,
	22: 35,
	23: 10,
}

const weekendMultiplier = 1.4

var statuses = []string{"PENDING", "PREPARING", "READY", "COMPLETED", "CANCELLED"}
var statusWeights = []int{5, 10, 5, 75, 5}

type Order struct {
	ID         int64
	BranchID   int
	CustomerID int
	Total      float64
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func generateRealisticTimestamp(daysBack int) time.Time {
	// Fecha aleatoria
	rand.Seed(time.Now().UnixNano() + rand.Int63())
	baseDate := time.Now().AddDate(0, 0, -rand.Intn(daysBack))

	// Aplicar patrón horario
	var hours []int
	var weights []int
	for h, w := range hourlyTrafficPattern {
		hours = append(hours, h)
		weights = append(weights, w)
	}

	hour := weightedRandomChoice(hours, weights)

	// Aplicar multiplicador fin de semana
	isWeekend := baseDate.Weekday() == time.Saturday || baseDate.Weekday() == time.Sunday
	if isWeekend && rand.Float64() < weekendMultiplier-1 {
		peakHours := []int{12, 13, 19, 20}
		hour = peakHours[rand.Intn(len(peakHours))]
	}

	return time.Date(
		baseDate.Year(), baseDate.Month(), baseDate.Day(),
		hour, rand.Intn(60), rand.Intn(60), 0, time.UTC,
	)
}

func weightedRandomChoice(items []int, weights []int) int {
	total := 0
	for _, w := range weights {
		total += w
	}

	r := rand.Intn(total)
	cumulative := 0
	for i, w := range weights {
		cumulative += w
		if r < cumulative {
			return items[i]
		}
	}
	return items[0]
}

func generateBatch(size int) []Order {
	orders := make([]Order, size)

	for i := range size {
		// Branch ID (1-100)
		branchID := rand.Intn(100) + 1
		customerID := rand.Intn(10000) + 1

		// Total realista
		var total float64
		r := rand.Float64()
		switch {
		case r < 0.7: // 70% de probabilidad (r entre 0.0 y 0.6999)
			total = 50.0 + rand.Float64()*250.0 // 50-300
		case r < 0.9: // 20% de probabilidad (r entre 0.7 y 0.8999)
			total = 300.0 + rand.Float64()*500.0 // 300-800
		default: // 10% de probabilidad (r entre 0.9 y 1.0)
			total = 800.0 + rand.Float64()*1200.0 // 800-2000
		}

		// Status con pesos
		status := weightedRandomChoiceString(statuses, statusWeights)

		createdAt := generateRealisticTimestamp(180)

		orders[i] = Order{
			ID:         0,
			BranchID:   branchID,
			CustomerID: customerID,
			Total:      roundToTwoDecimals(total),
			Status:     status,
			CreatedAt:  createdAt,
			UpdatedAt:  createdAt,
		}
	}

	return orders
}

func weightedRandomChoiceString(items []string, weights []int) string {
	total := 0
	for _, w := range weights {
		total += w
	}

	r := rand.Intn(total)
	cumulative := 0
	for i, w := range weights {
		cumulative += w
		if r < cumulative {
			return items[i]
		}
	}
	return items[0]
}

func roundToTwoDecimals(value float64) float64 {
	return float64(int(value*100)) / 100
}

func createIndexes(db *sql.DB) error {
	fmt.Println("\n📊 Creando índices optimizados...")

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_orders_branch_created ON orders(branch_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status) WHERE status != 'COMPLETED'",
		"CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders(customer_id) WHERE customer_id IS NOT NULL",
	}

	for _, idxSQL := range indexes {
		_, err := db.Exec(idxSQL)
		if err != nil {
			return fmt.Errorf("error creando índice: %v", err)
		}
	}

	fmt.Println("✅ Índices creados")
	return nil
}

func verifyDataIntegrity(db *sql.DB) error {
	fmt.Println("\n🔍 Verificando integridad de datos...")

	// Total de registros
	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&total)
	if err != nil {
		return err
	}
	fmt.Printf("   Total registros: %d\n", total)

	// Distribución por status
	rows, err := db.Query("SELECT status, COUNT(*) FROM orders GROUP BY status ORDER BY status")
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("\n   Distribución por status:")
	for rows.Next() {
		var status string
		var count int
		err := rows.Scan(&status, &count)
		if err != nil {
			return err
		}
		pct := float64(count) / float64(total) * 100
		fmt.Printf("     %-12s : %7d (%5.2f%%)\n", status, count, pct)
	}

	// Top 5 sucursales
	rows, err = db.Query(`
		SELECT branch_id, COUNT(*) as orders, SUM(total) as revenue 
		FROM orders 
		GROUP BY branch_id 
		ORDER BY orders DESC 
		LIMIT 5
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("\n   Top 5 sucursales por volumen:")
	for rows.Next() {
		var branchID int
		var orders int
		var revenue float64
		err := rows.Scan(&branchID, &orders, &revenue)
		if err != nil {
			return err
		}
		fmt.Printf("     Sucursal %3d : %5d órdenes - $%.2f\n", branchID, orders, revenue)
	}

	// Distribución por hora
	rows, err = db.Query(`
		SELECT EXTRACT(HOUR FROM created_at) as hour, COUNT(*) 
		FROM orders 
		GROUP BY hour 
		ORDER BY hour
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("\n   Distribución por hora del día:")
	for rows.Next() {
		var hour int
		var count int
		err := rows.Scan(&hour, &count)
		if err != nil {
			return err
		}
		barCount := count / 2000
		bar := ""
		for i := 0; i < barCount && i < 50; i++ {
			bar += "█"
		}
		fmt.Printf("%02d:00 - %6d %s\n", hour, count, bar)
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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

	// Verificar conexión
	if err = db.Ping(); err != nil {
		log.Fatalf("Error al hacer ping a DB: %v", err)
	}
	log.Println("✅ Conectado a PostgreSQL")
}

func main() {
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Println("  CARGA MASIVA DE DATOS - TMH Challenge")
	fmt.Println("=" + strings.Repeat("=", 59))

	// Leer argumentos
	records := 500000
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &records)
	}

	if err := godotenv.Load("/app/.env"); err != nil {
		log.Println("No .env file found, continuing with environment variables or defaults")
	} else {
		log.Println("Loaded .env file")
	}
	initDB()
	defer db.Close()

	// Verificar si ya hay datos
	var existingCount int
	err := db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&existingCount)
	if err != nil {
		log.Fatalf("❌ Error verificando datos existentes: %v", err)
	}

	if existingCount > 0 {
		fmt.Printf("\n⚠️  Ya existen %d registros en la tabla.\n", existingCount)
		fmt.Print("¿Deseas continuar agregando más? (s/n): ")
		var response string
		fmt.Scanln(&response)
		if response != "s" && response != "S" {
			fmt.Println("Operación cancelada.")
			return
		}
	}

	totalRows := records
	batchSize := 5000

	fmt.Printf("\n📦 Iniciando carga de %d registros...\n", totalRows)
	fmt.Printf("   Batch size: %d\n", batchSize)
	fmt.Printf("   Patrón: Tráfico realista con rush hours\n\n")

	startTime := time.Now()

	// Preparar statement para inserción (omitimos `id` para que SERIAL lo genere)
	stmt, err := db.Prepare(`
		INSERT INTO orders (branch_id, customer_id, total, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		log.Fatalf("❌ Error preparando statement: %v", err)
	}
	defer stmt.Close()

	for i := 0; i < totalRows; i += batchSize {
		batchStart := time.Now()
		currentBatchSize := batchSize
		if i+batchSize > totalRows {
			currentBatchSize = totalRows - i
		}

		orders := generateBatch(currentBatchSize)

		// Iniciar transacción
		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("❌ Error iniciando transacción: %v", err)
		}

		// Insertar batch
		for _, order := range orders {
			_, err = tx.Stmt(stmt).Exec(
				order.BranchID,
				order.CustomerID,
				order.Total,
				order.Status,
				order.CreatedAt,
				order.UpdatedAt,
			)
			if err != nil {
				tx.Rollback()
				log.Fatalf("❌ Error insertando orden: %v", err)
			}
		}

		// Commit transacción
		err = tx.Commit()
		if err != nil {
			log.Fatalf("❌ Error en commit: %v", err)
		}

		batchDuration := time.Since(batchStart)
		progress := float64(i+currentBatchSize) / float64(totalRows) * 100
		rate := float64(currentBatchSize) / batchDuration.Seconds()

		fmt.Printf("   [%5.1f%%] Insertados %7d / %d - %.0f rows/sec - Batch: %.2fs\n",
			progress, i+currentBatchSize, totalRows, rate, batchDuration.Seconds())
	}

	duration := time.Since(startTime)
	overallRate := float64(totalRows) / duration.Seconds()

	fmt.Printf("\n✅ Carga completada exitosamente!\n")
	fmt.Printf("   Tiempo total: %.2f segundos (%.2f minutos)\n", duration.Seconds(), duration.Minutes())
	fmt.Printf("   Velocidad promedio: %.0f rows/sec\n", overallRate)

	goalMet := duration < 10*time.Minute
	goalStatus := "✅ CUMPLIDO"
	if !goalMet {
		goalStatus = "❌ NO CUMPLIDO"
	}
	fmt.Printf("   Objetivo: <10 minutos - %s\n", goalStatus)

	// Crear índices
	err = createIndexes(db)
	if err != nil {
		log.Printf("⚠️ Error creando índices: %v", err)
	}

	// Verificar integridad
	err = verifyDataIntegrity(db)
	if err != nil {
		log.Printf("⚠️ Error verificando integridad: %v", err)
	}

	// Actualizar estadísticas
	fmt.Println("\n🔧 Actualizando estadísticas de la base de datos...")
	_, err = db.Exec("ANALYZE orders")
	if err != nil {
		log.Printf("⚠️ Error actualizando estadísticas: %v", err)
	}
	fmt.Println("✅ Estadísticas actualizadas")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  PROCESO COMPLETADO - Ready para testing")
	fmt.Println(strings.Repeat("=", 60) + "\n")
}
