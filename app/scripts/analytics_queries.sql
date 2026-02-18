-- Top 10 productos por sucursal en el último mes
EXPLAIN ANALYZE
SELECT 
    o.branch_id, 
    p.name, SUM(oi.quantity * oi.price) AS total_sales 
FROM orders o 
JOIN order_items oi ON o.id = oi.order_id 
JOIN products p ON p.id = oi.product_id 
WHERE o.created_at >= NOW() - INTERVAL '1 month' 
GROUP BY o.branch_id, p.name 
ORDER BY total_sales DESC LIMIT 10;

-- Análisis de tendencias por hora del día
EXPLAIN ANALYZE
SELECT EXTRACT(HOUR FROM created_at) as hour_of_day,  COUNT(*) as order_count, AVG(total) as avg_order_value FROM orders WHERE created_at >= NOW() - INTERVAL '7 days' GROUP BY EXTRACT(HOUR FROM created_at) ORDER BY hour_of_day;

-- Reporte de ingresos diarios por categoría
EXPLAIN ANALYZE
SELECT DATE(o.created_at) as date, c.name as category, SUM(o.total) as daily_revenue FROM orders o JOIN order_items oi ON o.id = oi.order_id JOIN products p ON oi.product_id = p.id JOIN categories c ON p.category_id = c.id WHERE o.created_at >= NOW() - INTERVAL '30 days' GROUP BY DATE(o.created_at), c.name ORDER BY date DESC, daily_revenue DESC;