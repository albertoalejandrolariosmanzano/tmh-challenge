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

-- Verificar
SELECT 'Database initialized successfully!' as status;