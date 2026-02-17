help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

up: ## Levantar todos los servicios
	docker-compose up -d --build
	@echo "✅ Servicios iniciados. Esperando que estén listos..."
	@sleep 10
	@docker-compose ps

down: ## Detener todos los servicios
	docker-compose down

build: ## Construir imágenes sin cache
	docker-compose build --no-cache

logs: ## Ver logs de todos los servicios
	docker-compose logs -f

logs-api: ## Ver logs solo de la API
	docker-compose logs -f api

test: ## Ejecutar tests básicos
	@echo "🧪 Ejecutando smoke tests..."
	@curl -s http://localhost:8088/health || echo "❌ API no responde"
	@curl -s http://localhost:8088/ready || echo "❌ Readiness check falló"
	@curl -s http://localhost:9090/-/healthy || echo "❌ Prometheus no responde"

load-data: ## Cargar 500k órdenes (toma 12 segundos)
	@echo "📦 Iniciando carga de datos..."
	docker-compose exec api /app/scripts/loader records=500000, cat=0, p=0

load-data-small: ## Cargar solo 10k órdenes (para testing rápido)
	@echo "📦 Carga rápida de 10k registros..."
	docker-compose exec api /app/scripts/loader records=10000, cat=0, p=0

db-shell: ## Abrir shell de PostgreSQL
	docker-compose exec postgres psql -U postgres -d tmh_db

redis-shell: ## Abrir shell de Redis
	docker-compose exec redis redis-cli

db-stats: ## Ver estadísticas de la DB
	docker-compose exec postgres psql -U postgres -d tmh_db -c "SELECT COUNT(*) as categories FROM categories;"
	docker-compose exec postgres psql -U postgres -d tmh_db -c "SELECT COUNT(*) as products FROM products;"
	docker-compose exec postgres psql -U postgres -d tmh_db -c "SELECT COUNT(*) as total_orders FROM orders;"
	docker-compose exec postgres psql -U postgres -d tmh_db -c "SELECT status, COUNT(*) FROM orders GROUP BY status;"

clean: ## Limpiar datos (mantiene volúmenes)
	docker-compose down

reset: ## Reset completo (elimina volúmenes)
	docker-compose down -v
	@echo "⚠️  Todos los datos han sido eliminados"

restart-api: ## Reiniciar solo la API
	docker-compose restart api

k6-test: ## Ejecutar load test con K6
	k6 run test/k6.js

metrics: ## Ver métricas en tiempo real
	@echo "📊 Métricas disponibles en:"
	@echo "   Prometheus: http://localhost:9090"
	@echo "   Grafana:    http://localhost:3000 (admin/admin)"
	@echo "   API Metrics: http://localhost:8088/metrics"

status: ## Ver estado de servicios
	@docker-compose ps
	@echo ""
	@echo "🔍 Health Checks:"
	@curl -s http://localhost:8088/health && echo "✅ API Health OK" || echo "❌ API Health FAIL"
	@curl -s http://localhost:8088/ready && echo "✅ API Ready OK" || echo "❌ API Ready FAIL"
    