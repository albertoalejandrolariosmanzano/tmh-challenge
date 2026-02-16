-- Tabla principal de órdenes
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    branch_id INTEGER NOT NULL,
    customer_id INTEGER NOT NULL,
    total DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Índices básicos (más se crean en load_500k.go)
CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(created_at DESC);

-- Verificar
SELECT 'Database initialized successfully!' as status;