// Day 58: Prometheus Histogram
// リクエストのレイテンシ分布を計測するヒストグラムを実装

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Histogram Prometheusヒストグラム構造体
type Histogram struct {
	name         string
	help         string
	buckets      []float64
	bucketCounts []int64
	sum          float64
	count        int64
	mu           sync.RWMutex
}

// HistogramVec ラベル付きヒストグラム
type HistogramVec struct {
	name        string
	help        string
	buckets     []float64
	labelNames  []string
	histograms  map[string]*Histogram
	mu          sync.RWMutex
}

// HistogramObserver ヒストグラム観測器
type HistogramObserver struct {
	histogram *Histogram
}

// NewHistogram 新しいヒストグラムを作成
func NewHistogram(name, help string, buckets []float64) *Histogram {
	// バケットをソート
	sortedBuckets := make([]float64, len(buckets))
	copy(sortedBuckets, buckets)
	sort.Float64s(sortedBuckets)
	
	// +Infバケットを追加
	if len(sortedBuckets) == 0 || sortedBuckets[len(sortedBuckets)-1] != math.Inf(1) {
		sortedBuckets = append(sortedBuckets, math.Inf(1))
	}
	
	return &Histogram{
		name:         name,
		help:         help,
		buckets:      sortedBuckets,
		bucketCounts: make([]int64, len(sortedBuckets)),
		sum:          0,
		count:        0,
	}
}

// NewHistogramVec 新しいヒストグラムベクトルを作成
func NewHistogramVec(name, help string, labelNames []string, buckets []float64) *HistogramVec {
	return &HistogramVec{
		name:       name,
		help:       help,
		buckets:    buckets,
		labelNames: labelNames,
		histograms: make(map[string]*Histogram),
	}
}

// Observe 値を観測
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.sum += value
	h.count++
	
	// 適切なバケットを見つけて増加
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.bucketCounts[i]++
		}
	}
}

// ObserveMany 複数の値を一度に観測
func (h *Histogram) ObserveMany(values []float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	for _, value := range values {
		h.sum += value
		h.count++
		
		for i, bucket := range h.buckets {
			if value <= bucket {
				h.bucketCounts[i]++
			}
		}
	}
}

// GetStats 統計情報を取得
func (h *Histogram) GetStats() HistogramStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	stats := HistogramStats{
		Name:         h.name,
		Count:        h.count,
		Sum:          h.sum,
		BucketCounts: make([]BucketCount, len(h.buckets)),
	}
	
	if h.count > 0 {
		stats.Average = h.sum / float64(h.count)
	}
	
	for i, bucket := range h.buckets {
		stats.BucketCounts[i] = BucketCount{
			UpperBound: bucket,
			Count:      h.bucketCounts[i],
		}
	}
	
	return stats
}

// GetQuantile 分位数を計算
func (h *Histogram) GetQuantile(q float64) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if h.count == 0 {
		return 0
	}
	
	targetCount := float64(h.count) * q
	
	// 線形補間で分位数を推定
	var prevBucket float64 = 0
	var cumulativeCount int64 = 0
	
	for i, bucket := range h.buckets {
		newCumulativeCount := cumulativeCount + h.bucketCounts[i]
		
		if float64(newCumulativeCount) >= targetCount {
			// このバケット内で分位数を線形補間
			if h.bucketCounts[i] == 0 {
				return prevBucket
			}
			
			ratio := (targetCount - float64(cumulativeCount)) / float64(h.bucketCounts[i])
			return prevBucket + ratio*(bucket-prevBucket)
		}
		
		cumulativeCount = newCumulativeCount
		prevBucket = bucket
	}
	
	return h.buckets[len(h.buckets)-1]
}

// WithLabelValues ラベル値でヒストグラムを取得
func (hv *HistogramVec) WithLabelValues(values ...string) *HistogramObserver {
	if len(values) != len(hv.labelNames) {
		panic(fmt.Sprintf("expected %d label values, got %d", len(hv.labelNames), len(values)))
	}
	
	key := joinLabels(values)
	
	hv.mu.RLock()
	if histogram, exists := hv.histograms[key]; exists {
		hv.mu.RUnlock()
		return &HistogramObserver{histogram: histogram}
	}
	hv.mu.RUnlock()
	
	hv.mu.Lock()
	defer hv.mu.Unlock()
	
	// ダブルチェック
	if histogram, exists := hv.histograms[key]; exists {
		return &HistogramObserver{histogram: histogram}
	}
	
	// 新しいヒストグラムを作成
	histogram := NewHistogram(hv.name, hv.help, hv.buckets)
	hv.histograms[key] = histogram
	
	return &HistogramObserver{histogram: histogram}
}

// Observe ヒストグラムオブザーバーで値を観測
func (ho *HistogramObserver) Observe(value float64) {
	ho.histogram.Observe(value)
}

// GetAllStats 全てのヒストグラムの統計を取得
func (hv *HistogramVec) GetAllStats() map[string]HistogramStats {
	hv.mu.RLock()
	defer hv.mu.RUnlock()
	
	stats := make(map[string]HistogramStats)
	for key, histogram := range hv.histograms {
		stats[key] = histogram.GetStats()
	}
	
	return stats
}

// HistogramStats ヒストグラム統計情報
type HistogramStats struct {
	Name         string        `json:"name"`
	Count        int64         `json:"count"`
	Sum          float64       `json:"sum"`
	Average      float64       `json:"average"`
	BucketCounts []BucketCount `json:"bucket_counts"`
}

type BucketCount struct {
	UpperBound float64 `json:"upper_bound"`
	Count      int64   `json:"count"`
}

// RequestLatencyTracker リクエストレイテンシトラッカー
type RequestLatencyTracker struct {
	histogram *HistogramVec
}

func NewRequestLatencyTracker() *RequestLatencyTracker {
	// 典型的なレイテンシバケット（秒単位）
	buckets := []float64{
		0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
	}
	
	histogram := NewHistogramVec(
		"http_request_duration_seconds",
		"Time spent on HTTP requests",
		[]string{"method", "endpoint", "status"},
		buckets,
	)
	
	return &RequestLatencyTracker{
		histogram: histogram,
	}
}

// TrackRequest リクエストを追跡
func (t *RequestLatencyTracker) TrackRequest(method, endpoint, status string, duration time.Duration) {
	seconds := duration.Seconds()
	t.histogram.WithLabelValues(method, endpoint, status).Observe(seconds)
}

// GetStats 統計情報を取得
func (t *RequestLatencyTracker) GetStats() map[string]HistogramStats {
	return t.histogram.GetAllStats()
}

// HTTPミドルウェア実装
func (t *RequestLatencyTracker) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// レスポンスライターをラップしてステータスコードを取得
		wrappedWriter := &responseWriterSolution{ResponseWriter: w, statusCode: http.StatusOK}
		
		// 次のハンドラを実行
		next.ServeHTTP(wrappedWriter, r)
		
		// レイテンシを記録
		duration := time.Since(start)
		status := strconv.Itoa(wrappedWriter.statusCode)
		
		t.TrackRequest(r.Method, r.URL.Path, status, duration)
	})
}

type responseWriterSolution struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterSolution) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// PerformanceAnalyzerSolution パフォーマンス分析器
type PerformanceAnalyzerSolution struct {
	tracker *RequestLatencyTracker
}

func NewPerformanceAnalyzerSolution(tracker *RequestLatencyTracker) *PerformanceAnalyzerSolution {
	return &PerformanceAnalyzerSolution{
		tracker: tracker,
	}
}

// AnalyzePerformance パフォーマンス分析を実行
func (pa *PerformanceAnalyzerSolution) AnalyzePerformance() PerformanceReport {
	stats := pa.tracker.GetStats()
	
	report := PerformanceReport{
		Timestamp: time.Now(),
		Endpoints: make([]EndpointPerformance, 0),
	}
	
	// エンドポイント別の性能分析
	endpointStats := make(map[string]*EndpointPerformance)
	
	for labelKey, histStats := range stats {
		labels := parseLabels(labelKey)
		if len(labels) >= 2 {
			endpoint := labels[1]
			
			if ep, exists := endpointStats[endpoint]; exists {
				ep.TotalRequests += histStats.Count
				ep.TotalTime += histStats.Sum
			} else {
				endpointStats[endpoint] = &EndpointPerformance{
					Endpoint:      endpoint,
					TotalRequests: histStats.Count,
					TotalTime:     histStats.Sum,
				}
			}
		}
	}
	
	// 平均レイテンシとパーセンタイルを計算
	for _, ep := range endpointStats {
		if ep.TotalRequests > 0 {
			ep.AverageLatency = ep.TotalTime / float64(ep.TotalRequests)
		}
		
		// ヒストグラムから分位数を計算（簡略化）
		ep.P50Latency = pa.calculatePercentile(ep.Endpoint, 0.5)
		ep.P95Latency = pa.calculatePercentile(ep.Endpoint, 0.95)
		ep.P99Latency = pa.calculatePercentile(ep.Endpoint, 0.99)
		
		report.Endpoints = append(report.Endpoints, *ep)
	}
	
	// エンドポイントを平均レイテンシでソート
	sort.Slice(report.Endpoints, func(i, j int) bool {
		return report.Endpoints[i].AverageLatency > report.Endpoints[j].AverageLatency
	})
	
	return report
}

func (pa *PerformanceAnalyzer) calculatePercentile(endpoint string, percentile float64) float64 {
	stats := pa.tracker.GetStats()
	
	// エンドポイントに関連するヒストグラムを検索
	for labelKey, histStats := range stats {
		if contains(labelKey, endpoint) && histStats.Count > 0 {
			// 簡略化された分位数計算
			return pa.estimateQuantileFromBuckets(histStats.BucketCounts, percentile)
		}
	}
	
	return 0
}

func (pa *PerformanceAnalyzer) estimateQuantileFromBuckets(buckets []BucketCount, quantile float64) float64 {
	if len(buckets) == 0 {
		return 0
	}
	
	totalCount := buckets[len(buckets)-1].Count
	if totalCount == 0 {
		return 0
	}
	
	targetCount := float64(totalCount) * quantile
	
	var prevBound float64 = 0
	var cumulativeCount int64 = 0
	
	for _, bucket := range buckets {
		if float64(bucket.Count) >= targetCount-float64(cumulativeCount) {
			// 線形補間
			if bucket.Count-cumulativeCount == 0 {
				return prevBound
			}
			
			ratio := (targetCount - float64(cumulativeCount)) / float64(bucket.Count-cumulativeCount)
			return prevBound + ratio*(bucket.UpperBound-prevBound)
		}
		
		cumulativeCount = bucket.Count
		prevBound = bucket.UpperBound
	}
	
	return buckets[len(buckets)-1].UpperBound
}

type PerformanceReport struct {
	Timestamp time.Time             `json:"timestamp"`
	Endpoints []EndpointPerformance `json:"endpoints"`
}

type EndpointPerformance struct {
	Endpoint       string  `json:"endpoint"`
	TotalRequests  int64   `json:"total_requests"`
	TotalTime      float64 `json:"total_time"`
	AverageLatency float64 `json:"average_latency"`
	P50Latency     float64 `json:"p50_latency"`
	P95Latency     float64 `json:"p95_latency"`
	P99Latency     float64 `json:"p99_latency"`
}

// MetricsServer メトリクスサーバー
type MetricsServer struct {
	tracker  *RequestLatencyTracker
	analyzer *PerformanceAnalyzer
	mux      *http.ServeMux
}

func NewMetricsServer(tracker *RequestLatencyTracker) *MetricsServer {
	analyzer := NewPerformanceAnalyzerSolution(tracker)
	mux := http.NewServeMux()
	
	server := &MetricsServer{
		tracker:  tracker,
		analyzer: analyzer,
		mux:      mux,
	}
	
	server.setupRoutes()
	return server
}

func (ms *MetricsServer) setupRoutes() {
	ms.mux.HandleFunc("/metrics", ms.metricsHandler)
	ms.mux.HandleFunc("/metrics/histogram", ms.histogramHandler)
	ms.mux.HandleFunc("/metrics/performance", ms.performanceHandler)
}

func (ms *MetricsServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	stats := ms.tracker.GetStats()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"metrics": stats,
		"timestamp": time.Now(),
	})
}

func (ms *MetricsServer) histogramHandler(w http.ResponseWriter, r *http.Request) {
	stats := ms.tracker.GetStats()
	
	// Prometheus形式で出力
	w.Header().Set("Content-Type", "text/plain")
	
	for labelKey, histStats := range stats {
		fmt.Fprintf(w, "# HELP %s %s\n", histStats.Name, "Time spent on HTTP requests")
		fmt.Fprintf(w, "# TYPE %s histogram\n", histStats.Name)
		
		for _, bucket := range histStats.BucketCounts {
			if math.IsInf(bucket.UpperBound, 1) {
				fmt.Fprintf(w, "%s_bucket{%s,le=\"+Inf\"} %d\n", histStats.Name, labelKey, bucket.Count)
			} else {
				fmt.Fprintf(w, "%s_bucket{%s,le=\"%.3f\"} %d\n", histStats.Name, labelKey, bucket.UpperBound, bucket.Count)
			}
		}
		
		fmt.Fprintf(w, "%s_sum{%s} %.3f\n", histStats.Name, labelKey, histStats.Sum)
		fmt.Fprintf(w, "%s_count{%s} %d\n", histStats.Name, labelKey, histStats.Count)
	}
}

func (ms *MetricsServer) performanceHandler(w http.ResponseWriter, r *http.Request) {
	report := ms.analyzer.AnalyzePerformance()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (ms *MetricsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ms.mux.ServeHTTP(w, r)
}

// AlertingSystem アラートシステム
type AlertingSystem struct {
	tracker   *RequestLatencyTracker
	thresholds map[string]LatencyThreshold
	alertCh   chan Alert
}

type LatencyThreshold struct {
	P95Threshold float64
	P99Threshold float64
	ErrorRate    float64
}

type Alert struct {
	Type        string    `json:"type"`
	Endpoint    string    `json:"endpoint"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"`
	Timestamp   time.Time `json:"timestamp"`
	CurrentValue float64  `json:"current_value"`
	Threshold   float64   `json:"threshold"`
}

func NewAlertingSystem(tracker *RequestLatencyTracker) *AlertingSystem {
	return &AlertingSystem{
		tracker:    tracker,
		thresholds: make(map[string]LatencyThreshold),
		alertCh:    make(chan Alert, 100),
	}
}

func (as *AlertingSystem) SetThreshold(endpoint string, threshold LatencyThreshold) {
	as.thresholds[endpoint] = threshold
}

func (as *AlertingSystem) StartMonitoring(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			as.checkThresholds()
		}
	}
}

func (as *AlertingSystem) checkThresholds() {
	analyzer := NewPerformanceAnalyzer(as.tracker)
	report := analyzer.AnalyzePerformance()
	
	for _, ep := range report.Endpoints {
		if threshold, exists := as.thresholds[ep.Endpoint]; exists {
			// P95レイテンシチェック
			if ep.P95Latency > threshold.P95Threshold {
				alert := Alert{
					Type:         "latency_p95",
					Endpoint:     ep.Endpoint,
					Message:      fmt.Sprintf("P95 latency exceeds threshold"),
					Severity:     "warning",
					Timestamp:    time.Now(),
					CurrentValue: ep.P95Latency,
					Threshold:    threshold.P95Threshold,
				}
				
				select {
				case as.alertCh <- alert:
					log.Printf("Alert: P95 latency for %s is %.3fs (threshold: %.3fs)", 
						ep.Endpoint, ep.P95Latency, threshold.P95Threshold)
				default:
					log.Printf("Alert channel full, dropping alert")
				}
			}
			
			// P99レイテンシチェック
			if ep.P99Latency > threshold.P99Threshold {
				alert := Alert{
					Type:         "latency_p99",
					Endpoint:     ep.Endpoint,
					Message:      fmt.Sprintf("P99 latency exceeds threshold"),
					Severity:     "critical",
					Timestamp:    time.Now(),
					CurrentValue: ep.P99Latency,
					Threshold:    threshold.P99Threshold,
				}
				
				select {
				case as.alertCh <- alert:
					log.Printf("CRITICAL: P99 latency for %s is %.3fs (threshold: %.3fs)", 
						ep.Endpoint, ep.P99Latency, threshold.P99Threshold)
				default:
					log.Printf("Alert channel full, dropping critical alert")
				}
			}
		}
	}
}

func (as *AlertingSystem) GetAlerts() <-chan Alert {
	return as.alertCh
}

// ユーティリティ関数
func joinLabels(values []string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("label%d=\"%s\"", i, value)
	}
	return result
}

func parseLabels(labelKey string) []string {
	// 簡略化されたラベルパース（実際の実装ではより堅牢なパースが必要）
	labels := make([]string, 0)
	// この実装では簡略化
	return labels
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// サンプルHTTPハンドラー
func createSampleHandlers() http.Handler {
	mux := http.NewServeMux()
	
	// 様々なレイテンシパターンのエンドポイント
	mux.HandleFunc("/api/fast", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "ok", "endpoint": "fast"}`)
	})
	
	mux.HandleFunc("/api/medium", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "ok", "endpoint": "medium"}`)
	})
	
	mux.HandleFunc("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)
		if rand.Float64() < 0.1 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "internal server error"}`)
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status": "ok", "endpoint": "slow"}`)
		}
	})
	
	return mux
}

func mainSolution() {
	fmt.Println("Day 58: Prometheus Histogram")
	fmt.Println("Run 'go test -v' to see the histogram system in action")
	
	// デモンストレーション
	tracker := NewRequestLatencyTracker()
	metricsServer := NewMetricsServer(tracker)
	sampleHandlers := createSampleHandlers()
	
	// メトリクス付きハンドラーを作成
	instrumentedHandlers := tracker.Middleware(sampleHandlers)
	
	// サーバー起動
	go func() {
		log.Println("Sample API server starting on :8080")
		http.ListenAndServe(":8080", instrumentedHandlers)
	}()
	
	go func() {
		log.Println("Metrics server starting on :9090")
		http.ListenAndServe(":9090", metricsServer)
	}()
	
	// 負荷生成
	go func() {
		time.Sleep(1 * time.Second)
		for i := 0; i < 100; i++ {
			endpoints := []string{"/api/fast", "/api/medium", "/api/slow"}
			endpoint := endpoints[rand.Intn(len(endpoints))]
			
			go func(ep string) {
				resp, err := http.Get("http://localhost:8080" + ep)
				if err == nil {
					resp.Body.Close()
				}
			}(endpoint)
			
			time.Sleep(100 * time.Millisecond)
		}
	}()
	
	time.Sleep(30 * time.Second)
	log.Println("Demo completed")
}