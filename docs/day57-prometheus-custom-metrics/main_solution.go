package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/common/model"
	dto "github.com/prometheus/client_model/go"
)

// ServiceMetrics - API リクエスト関連のメトリクス
type ServiceMetrics struct {
	requestsTotal     prometheus.Counter
	errorsTotal      prometheus.Counter
	httpRequestsTotal *prometheus.CounterVec
}

func NewServiceMetrics() *ServiceMetrics {
	sm := &ServiceMetrics{
		requestsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "myapp",
			Subsystem: "api",
			Name:     "requests_total",
			Help:     "Total number of requests processed",
		}),
		errorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "myapp",
			Subsystem: "api",
			Name:     "errors_total",
			Help:     "Total number of errors occurred",
		}),
		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "myapp",
				Subsystem: "http",
				Name:     "requests_total",
				Help:     "Total HTTP requests by method and status",
			},
			[]string{"method", "status_code", "endpoint"},
		),
	}
	
	prometheus.MustRegister(sm.requestsTotal)
	prometheus.MustRegister(sm.errorsTotal)
	prometheus.MustRegister(sm.httpRequestsTotal)
	
	return sm
}

func (sm *ServiceMetrics) RecordRequest(method, endpoint, statusCode string) {
	sm.requestsTotal.Inc()
	sm.httpRequestsTotal.WithLabelValues(method, statusCode, endpoint).Inc()
}

func (sm *ServiceMetrics) RecordError() {
	sm.errorsTotal.Inc()
}

// SystemMetrics - システムリソース関連のメトリクス
type SystemMetrics struct {
	currentConnections prometheus.Gauge
	memoryUsage       prometheus.Gauge
	goroutineCount    prometheus.Gauge
	queueSize         *prometheus.GaugeVec
}

func NewSystemMetrics() *SystemMetrics {
	sm := &SystemMetrics{
		currentConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "myapp",
			Subsystem: "system",
			Name:     "connections_current",
			Help:     "Current number of active connections",
		}),
		memoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "myapp",
			Subsystem: "system",
			Name:     "memory_usage_bytes",
			Help:     "Current memory usage in bytes",
		}),
		goroutineCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "myapp",
			Subsystem: "system",
			Name:     "goroutines_current",
			Help:     "Current number of goroutines",
		}),
		queueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "myapp",
				Subsystem: "queues",
				Name:     "size_current",
				Help:     "Current queue size by queue name",
			},
			[]string{"queue_name"},
		),
	}
	
	prometheus.MustRegister(sm.currentConnections)
	prometheus.MustRegister(sm.memoryUsage)
	prometheus.MustRegister(sm.goroutineCount)
	prometheus.MustRegister(sm.queueSize)
	
	return sm
}

func (sm *SystemMetrics) UpdateConnections(count float64) {
	sm.currentConnections.Set(count)
}

func (sm *SystemMetrics) UpdateMemoryUsage(bytes float64) {
	sm.memoryUsage.Set(bytes)
}

func (sm *SystemMetrics) UpdateGoroutineCount() {
	sm.goroutineCount.Set(float64(runtime.NumGoroutine()))
}

func (sm *SystemMetrics) UpdateQueueSize(queueName string, size float64) {
	sm.queueSize.WithLabelValues(queueName).Set(size)
}

// RequestMetrics - HTTP リクエスト詳細メトリクス
type RequestMetrics struct {
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
}

func NewRequestMetrics() *RequestMetrics {
	rm := &RequestMetrics{
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "myapp",
				Subsystem: "http",
				Name:     "request_duration_seconds",
				Help:     "HTTP request duration in seconds",
				Buckets:  []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"method", "endpoint", "status_code"},
		),
		requestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "myapp",
				Subsystem: "http",
				Name:     "request_size_bytes",
				Help:     "HTTP request size in bytes",
				Buckets:  prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "endpoint"},
		),
		responseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "myapp",
				Subsystem: "http",
				Name:     "response_size_bytes",
				Help:     "HTTP response size in bytes",
				Buckets:  prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "endpoint", "status_code"},
		),
	}
	
	prometheus.MustRegister(rm.requestDuration)
	prometheus.MustRegister(rm.requestSize)
	prometheus.MustRegister(rm.responseSize)
	
	return rm
}

func (rm *RequestMetrics) RecordRequestDuration(method, endpoint, statusCode string, duration time.Duration) {
	rm.requestDuration.WithLabelValues(method, endpoint, statusCode).Observe(duration.Seconds())
}

func (rm *RequestMetrics) RecordRequestSize(method, endpoint string, size float64) {
	rm.requestSize.WithLabelValues(method, endpoint).Observe(size)
}

func (rm *RequestMetrics) RecordResponseSize(method, endpoint, statusCode string, size float64) {
	rm.responseSize.WithLabelValues(method, endpoint, statusCode).Observe(size)
}

// ProcessingMetrics - タスク処理関連のメトリクス
type ProcessingMetrics struct {
	taskDuration   *prometheus.SummaryVec
	taskComplexity *prometheus.SummaryVec
}

func NewProcessingMetrics() *ProcessingMetrics {
	pm := &ProcessingMetrics{
		taskDuration: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace:  "myapp",
				Subsystem:  "tasks",
				Name:      "duration_seconds",
				Help:      "Task processing duration in seconds",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
				MaxAge:     time.Hour,
				AgeBuckets: 5,
				BufCap:     500,
			},
			[]string{"task_type", "worker_id"},
		),
		taskComplexity: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace:  "myapp",
				Subsystem:  "tasks",
				Name:      "complexity_score",
				Help:      "Task complexity score",
				Objectives: map[float64]float64{0.5: 0.05, 0.95: 0.01, 0.99: 0.001},
			},
			[]string{"task_type"},
		),
	}
	
	prometheus.MustRegister(pm.taskDuration)
	prometheus.MustRegister(pm.taskComplexity)
	
	return pm
}

func (pm *ProcessingMetrics) RecordTaskDuration(taskType, workerID string, duration time.Duration) {
	pm.taskDuration.WithLabelValues(taskType, workerID).Observe(duration.Seconds())
}

func (pm *ProcessingMetrics) RecordTaskComplexity(taskType string, complexity float64) {
	pm.taskComplexity.WithLabelValues(taskType).Observe(complexity)
}

// MetricsMiddleware - HTTPメトリクス収集ミドルウェア
type MetricsMiddleware struct {
	requestMetrics *RequestMetrics
	serviceMetrics *ServiceMetrics
}

func NewMetricsMiddleware(reqMetrics *RequestMetrics, svcMetrics *ServiceMetrics) *MetricsMiddleware {
	return &MetricsMiddleware{
		requestMetrics: reqMetrics,
		serviceMetrics: svcMetrics,
	}
}

func (mm *MetricsMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// リクエストサイズを記録
		if r.ContentLength > 0 {
			mm.requestMetrics.RecordRequestSize(r.Method, r.URL.Path, float64(r.ContentLength))
		}
		
		// レスポンスライターをラップ
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     200,
			responseSize:   0,
		}
		
		// 次のハンドラーを実行
		next.ServeHTTP(wrapped, r)
		
		// メトリクスを記録
		duration := time.Since(start)
		statusCode := strconv.Itoa(wrapped.statusCode)
		
		mm.serviceMetrics.RecordRequest(r.Method, r.URL.Path, statusCode)
		mm.requestMetrics.RecordRequestDuration(r.Method, r.URL.Path, statusCode, duration)
		mm.requestMetrics.RecordResponseSize(r.Method, r.URL.Path, statusCode, float64(wrapped.responseSize))
		
		if wrapped.statusCode >= 400 {
			mm.serviceMetrics.RecordError()
		}
	})
}

// responseWriter - レスポンス情報を記録するラッパー
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.responseSize += size
	return size, err
}

// CustomCollector - カスタムメトリクスコレクター
type CustomCollector struct {
	appInfo   *prometheus.Desc
	uptime    *prometheus.Desc
	version   string
	startTime time.Time
	buildInfo map[string]string
}

func NewCustomCollector(version string, buildInfo map[string]string) *CustomCollector {
	return &CustomCollector{
		appInfo: prometheus.NewDesc(
			"myapp_info",
			"Application information",
			[]string{"version", "commit", "build_date"},
			nil,
		),
		uptime: prometheus.NewDesc(
			"myapp_uptime_seconds",
			"Application uptime in seconds",
			nil,
			nil,
		),
		version:   version,
		startTime: time.Now(),
		buildInfo: buildInfo,
	}
}

func (cc *CustomCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cc.appInfo
	ch <- cc.uptime
}

func (cc *CustomCollector) Collect(ch chan<- prometheus.Metric) {
	// アプリケーション情報
	ch <- prometheus.MustNewConstMetric(
		cc.appInfo,
		prometheus.GaugeValue,
		1,
		cc.version,
		cc.buildInfo["commit"],
		cc.buildInfo["build_date"],
	)
	
	// アップタイム
	uptime := time.Since(cc.startTime).Seconds()
	ch <- prometheus.MustNewConstMetric(
		cc.uptime,
		prometheus.GaugeValue,
		uptime,
	)
}

// MetricsManager - メトリクス管理システム
type MetricsManager struct {
	registry     *prometheus.Registry
	pushGateway  *push.Pusher
	gatherer     prometheus.Gatherer
	collectors   []prometheus.Collector
	config       *MetricsConfig
	mu           sync.RWMutex
}

type MetricsConfig struct {
	Namespace       string
	EnablePush      bool
	PushInterval    time.Duration
	PushGatewayURL  string
	JobName         string
	InstanceID      string
	DefaultLabels   map[string]string
}

func NewMetricsManager(config *MetricsConfig) *MetricsManager {
	registry := prometheus.NewRegistry()
	
	mm := &MetricsManager{
		registry:   registry,
		gatherer:   registry,
		collectors: make([]prometheus.Collector, 0),
		config:     config,
	}
	
	if config.EnablePush {
		mm.setupPushGateway()
	}
	
	return mm
}

func (mm *MetricsManager) setupPushGateway() {
	mm.pushGateway = push.New(mm.config.PushGatewayURL, mm.config.JobName).
		Collector(mm.gatherer).
		Grouping("instance", mm.config.InstanceID)
	
	// デフォルトラベルを追加
	for key, value := range mm.config.DefaultLabels {
		mm.pushGateway = mm.pushGateway.Grouping(key, value)
	}
}

func (mm *MetricsManager) RegisterCollector(collector prometheus.Collector) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	if err := mm.registry.Register(collector); err != nil {
		return fmt.Errorf("failed to register collector: %w", err)
	}
	
	mm.collectors = append(mm.collectors, collector)
	return nil
}

func (mm *MetricsManager) StartPushMetrics(ctx context.Context) {
	if !mm.config.EnablePush || mm.pushGateway == nil {
		return
	}
	
	ticker := time.NewTicker(mm.config.PushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := mm.pushGateway.Push(); err != nil {
				log.Printf("Failed to push metrics: %v", err)
			}
		case <-ctx.Done():
			// 最後にメトリクスをプッシュ
			if err := mm.pushGateway.Push(); err != nil {
				log.Printf("Failed to push final metrics: %v", err)
			}
			return
		}
	}
}

func (mm *MetricsManager) GetHandler() http.Handler {
	return promhttp.HandlerFor(mm.gatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:         mm.registry,
	})
}

func (mm *MetricsManager) Gather() ([]*dto.MetricFamily, error) {
	return mm.gatherer.Gather()
}

// AlertRule - アラートルール
type AlertRule struct {
	Name       string
	Query      string
	Threshold  float64
	Comparator string // ">", "<", ">=", "<=", "==", "!="
	Duration   time.Duration
	Severity   string
}

// Alert - アラート情報
type Alert struct {
	Rule      AlertRule
	Value     float64
	Timestamp time.Time
	Firing    bool
}

// AlertNotifier - アラート通知インターフェース
type AlertNotifier interface {
	SendAlert(alert Alert) error
}

// AlertManager - アラート管理システム
type AlertManager struct {
	rules         []AlertRule
	alertHistory  []Alert
	notifier      AlertNotifier
	gatherer      prometheus.Gatherer
	mu            sync.RWMutex
}

func NewAlertManager(gatherer prometheus.Gatherer, notifier AlertNotifier) *AlertManager {
	return &AlertManager{
		rules:        make([]AlertRule, 0),
		alertHistory: make([]Alert, 0),
		notifier:     notifier,
		gatherer:     gatherer,
	}
}

func (am *AlertManager) AddRule(rule AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = append(am.rules, rule)
}

func (am *AlertManager) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			am.evaluateRules()
		case <-ctx.Done():
			return
		}
	}
}

func (am *AlertManager) evaluateRules() {
	am.mu.RLock()
	rules := make([]AlertRule, len(am.rules))
	copy(rules, am.rules)
	am.mu.RUnlock()
	
	for _, rule := range rules {
		value, err := am.evaluateRule(rule)
		if err != nil {
			log.Printf("Failed to evaluate rule %s: %v", rule.Name, err)
			continue
		}
		
		if am.checkAlert(rule, value) {
			alert := Alert{
				Rule:      rule,
				Value:     value,
				Timestamp: time.Now(),
				Firing:    true,
			}
			
			if err := am.notifier.SendAlert(alert); err != nil {
				log.Printf("Failed to send alert: %v", err)
			}
			
			am.mu.Lock()
			am.alertHistory = append(am.alertHistory, alert)
			am.mu.Unlock()
		}
	}
}

func (am *AlertManager) evaluateRule(rule AlertRule) (float64, error) {
	metricFamilies, err := am.gatherer.Gather()
	if err != nil {
		return 0, err
	}
	
	// 簡単なクエリエンジン（メトリクス名での検索）
	for _, mf := range metricFamilies {
		if mf.GetName() == rule.Query {
			for _, metric := range mf.GetMetric() {
				if metric.GetCounter() != nil {
					return metric.GetCounter().GetValue(), nil
				}
				if metric.GetGauge() != nil {
					return metric.GetGauge().GetValue(), nil
				}
			}
		}
	}
	
	return 0, fmt.Errorf("metric not found: %s", rule.Query)
}

func (am *AlertManager) checkAlert(rule AlertRule, value float64) bool {
	switch rule.Comparator {
	case ">":
		return value > rule.Threshold
	case "<":
		return value < rule.Threshold
	case ">=":
		return value >= rule.Threshold
	case "<=":
		return value <= rule.Threshold
	case "==":
		return value == rule.Threshold
	case "!=":
		return value != rule.Threshold
	default:
		return false
	}
}

// SimpleNotifier - シンプルな通知サービス
type SimpleNotifier struct {
	alerts []Alert
	mu     sync.RWMutex
}

func NewSimpleNotifier() *SimpleNotifier {
	return &SimpleNotifier{
		alerts: make([]Alert, 0),
	}
}

func (sn *SimpleNotifier) SendAlert(alert Alert) error {
	sn.mu.Lock()
	defer sn.mu.Unlock()
	
	log.Printf("ALERT [%s]: %s = %.2f (threshold: %.2f)", 
		alert.Rule.Severity, alert.Rule.Name, alert.Value, alert.Rule.Threshold)
	
	sn.alerts = append(sn.alerts, alert)
	return nil
}

func (sn *SimpleNotifier) GetAlerts() []Alert {
	sn.mu.RLock()
	defer sn.mu.RUnlock()
	
	alerts := make([]Alert, len(sn.alerts))
	copy(alerts, sn.alerts)
	return alerts
}

func main() {
	// メトリクス設定
	config := &MetricsConfig{
		Namespace:      "myapp",
		EnablePush:     false,
		PushInterval:   30 * time.Second,
		JobName:        "myapp",
		InstanceID:     "instance-1",
		DefaultLabels:  map[string]string{"env": "dev"},
	}
	
	// メトリクス管理システムを作成
	metricsManager := NewMetricsManager(config)
	
	// 各種メトリクスを作成
	serviceMetrics := NewServiceMetrics()
	systemMetrics := NewSystemMetrics()
	requestMetrics := NewRequestMetrics()
	processingMetrics := NewProcessingMetrics()
	
	// カスタムコレクターを作成
	buildInfo := map[string]string{
		"commit":     "abc123",
		"build_date": "2024-01-01",
	}
	customCollector := NewCustomCollector("1.0.0", buildInfo)
	
	// コレクターを登録
	metricsManager.RegisterCollector(customCollector)
	
	// アラートシステムを設定
	notifier := NewSimpleNotifier()
	alertManager := NewAlertManager(metricsManager.gatherer, notifier)
	
	// アラートルールを追加
	alertManager.AddRule(AlertRule{
		Name:       "HighErrorRate",
		Query:      "myapp_api_errors_total",
		Threshold:  10,
		Comparator: ">",
		Duration:   5 * time.Minute,
		Severity:   "warning",
	})
	
	// ミドルウェアを作成
	metricsMiddleware := NewMetricsMiddleware(requestMetrics, serviceMetrics)
	
	// HTTP ハンドラーを設定
	mux := http.NewServeMux()
	
	// メトリクス エンドポイント
	mux.Handle("/metrics", metricsManager.GetHandler())
	
	// サンプル API エンドポイント
	mux.Handle("/api/hello", metricsMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// サンプル処理
		processingTime := time.Duration(10+rand.Intn(100)) * time.Millisecond
		time.Sleep(processingTime)
		
		// タスク処理メトリクスを記録
		processingMetrics.RecordTaskDuration("hello", "worker-1", processingTime)
		processingMetrics.RecordTaskComplexity("hello", float64(rand.Intn(100)))
		
		// エラーレスポンスをシミュレート（10%の確率）
		if rand.Intn(10) == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "Internal server error", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"message": "Hello, World!", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})))
	
	// エラーを発生させるエンドポイント（テスト用）
	mux.Handle("/api/error", metricsMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error": "Bad request", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})))
	
	// アラート一覧エンドポイント
	mux.HandleFunc("/alerts", func(w http.ResponseWriter, r *http.Request) {
		alerts := notifier.GetAlerts()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"alerts\": [")
		for i, alert := range alerts {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, "{\"rule\": \"%s\", \"value\": %.2f, \"threshold\": %.2f, \"timestamp\": \"%s\"}",
				alert.Rule.Name, alert.Value, alert.Rule.Threshold, alert.Timestamp.Format(time.RFC3339))
		}
		fmt.Fprint(w, "]}")
	})
	
	// システムメトリクスを定期更新
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			systemMetrics.UpdateGoroutineCount()
			
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			systemMetrics.UpdateMemoryUsage(float64(m.Alloc))
			
			// サンプルの接続数とキューサイズ
			systemMetrics.UpdateConnections(float64(10 + rand.Intn(90)))
			systemMetrics.UpdateQueueSize("processing", float64(rand.Intn(50)))
			systemMetrics.UpdateQueueSize("waiting", float64(rand.Intn(20)))
		}
	}()
	
	// アラート監視を開始
	ctx := context.Background()
	go alertManager.StartMonitoring(ctx, 10*time.Second)
	
	// Push メトリクス（有効な場合）
	if config.EnablePush {
		go metricsManager.StartPushMetrics(ctx)
	}
	
	fmt.Println("Metrics server starting on :8080")
	fmt.Println("Metrics endpoint: http://localhost:8080/metrics")
	fmt.Println("Sample API: http://localhost:8080/api/hello")
	fmt.Println("Error API: http://localhost:8080/api/error")
	fmt.Println("Alerts endpoint: http://localhost:8080/alerts")
	
	log.Fatal(http.ListenAndServe(":8080", mux))
}