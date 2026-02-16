import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

// Métricas personalizadas
const errorRate = new Rate('errors');
const getOrderDuration = new Trend('get_order_duration');
const createOrderDuration = new Trend('create_order_duration');
const ordersCreated = new Counter('orders_created');

// Configuración de prueba
export const options = {
  stages: [
    { duration: '30s', target: 100 },  // Ramp up: 0 → 100 usuarios
    { duration: '2m', target: 300 },   // Load normal: 300 usuarios (≈150 req/seg)
    { duration: '1m', target: 500 },   // Peak load: 500 usuarios (≈300 req/seg)
    { duration: '2m', target: 500 },   // Sostenido en pico
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    // REQUISITOS DEL CHALLENGE
    'http_req_duration{endpoint:get_order}': ['p(95)<50'],        // GET /:id < 50ms
    'http_req_duration{endpoint:create_order}': ['p(95)<150'],    // POST < 150ms
    'http_req_duration{endpoint:list_orders}': ['p(95)<200'],     // GET / < 200ms
    'http_req_failed': ['rate<0.01'],                             // Error rate < 1%
    'errors': ['rate<0.01'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8088';

export default function () {
  // Simular tráfico real: 60% lecturas, 30% creación, 10% updates
  const scenario = Math.random();

  if (scenario < 0.6) {
    // GET /api/v1/orders/:id (lectura individual)
    const orderId = Math.floor(Math.random() * 500000) + 1;
    const res = http.get(`${BASE_URL}/api/v1/orders/${orderId}`, {
      tags: { endpoint: 'get_order' },
    });
    
    const success = check(res, {
      'GET order: status 200': (r) => r.status === 200,
      'GET order: has id': (r) => JSON.parse(r.body).id !== undefined,
      'GET order: latency OK': (r) => r.timings.duration < 100,
    });
    
    if (!success) errorRate.add(1);
    getOrderDuration.add(res.timings.duration);

  } else if (scenario < 0.9) {
    // POST /api/v1/orders (crear orden)
    const payload = JSON.stringify({
      branch_id: Math.floor(Math.random() * 100) + 1,
      customer_id: Math.floor(Math.random() * 10000) + 1000,
      total: parseFloat((Math.random() * 500 + 50).toFixed(2)),
    });

    const res = http.post(`${BASE_URL}/api/v1/orders`, payload, {
      headers: { 'Content-Type': 'application/json' },
      tags: { endpoint: 'create_order' },
    });

    const success = check(res, {
      'POST order: status 201': (r) => r.status === 201,
      'POST order: returns id': (r) => JSON.parse(r.body).id !== undefined,
    });

    if (success) {
      ordersCreated.add(1);
    } else {
      errorRate.add(1);
    }
    createOrderDuration.add(res.timings.duration);

  } else {
    // GET /api/v1/orders (listar con paginación)
    const page = Math.floor(Math.random() * 100) + 1;
    const res = http.get(`${BASE_URL}/api/v1/orders?page=${page}`, {
      tags: { endpoint: 'list_orders' },
    });

    check(res, {
      'GET list: status 200': (r) => r.status === 200,
      'GET list: returns array': (r) => Array.isArray(JSON.parse(r.body)),
    });
  }

  // Simular think time de usuario (0.5-2 segundos)
  sleep(Math.random() * 1.5 + 0.5);
}

export function handleSummary(data) {
  console.log('');
  console.log('========================================');
  console.log('  RESULTADOS DE LOAD TEST - TMH Challenge');
  console.log('========================================');
  console.log('');
  console.log(`✅ Requests totales: ${data.metrics.http_reqs.values.count}`);
  console.log(`✅ Requests/seg (avg): ${data.metrics.http_reqs.values.rate.toFixed(2)}`);
  console.log(`✅ Error rate: ${(data.metrics.http_req_failed.values.rate * 100).toFixed(2)}%`);
  console.log('');
  console.log('Latencias (ms):');
  console.log(`  GET /orders/:id (P95): ${data.metrics['http_req_duration{endpoint:get_order}'].values['p(95)'].toFixed(2)}ms`);
  console.log(`  POST /orders (P95): ${data.metrics['http_req_duration{endpoint:create_order}'].values['p(95)'].toFixed(2)}ms`);
  console.log(`  GET /orders (P95): ${data.metrics['http_req_duration{endpoint:list_orders}'].values['p(95)'].toFixed(2)}ms`);
  console.log('');
  
  return {
    'summary.txt': textSummary(data, { indent: ' ', enableColors: false }),
  };
}