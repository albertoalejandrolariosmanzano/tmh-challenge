-- Tabla principal de órdenes con particionamiento por fecha
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    branch_id INTEGER NOT NULL,
    customer_id INTEGER NOT NULL,
    total DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- Particiones mensuales (últimos 6 meses)
CREATE TABLE orders_2025_09 PARTITION OF orders
    FOR VALUES FROM ('2025-09-01') TO ('2025-10-01');

CREATE TABLE orders_2025_10 PARTITION OF orders
    FOR VALUES FROM ('2025-10-01') TO ('2025-11-01');

CREATE TABLE orders_2025_11 PARTITION OF orders
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');

CREATE TABLE orders_2025_12 PARTITION OF orders
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');

CREATE TABLE orders_2026_01 PARTITION OF orders
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE orders_2026_02 PARTITION OF orders
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

-- Índices para mejorar performance
CREATE INDEX idx_orders_branch_created ON orders(branch_id, created_at DESC);
CREATE INDEX idx_orders_status ON orders(status) WHERE status != 'COMPLETED';
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL,
    product_id INTEGER NOT NULL,
    quantity INT NOT NULL,
    price NUMERIC(10,2) NOT NULL
);

CREATE INDEX idx_order_items_product ON order_items (product_id);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    category_id INTEGER NOT NULL,
    price NUMERIC(10,2) NOT NULL,
    cost NUMERIC(10,2),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Para joins frecuentes con order_items
CREATE INDEX idx_order_items_order_id ON order_items(order_id);

-- Para top productos
CREATE INDEX idx_order_items_product_order 
ON order_items(product_id, order_id);

-- Para reportes por categoría
CREATE INDEX idx_products_category 
ON products(category_id);

-- Si haces búsquedas por producto activo
CREATE INDEX idx_products_active 
ON products(is_active) WHERE is_active = TRUE;

CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Vista materializada para analytics
CREATE MATERIALIZED VIEW product_sales_monthly AS
SELECT 
    date_trunc('month', o.created_at) AS month,
    o.branch_id,
    oi.product_id,
    SUM(oi.quantity * oi.price) AS total_sales
FROM orders o
JOIN order_items oi ON o.id = oi.order_id
GROUP BY month, o.branch_id, oi.product_id;

CREATE INDEX idx_product_sales_monthly ON product_sales_monthly(month, branch_id);

-- Función para refresh automático
CREATE OR REPLACE FUNCTION refresh_stats()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY product_sales_monthly;
END;
$$ LANGUAGE plpgsql;