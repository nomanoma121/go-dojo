package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// User represents a user entity
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Data represents generic data structure
type Data struct {
	ID    int         `json:"id"`
	Value interface{} `json:"value"`
}

// SortingAlgorithms contains various sorting implementations
type SortingAlgorithms struct{}

// BubbleSort implements bubble sort algorithm
func (s *SortingAlgorithms) BubbleSort(data []int) {
	n := len(data)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if data[j] > data[j+1] {
				data[j], data[j+1] = data[j+1], data[j]
			}
		}
	}
}

// QuickSort implements quick sort algorithm
func (s *SortingAlgorithms) QuickSort(data []int) {
	if len(data) <= 1 {
		return
	}
	s.quickSortHelper(data, 0, len(data)-1)
}

func (s *SortingAlgorithms) quickSortHelper(data []int, low, high int) {
	if low < high {
		pi := s.partition(data, low, high)
		s.quickSortHelper(data, low, pi-1)
		s.quickSortHelper(data, pi+1, high)
	}
}

func (s *SortingAlgorithms) partition(data []int, low, high int) int {
	pivot := data[high]
	i := low - 1

	for j := low; j < high; j++ {
		if data[j] <= pivot {
			i++
			data[i], data[j] = data[j], data[i]
		}
	}
	data[i+1], data[high] = data[high], data[i+1]
	return i + 1
}

// MergeSort implements merge sort algorithm
func (s *SortingAlgorithms) MergeSort(data []int) {
	if len(data) <= 1 {
		return
	}
	s.mergeSortHelper(data, 0, len(data)-1)
}

func (s *SortingAlgorithms) mergeSortHelper(data []int, left, right int) {
	if left < right {
		mid := (left + right) / 2
		s.mergeSortHelper(data, left, mid)
		s.mergeSortHelper(data, mid+1, right)
		s.merge(data, left, mid, right)
	}
}

func (s *SortingAlgorithms) merge(data []int, left, mid, right int) {
	leftArr := make([]int, mid-left+1)
	rightArr := make([]int, right-mid)

	copy(leftArr, data[left:mid+1])
	copy(rightArr, data[mid+1:right+1])

	i, j, k := 0, 0, left

	for i < len(leftArr) && j < len(rightArr) {
		if leftArr[i] <= rightArr[j] {
			data[k] = leftArr[i]
			i++
		} else {
			data[k] = rightArr[j]
			j++
		}
		k++
	}

	for i < len(leftArr) {
		data[k] = leftArr[i]
		i++
		k++
	}

	for j < len(rightArr) {
		data[k] = rightArr[j]
		j++
		k++
	}
}

// HeapSort implements heap sort algorithm
func (s *SortingAlgorithms) HeapSort(data []int) {
	n := len(data)

	// Build max heap
	for i := n/2 - 1; i >= 0; i-- {
		s.heapify(data, n, i)
	}

	// Extract elements from heap
	for i := n - 1; i > 0; i-- {
		data[0], data[i] = data[i], data[0]
		s.heapify(data, i, 0)
	}
}

func (s *SortingAlgorithms) heapify(data []int, n, i int) {
	largest := i
	left := 2*i + 1
	right := 2*i + 2

	if left < n && data[left] > data[largest] {
		largest = left
	}

	if right < n && data[right] > data[largest] {
		largest = right
	}

	if largest != i {
		data[i], data[largest] = data[largest], data[i]
		s.heapify(data, n, largest)
	}
}

// StringProcessor handles string operations
type StringProcessor struct{}

// Concatenate concatenates strings using + operator
func (sp *StringProcessor) Concatenate(strs []string) string {
	result := ""
	for _, s := range strs {
		result += s
	}
	return result
}

// BuilderConcatenate concatenates strings using strings.Builder
func (sp *StringProcessor) BuilderConcatenate(strs []string) string {
	var builder strings.Builder
	
	// Pre-allocate capacity for better performance
	totalLen := 0
	for _, s := range strs {
		totalLen += len(s)
	}
	builder.Grow(totalLen)
	
	for _, s := range strs {
		builder.WriteString(s)
	}
	return builder.String()
}

// ByteConcatenate concatenates strings using byte operations
func (sp *StringProcessor) ByteConcatenate(strs []string) string {
	// Calculate total length
	totalLen := 0
	for _, s := range strs {
		totalLen += len(s)
	}
	
	// Create byte slice with exact capacity
	result := make([]byte, 0, totalLen)
	
	for _, s := range strs {
		result = append(result, []byte(s)...)
	}
	
	return string(result)
}

// SearchAlgorithms contains search implementations
type SearchAlgorithms struct{}

// LinearSearch performs linear search
func (sa *SearchAlgorithms) LinearSearch(data []int, target int) int {
	for i, v := range data {
		if v == target {
			return i
		}
	}
	return -1
}

// BinarySearch performs binary search
func (sa *SearchAlgorithms) BinarySearch(data []int, target int) int {
	left, right := 0, len(data)-1
	
	for left <= right {
		mid := (left + right) / 2
		
		if data[mid] == target {
			return mid
		} else if data[mid] < target {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	
	return -1
}

// ConcurrencyManager handles concurrent operations
type ConcurrencyManager struct {
	mu    sync.RWMutex
	data  map[int]int
	smap  sync.Map
	pool  *WorkerPool
	
	// Channel-based storage
	chReqs  chan channelRequest
	chStore map[int]int
}

type channelRequest struct {
	op      string
	key     int
	value   int
	respCh  chan channelResponse
}

type channelResponse struct {
	value  int
	exists bool
}

// NewConcurrencyManager creates a new concurrency manager
func NewConcurrencyManager(poolSize int) *ConcurrencyManager {
	cm := &ConcurrencyManager{
		data:    make(map[int]int),
		pool:    NewWorkerPool(poolSize),
		chReqs:  make(chan channelRequest),
		chStore: make(map[int]int),
	}
	
	// Start channel-based storage goroutine
	go cm.channelStorageHandler()
	
	return cm
}

func (cm *ConcurrencyManager) channelStorageHandler() {
	for req := range cm.chReqs {
		switch req.op {
		case "read":
			value, exists := cm.chStore[req.key]
			req.respCh <- channelResponse{value: value, exists: exists}
		case "write":
			cm.chStore[req.key] = req.value
			req.respCh <- channelResponse{}
		}
	}
}

// MutexRead reads data using mutex
func (cm *ConcurrencyManager) MutexRead(key int) (int, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	value, exists := cm.data[key]
	return value, exists
}

// MutexWrite writes data using mutex
func (cm *ConcurrencyManager) MutexWrite(key, value int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.data[key] = value
}

// SyncMapRead reads data using sync.Map
func (cm *ConcurrencyManager) SyncMapRead(key int) (int, bool) {
	if value, exists := cm.smap.Load(key); exists {
		return value.(int), true
	}
	return 0, false
}

// SyncMapWrite writes data using sync.Map
func (cm *ConcurrencyManager) SyncMapWrite(key, value int) {
	cm.smap.Store(key, value)
}

// ChannelRead reads data using channel
func (cm *ConcurrencyManager) ChannelRead(key int) (int, bool) {
	respCh := make(chan channelResponse, 1)
	cm.chReqs <- channelRequest{
		op:     "read",
		key:    key,
		respCh: respCh,
	}
	
	resp := <-respCh
	return resp.value, resp.exists
}

// ChannelWrite writes data using channel
func (cm *ConcurrencyManager) ChannelWrite(key, value int) {
	respCh := make(chan channelResponse, 1)
	cm.chReqs <- channelRequest{
		op:     "write",
		key:    key,
		value:  value,
		respCh: respCh,
	}
	
	<-respCh
}

// WorkerPool implements worker pool pattern
type WorkerPool struct {
	workers   int
	jobs      chan Job
	results   chan Result
	wg        sync.WaitGroup
	done      chan bool
}

// Job represents a work unit
type Job struct {
	ID   int
	Data interface{}
}

// Result represents job result
type Result struct {
	JobID int
	Value interface{}
	Error error
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int) *WorkerPool {
	wp := &WorkerPool{
		workers: workers,
		jobs:    make(chan Job, 100),
		results: make(chan Result, 100),
		done:    make(chan bool),
	}
	
	// Start worker goroutines
	for i := 0; i < workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
	
	return wp
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	
	for {
		select {
		case job, ok := <-wp.jobs:
			if !ok {
				return
			}
			
			// Process job
			result := wp.processJob(job)
			wp.results <- result
			
		case <-wp.done:
			return
		}
	}
}

func (wp *WorkerPool) processJob(job Job) Result {
	// Simulate some work
	switch data := job.Data.(type) {
	case []int:
		return Result{
			JobID: job.ID,
			Value: ProcessData(data),
			Error: nil,
		}
	default:
		return Result{
			JobID: job.ID,
			Value: nil,
			Error: fmt.Errorf("unsupported data type"),
		}
	}
}

// Submit submits a job to the pool
func (wp *WorkerPool) Submit(job Job) {
	wp.jobs <- job
}

// GetResult gets result from the pool
func (wp *WorkerPool) GetResult() Result {
	return <-wp.results
}

// Close closes the worker pool
func (wp *WorkerPool) Close() {
	close(wp.jobs)
	
	// Signal workers to stop
	for i := 0; i < wp.workers; i++ {
		wp.done <- true
	}
	
	wp.wg.Wait()
	close(wp.results)
	close(wp.done)
}

// MemoryOptimizer handles memory optimization techniques
type MemoryOptimizer struct {
	objectPool sync.Pool
}

// NewMemoryOptimizer creates a new memory optimizer
func NewMemoryOptimizer() *MemoryOptimizer {
	return &MemoryOptimizer{
		objectPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 1024)
			},
		},
	}
}

// GetBuffer gets buffer from pool
func (mo *MemoryOptimizer) GetBuffer() []byte {
	buf := mo.objectPool.Get().([]byte)
	// Reset buffer
	for i := range buf {
		buf[i] = 0
	}
	return buf
}

// PutBuffer returns buffer to pool
func (mo *MemoryOptimizer) PutBuffer(buf []byte) {
	if cap(buf) == 1024 {
		mo.objectPool.Put(buf)
	}
}

// ProcessWithPool processes data using object pool
func (mo *MemoryOptimizer) ProcessWithPool(data []byte) []byte {
	buf := mo.GetBuffer()
	defer mo.PutBuffer(buf)
	
	// Process data
	result := make([]byte, len(data))
	copy(result, data)
	
	// Do some processing with buffer
	for i := 0; i < len(result) && i < len(buf); i++ {
		result[i] = result[i] ^ buf[i%len(buf)]
	}
	
	return result
}

// ProcessWithoutPool processes data without object pool
func (mo *MemoryOptimizer) ProcessWithoutPool(data []byte) []byte {
	buf := make([]byte, 1024)
	
	result := make([]byte, len(data))
	copy(result, data)
	
	// Do some processing with buffer
	for i := 0; i < len(result) && i < len(buf); i++ {
		result[i] = result[i] ^ buf[i%len(buf)]
	}
	
	return result
}

// SliceOptimizer handles slice optimization
type SliceOptimizer struct{}

// PreallocatedAppend appends with preallocated slice
func (so *SliceOptimizer) PreallocatedAppend(size int) []int {
	result := make([]int, 0, size)
	for i := 0; i < size; i++ {
		result = append(result, i)
	}
	return result
}

// DynamicAppend appends without preallocation
func (so *SliceOptimizer) DynamicAppend(size int) []int {
	var result []int
	for i := 0; i < size; i++ {
		result = append(result, i)
	}
	return result
}

// InterfaceProcessor handles interface vs concrete type performance
type InterfaceProcessor struct{}

// ProcessInterface processes data using interface
func (ip *InterfaceProcessor) ProcessInterface(data interface{}) interface{} {
	switch v := data.(type) {
	case int:
		return v * 2
	case string:
		return v + "_processed"
	case []int:
		result := make([]int, len(v))
		for i, val := range v {
			result[i] = val * 2
		}
		return result
	default:
		return data
	}
}

// ProcessConcrete processes data using concrete type
func (ip *InterfaceProcessor) ProcessConcrete(data int) int {
	return data * 2
}

// FileProcessor handles file I/O operations
type FileProcessor struct{}

// WriteFile writes data to file
func (fp *FileProcessor) WriteFile(filename string, data []byte) error {
	return ioutil.WriteFile(filename, data, 0644)
}

// ReadFile reads data from file
func (fp *FileProcessor) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

// JSONProcessor handles JSON operations
type JSONProcessor struct{}

// EncodeJSON encodes data to JSON
func (jp *JSONProcessor) EncodeJSON(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// DecodeJSON decodes JSON data
func (jp *JSONProcessor) DecodeJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// HTTPProcessor handles HTTP operations
type HTTPProcessor struct{}

// HandleRequest handles HTTP request
func (hp *HTTPProcessor) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// Parse request
	userID := r.URL.Query().Get("id")
	if userID == "" {
		http.Error(w, "Missing user ID", http.StatusBadRequest)
		return
	}
	
	// Generate response
	id, err := strconv.Atoi(userID)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	
	user := User{
		ID:    id,
		Name:  fmt.Sprintf("User %d", id),
		Email: fmt.Sprintf("user%d@example.com", id),
	}
	
	// Write response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// ServeStatic serves static content
func (hp *HTTPProcessor) ServeStatic(w http.ResponseWriter, r *http.Request) {
	// Simple static content
	content := `<!DOCTYPE html>
<html>
<head><title>Static Page</title></head>
<body><h1>Hello, World!</h1></body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
}

// Utility functions for benchmarks

// GenerateRandomData generates random integer slice
func GenerateRandomData(size int) []int {
	rand.Seed(time.Now().UnixNano())
	data := make([]int, size)
	for i := 0; i < size; i++ {
		data[i] = rand.Intn(1000)
	}
	return data
}

// GenerateRandomStrings generates random string slice
func GenerateRandomStrings(count, length int) []string {
	rand.Seed(time.Now().UnixNano())
	strs := make([]string, count)
	
	for i := 0; i < count; i++ {
		bytes := make([]byte, length)
		for j := 0; j < length; j++ {
			bytes[j] = byte(rand.Intn(26) + 'a')
		}
		strs[i] = string(bytes)
	}
	
	return strs
}

// GenerateRandomUsers generates random user slice
func GenerateRandomUsers(count int) []User {
	rand.Seed(time.Now().UnixNano())
	users := make([]User, count)
	
	for i := 0; i < count; i++ {
		users[i] = User{
			ID:    i + 1,
			Name:  fmt.Sprintf("User %d", i+1),
			Email: fmt.Sprintf("user%d@example.com", i+1),
		}
	}
	
	return users
}

// IsSorted checks if slice is sorted
func IsSorted(data []int) bool {
	for i := 1; i < len(data); i++ {
		if data[i] < data[i-1] {
			return false
		}
	}
	return true
}

// ProcessData performs some CPU-intensive operation
func ProcessData(data []int) int {
	sum := 0
	for _, v := range data {
		sum += v * v
	}
	return sum
}

// main function for example usage
func main() {
	// Example usage of implemented algorithms
	sorter := &SortingAlgorithms{}
	data := GenerateRandomData(1000)
	
	fmt.Println("Original data length:", len(data))
	
	// Test sorting
	testData := make([]int, len(data))
	copy(testData, data)
	sorter.QuickSort(testData)
	fmt.Println("Sorted:", IsSorted(testData))
	
	// Test string processing
	processor := &StringProcessor{}
	strs := GenerateRandomStrings(100, 10)
	result := processor.BuilderConcatenate(strs)
	fmt.Println("Concatenated length:", len(result))
	
	// Test concurrency
	cm := NewConcurrencyManager(10)
	cm.MutexWrite(1, 100)
	value, exists := cm.MutexRead(1)
	fmt.Printf("Value: %d, Exists: %v\n", value, exists)
	
	// Clean up
	cm.pool.Close()
}