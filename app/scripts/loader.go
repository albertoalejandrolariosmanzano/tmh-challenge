package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

var productPrices = map[int]float64{}
var productIDs []int

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
	12: 50, // Hora comida
	13: 60, // Hora pico
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

func main() {
	rand.Seed(time.Now().UnixNano() + rand.Int63())

	// ====== ARGUMENTOS ======
	addCategories := 0
	addProducts := 0
	totalRecords := 500000

	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "cat=") {
			addCategories, _ = strconv.Atoi(strings.TrimPrefix(arg, "cat="))
		}
		if strings.HasPrefix(arg, "p=") {
			addProducts, _ = strconv.Atoi(strings.TrimPrefix(arg, "p="))
		}
		if strings.HasPrefix(arg, "records=") {
			totalRecords, _ = strconv.Atoi(strings.TrimPrefix(arg, "records="))
		}
	}

	fmt.Println("📊 Configuración:")
	fmt.Println("   Categorías extra:", addCategories)
	fmt.Println("   Productos extra:", addProducts)
	fmt.Println("   Órdenes a crear:", totalRecords)

	db := connectDB()
	defer db.Close()

	err := ensureSeedData(db, addCategories, addProducts)
	if err != nil {
		log.Fatal(err)
	}

	loadProducts(db)

	fmt.Printf("\n📦 Iniciando carga de %d registros...\n", totalRecords)
	fmt.Printf("   Patrón: Tráfico realista con rush hours\n\n")

	start := time.Now()

	err = bulkInsertWithCopy(db, totalRecords)
	if err != nil {
		log.Fatal(err)
	}

	elapsed := time.Since(start)
	overallRate := float64(totalRecords) / elapsed.Seconds()

	fmt.Printf("\n✅ Carga completada exitosamente!\n")
	fmt.Printf("   Tiempo total: %.2f segundos (%.2f minutos)\n", elapsed.Seconds(), elapsed.Minutes())
	fmt.Printf("   Velocidad promedio: %.0f rows/sec\n", overallRate)

	goalMet := elapsed < 10*time.Minute
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

	fmt.Println("🔧 Actualizando estadísticas...")
	_, err = db.Exec("ANALYZE orders")
	_, err = db.Exec("ANALYZE order_items")
	if err != nil {
		log.Printf("⚠️ Error actualizando estadísticas: %v", err)
	}
	fmt.Println("✅ Estadísticas actualizadas")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  PROCESO COMPLETADO - Ready para testing")
	fmt.Println(strings.Repeat("=", 60) + "\n")
}

func connectDB() *sql.DB {

	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "password")
	dbname := getEnv("DB_NAME", "tmh_db")

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(0) // 🔥 IMPORTANTE
	db.SetConnMaxIdleTime(0)

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func ensureSeedData(db *sql.DB, addCategories, addProducts int) error {

	var totalCategories int
	var categoryCount int
	db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&categoryCount)

	// Si no hay categorías, crear 10 categorías base y asi addCategories se suma a estas 10,
	// si addCategories es 0, solo se crean estas 10 categorías base
	if categoryCount == 0 && addCategories == 0 {
		totalCategories = 10
		fmt.Println("📦 Creando 10 categorías base...")
	} else if categoryCount > 0 && addCategories > 0 {
		fmt.Printf("➕ Agregando %d categorías...\n", addCategories)
		totalCategories = addCategories
	} else if categoryCount == 0 && addCategories > 0 {
		fmt.Printf("📦 Creando 10 categorías base, ➕ %d nuevas categorías\n", addCategories)
		totalCategories = addCategories + 10
	} else if categoryCount > 0 && addCategories == 0 {
		totalCategories = categoryCount
	}

	if totalCategories > 0 && categoryCount != totalCategories {
		for i := 1; i <= totalCategories; i++ {
			db.Exec(`
				INSERT INTO categories (name)
				VALUES ($1)
			`, fmt.Sprintf("Category %d", i))
		}
		fmt.Printf("✅ Creadas %d categorías.\n", totalCategories)
	}

	var productCount int
	var totalProducts int
	db.QueryRow("SELECT COUNT(*) FROM products").Scan(&productCount)

	if productCount == 0 && addProducts == 0 {
		totalProducts = 200
		fmt.Println("📦 Creando 200 productos base...")
	} else if productCount > 0 && addProducts > 0 {
		fmt.Printf("➕ Agregando %d productos...\n", addProducts)
		totalProducts = addProducts
	} else if productCount == 0 && addProducts > 0 {
		fmt.Printf("📦 Creando 200 productos base, ➕ %d nuevos productos\n", addProducts)
		totalProducts = addProducts + 200
	} else if productCount > 0 && addProducts == 0 {
		totalProducts = productCount
	}

	if totalProducts > 0 && productCount != totalProducts {
		for i := 1; i <= totalProducts; i++ {
			db.Exec(`
				INSERT INTO products (name, category_id, price)
				VALUES ($1, $2, $3)
			`, fmt.Sprintf("Product %d", i), rand.Intn(totalCategories)+1, float64(rand.Intn(5000)+500)/100)
		}
		fmt.Printf("✅ Creados %d productos.\n", totalProducts)
	}
	return nil
}

func loadProducts(db *sql.DB) {
	rows, _ := db.Query("SELECT id, price FROM products")
	defer rows.Close()

	for rows.Next() {
		var id int
		var price float64
		rows.Scan(&id, &price)
		productPrices[id] = price
		productIDs = append(productIDs, id)
	}
}

func generateRealisticTimestamp(daysBack int) time.Time {
	// Fecha aleatoria
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

func bulkInsertWithCopy(db *sql.DB, total int) error {

	if len(productIDs) == 0 {
		return fmt.Errorf("no hay productos cargados en memoria")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Obtener último ID
	var lastID int64
	err = tx.QueryRow(`SELECT COALESCE(MAX(id),0) FROM orders`).Scan(&lastID)
	if err != nil {
		tx.Rollback()
		return err
	}

	currentID := lastID

	// ===============================
	// 1️⃣ COPY ORDERS
	// ===============================

	orderStmt, err := tx.Prepare(pq.CopyIn(
		"orders",
		"id",
		"branch_id", "customer_id", "total",
		"status", "created_at", "updated_at",
	))
	if err != nil {
		tx.Rollback()
		return err
	}

	type orderItem struct {
		orderID   int64
		productID int
		quantity  int
		price     float64
	}

	var itemsBuffer []orderItem

	for i := 0; i < total; i++ {

		currentID++

		branchID := rand.Intn(10) + 1
		customerID := rand.Intn(10000) + 1
		status := weightedRandomChoiceString(statuses, statusWeights)
		created := generateRealisticTimestamp(180)

		itemsCount := rand.Intn(5) + 1
		var totalAmount float64

		for j := 0; j < itemsCount; j++ {
			productID := productIDs[rand.Intn(len(productIDs))]
			quantity := rand.Intn(3) + 1
			price := productPrices[productID]

			totalAmount += price * float64(quantity)

			itemsBuffer = append(itemsBuffer, orderItem{
				orderID:   currentID,
				productID: productID,
				quantity:  quantity,
				price:     price,
			})
		}

		_, err = orderStmt.Exec(
			currentID,
			branchID,
			customerID,
			totalAmount,
			status,
			created,
			created,
		)
		if err != nil {
			orderStmt.Close()
			tx.Rollback()
			return err
		}
	}

	_, err = orderStmt.Exec()
	if err != nil {
		orderStmt.Close()
		tx.Rollback()
		return err
	}

	orderStmt.Close()

	// ===============================
	// 2️⃣ COPY ORDER_ITEMS
	// ===============================

	itemStmt, err := tx.Prepare(pq.CopyIn(
		"order_items",
		"order_id", "product_id", "quantity", "price",
	))
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, item := range itemsBuffer {
		_, err = itemStmt.Exec(
			item.orderID,
			item.productID,
			item.quantity,
			item.price,
		)
		if err != nil {
			itemStmt.Close()
			tx.Rollback()
			return err
		}
	}

	_, err = itemStmt.Exec()
	if err != nil {
		itemStmt.Close()
		tx.Rollback()
		return err
	}

	itemStmt.Close()

	err = tx.Commit()
	if err != nil {
		return err
	}

	// Ajustar secuencia
	_, err = db.Exec(`
		SELECT setval(pg_get_serial_sequence('orders','id'),
		(SELECT MAX(id) FROM orders))
	`)

	return err
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
			return fmt.Errorf("Error creando índice: %v", err)
		}
	}

	fmt.Println("✅ Índices creados")
	return nil
}
