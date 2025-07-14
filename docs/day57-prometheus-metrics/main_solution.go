// Day 57: Prometheus Custom Metrics
// HTTPリクエスト数などのカスタムメトリクスを実装・公開

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// 模擬Prometheusメトリクス構造体
type CounterVec struct {
	metrics map[string]float64
	mu      sync.RWMutex
}

type HistogramVec struct {
	metrics map[string][]float64
	buckets []float64
	mu      sync.RWMutex
}

type Gauge struct {
	value float64
	mu    sync.RWMutex
}

type GaugeVec struct {
	metrics map[string]float64
	mu      sync.RWMutex
}

// CounterVec実装
func NewCounterVec(name, help string, labels []string) *CounterVec {
	return &CounterVec{
		metrics: make(map[string]float64),
	}
}

func (c *CounterVec) WithLabelValues(values ...string) *Counter {
	key := joinLabels(values)
	return &Counter{vec: c, key: key}
}

func (c *CounterVec) Inc(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics[key]++
}

func (c *CounterVec) Add(key string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics[key] += value
}

func (c *CounterVec) GetMetrics() map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]float64)
	for k, v := range c.metrics {
		result[k] = v
	}
	return result
}

type Counter struct {
	vec *CounterVec
	key string
}

func (c *Counter) Inc() {
	c.vec.Inc(c.key)
}

func (c *Counter) Add(value float64) {
	c.vec.Add(c.key, value)
}

// HistogramVec実装
func NewHistogramVec(name, help string, buckets []float64, labels []string) *HistogramVec {
	if buckets == nil {
		buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	}
	return &HistogramVec{
		metrics: make(map[string][]float64),
		buckets: buckets,
	}
}

func (h *HistogramVec) WithLabelValues(values ...string) *Histogram {
	key := joinLabels(values)
	return &Histogram{vec: h, key: key}
}

func (h *HistogramVec) Observe(key string, value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.metrics[key] == nil {
		h.metrics[key] = make([]float64, 0)
	}
	h.metrics[key] = append(h.metrics[key], value)
}

func (h *HistogramVec) GetMetrics() map[string]map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	result := make(map[string]map[string]interface{})
	for key, values := range h.metrics {
		if len(values) == 0 {
			continue
		}
		
		// 統計計算
		sum := 0.0
		count := len(values)
		for _, v := range values {
			sum += v
		}
		
		// バケット計算
		bucketCounts := make(map[string]int)
		for _, bucket := range h.buckets {
			bucketKey := fmt.Sprintf("le_%.3f", bucket)
			bucketCounts[bucketKey] = 0
			for _, v := range values {
				if v <= bucket {
					bucketCounts[bucketKey]++
				}
			}
		}
		
		result[key] = map[string]interface{}{
			"sum":     sum,
			"count":   count,
			"buckets": bucketCounts,
		}
	}
	
	return result
}

type Histogram struct {
	vec *HistogramVec
	key string
}

func (h *Histogram) Observe(value float64) {
	h.vec.Observe(h.key, value)
}

// Gauge実装
func NewGauge(name, help string) *Gauge {
	return &Gauge{}
}

func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = value
}

func (g *Gauge) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

func (g *Gauge) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
}

func (g *Gauge) Add(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value += value
}

func (g *Gauge) Get() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.value
}

// GaugeVec実装
func NewGaugeVec(name, help string, labels []string) *GaugeVec {
	return &GaugeVec{
		metrics: make(map[string]float64),
	}
}

func (g *GaugeVec) WithLabelValues(values ...string) *GaugeMetric {
	key := joinLabels(values)
	return &GaugeMetric{vec: g, key: key}
}

func (g *GaugeVec) Set(key string, value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.metrics[key] = value
}

func (g *GaugeVec) GetMetrics() map[string]float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make(map[string]float64)
	for k, v := range g.metrics {
		result[k] = v
	}
	return result
}

type GaugeMetric struct {
	vec *GaugeVec
	key string
}

func (g *GaugeMetric) Set(value float64) {
	g.vec.Set(g.key, value)
}

func (g *GaugeMetric) Inc() {
	g.vec.Set(g.key, g.vec.metrics[g.key]+1)
}

func (g *GaugeMetric) Dec() {
	g.vec.Set(g.key, g.vec.metrics[g.key]-1)
}

// メトリクス定義
var (
	// HTTPメトリクス
	httpRequestsTotal = NewCounterVec(
		"http_requests_total",
		"Total number of HTTP requests",
		[]string{"method", "endpoint", "status"},
	)
	
	httpRequestDuration = NewHistogramVec(
		"http_request_duration_seconds",
		"HTTP request duration in seconds",
		nil,
		[]string{"method", "endpoint"},
	)
	
	// ビジネスメトリクス
	activeUsers = NewGauge(
		"active_users_total",
		"Number of currently active users",
	)
	
	ordersTotal = NewCounterVec(
		"orders_total",
		"Total number of orders",
		[]string{"status", "product_category"},
	)
	
	revenueTotal = NewCounterVec(
		"revenue_total",
		"Total revenue amount",
		[]string{"currency", "product_category"},
	)
	
	// システムメトリクス
	cpuUsage = NewGauge(
		"cpu_usage_percent",
		"CPU usage percentage",
	)
	
	memoryUsage = NewGaugeVec(
		"memory_usage_bytes",
		"Memory usage in bytes",
		[]string{"type"},
	)
	
	diskUsage = NewGaugeVec(
		"disk_usage_percent",
		"Disk usage percentage",
		[]string{"device", "mount_point"},
	)
	
	// アプリケーションメトリクス
	databaseConnections = NewGaugeVec(
		"database_connections",
		"Number of database connections",
		[]string{"database", "state"},
	)
	
	cacheHitRatio = NewGauge(
		"cache_hit_ratio",
		"Cache hit ratio",
	)
	
	apiResponseSize = NewHistogramVec(
		"api_response_size_bytes",
		"API response size in bytes",
		[]float64{100, 1000, 10000, 100000, 1000000},
		[]string{"endpoint"},
	)
)

// メトリクス収集器
type MetricsCollector struct {
	registry map[string]interface{}
	mu       sync.RWMutex
}

func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		registry: make(map[string]interface{}),
	}
	
	// メトリクスを登録
	mc.Register("http_requests_total", httpRequestsTotal)
	mc.Register("http_request_duration_seconds", httpRequestDuration)
	mc.Register("active_users_total", activeUsers)
	mc.Register("orders_total", ordersTotal)
	mc.Register("revenue_total", revenueTotal)
	mc.Register("cpu_usage_percent", cpuUsage)
	mc.Register("memory_usage_bytes", memoryUsage)
	mc.Register("disk_usage_percent", diskUsage)
	mc.Register("database_connections", databaseConnections)
	mc.Register("cache_hit_ratio", cacheHitRatio)
	mc.Register("api_response_size_bytes", apiResponseSize)
	
	return mc
}

func (mc *MetricsCollector) Register(name string, metric interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.registry[name] = metric
}

func (mc *MetricsCollector) Gather() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	result := make(map[string]interface{})
	
	for name, metric := range mc.registry {
		switch m := metric.(type) {
		case *CounterVec:
			result[name] = m.GetMetrics()
		case *HistogramVec:
			result[name] = m.GetMetrics()
		case *Gauge:
			result[name] = m.Get()
		case *GaugeVec:
			result[name] = m.GetMetrics()
		}
	}
	
	return result
}

// HTTPミドルウェア
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// レスポンスライターをラップ
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// 次のハンドラを実行
		next.ServeHTTP(ww, r)
		
		// メトリクスを記録
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.statusCode)
		
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// ビジネスメトリクス記録器
type BusinessMetrics struct {
	collector *MetricsCollector
}

func NewBusinessMetrics(collector *MetricsCollector) *BusinessMetrics {
	return &BusinessMetrics{collector: collector}
}

func (bm *BusinessMetrics) RecordOrder(status, category string, amount float64) {
	ordersTotal.WithLabelValues(status, category).Inc()
	revenueTotal.WithLabelValues("USD", category).Add(amount)
}

func (bm *BusinessMetrics) UpdateActiveUsers(count int) {
	activeUsers.Set(float64(count))
}

func (bm *BusinessMetrics) RecordCacheHit(hit bool, total int) {
	// 簡単なヒット率計算
	if total > 0 {
		hitCount := 0
		if hit {
			hitCount = 1
		}
		ratio := float64(hitCount) / float64(total)
		cacheHitRatio.Set(ratio)
	}
}

// システムメトリクス収集器
type SystemMetricsCollector struct {
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewSystemMetricsCollector() *SystemMetricsCollector {
	return &SystemMetricsCollector{
		stopCh: make(chan struct{}),
	}
}

func (smc *SystemMetricsCollector) Start(interval time.Duration) {
	smc.wg.Add(1)
	go smc.collectLoop(interval)
}

func (smc *SystemMetricsCollector) Stop() {
	close(smc.stopCh)
	smc.wg.Wait()
}

func (smc *SystemMetricsCollector) collectLoop(interval time.Duration) {
	defer smc.wg.Done()
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			smc.collectSystemMetrics()
		case <-smc.stopCh:
			return
		}
	}
}

func (smc *SystemMetricsCollector) collectSystemMetrics() {
	// CPU使用率（模擬）
	cpuPercent := rand.Float64() * 100
	cpuUsage.Set(cpuPercent)
	
	// メモリ使用量
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	memoryUsage.WithLabelValues("heap").Set(float64(m.HeapInuse))
	memoryUsage.WithLabelValues("stack").Set(float64(m.StackInuse))
	memoryUsage.WithLabelValues("sys").Set(float64(m.Sys))
	
	// ディスク使用率（模擬）
	diskUsage.WithLabelValues("/dev/sda1", "/").Set(rand.Float64() * 100)
	diskUsage.WithLabelValues("/dev/sda2", "/home").Set(rand.Float64() * 100)
	
	// データベース接続数（模擬）
	databaseConnections.WithLabelValues("postgres", "active").Set(float64(rand.Intn(50)))
	databaseConnections.WithLabelValues("postgres", "idle").Set(float64(rand.Intn(20)))
	databaseConnections.WithLabelValues("redis", "active").Set(float64(rand.Intn(10)))
}

// APIサービス
type APIService struct {
	businessMetrics *BusinessMetrics
	userCount       int
	mu              sync.RWMutex
}

func NewAPIService(businessMetrics *BusinessMetrics) *APIService {
	return &APIService{
		businessMetrics: businessMetrics,
		userCount:       100, // 初期値
	}
}

func (api *APIService) UsersHandler(w http.ResponseWriter, r *http.Request) {
	api.mu.RLock()
	count := api.userCount
	api.mu.RUnlock()
	
	// アクティブユーザー数を更新
	api.businessMetrics.UpdateActiveUsers(count)
	
	response := map[string]interface{}{
		"active_users": count,
		"timestamp":    time.Now().Unix(),
	}
	
	data, _ := json.Marshal(response)
	apiResponseSize.WithLabelValues("/users").Observe(float64(len(data)))
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (api *APIService) OrdersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 注文を記録
	categories := []string{"electronics", "books", "clothing"}
	category := categories[rand.Intn(len(categories))]
	amount := rand.Float64() * 1000
	
	api.businessMetrics.RecordOrder("completed", category, amount)
	
	response := map[string]interface{}{
		"order_id": fmt.Sprintf("order_%d", time.Now().UnixNano()),
		"amount":   amount,
		"category": category,
		"status":   "completed",
	}
	
	data, _ := json.Marshal(response)
	apiResponseSize.WithLabelValues("/orders").Observe(float64(len(data)))
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (api *APIService) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	collector := NewMetricsCollector()
	metrics := collector.Gather()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (api *APIService) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","timestamp":` + strconv.FormatInt(time.Now().Unix(), 10) + `}`))
}

// サーバー設定
func SetupServer(businessMetrics *BusinessMetrics) *http.Server {
	apiService := NewAPIService(businessMetrics)
	
	mux := http.NewServeMux()
	mux.HandleFunc("/users", apiService.UsersHandler)
	mux.HandleFunc("/orders", apiService.OrdersHandler)
	mux.HandleFunc("/metrics", apiService.MetricsHandler)
	mux.HandleFunc("/health", apiService.HealthHandler)
	
	// Prometheusミドルウェアを適用
	handler := PrometheusMiddleware(mux)
	
	return &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
}

// ユーティリティ関数
func joinLabels(values []string) string {
	return fmt.Sprintf("%v", values)
}

func main() {
	fmt.Println("Day 57: Prometheus Custom Metrics")
	
	// メトリクス収集器を初期化
	collector := NewMetricsCollector()
	businessMetrics := NewBusinessMetrics(collector)
	
	// システムメトリクス収集を開始
	systemCollector := NewSystemMetricsCollector()
	systemCollector.Start(10 * time.Second)
	defer systemCollector.Stop()
	
	// サーバーを設定
	server := SetupServer(businessMetrics)
	
	log.Printf("Starting server on %s", server.Addr)
	log.Printf("Metrics available at http://localhost%s/metrics", server.Addr)
	log.Fatal(server.ListenAndServe())
}