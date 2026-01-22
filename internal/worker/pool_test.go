package worker

import (
	"sync"
	"testing"
	"time"
)

func TestWorkerPool_Submit(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	var wg sync.WaitGroup
	results := make(chan int, 10)

	// 提交5个任务
	for i := 0; i < 5; i++ {
		wg.Add(1)
		taskNum := i
		pool.Submit(func() {
			defer wg.Done()
			results <- taskNum
			time.Sleep(10 * time.Millisecond)
		})
	}

	wg.Wait()
	close(results)

	count := 0
	for range results {
		count++
	}

	if count != 5 {
		t.Errorf("Expected 5 results, got %d", count)
	}
}

func TestWorkerPool_SubmitWithResult(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	result, err := pool.SubmitWithResult(func() (interface{}, error) {
		return 42, nil
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.(int) != 42 {
		t.Errorf("Expected 42, got %v", result)
	}
}

func TestWorkerPool_SubmitWithError(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	_, err := pool.SubmitWithResult(func() (interface{}, error) {
		return nil, &TestError{msg: "task failed"}
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "task failed" {
		t.Errorf("Expected 'task failed', got %v", err)
	}
}

func TestWorkerPool_Close(t *testing.T) {
	pool := NewPool(2)

	// 提交一些任务
	for i := 0; i < 5; i++ {
		pool.Submit(func() {
			time.Sleep(10 * time.Millisecond)
		})
	}

	// 关闭池
	pool.Close()

	// 等待池关闭
	time.Sleep(100 * time.Millisecond)

	// 再次关闭应该是安全的
	pool.Close()
}

func TestWorkerPool_Concurrent(t *testing.T) {
	pool := NewPool(10)
	defer pool.Close()

	var wg sync.WaitGroup
	counter := int64(0)
	var mu sync.Mutex

	// 并发提交100个任务
	for i := 0; i < 100; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			mu.Lock()
			counter++
			mu.Unlock()
		})
	}

	wg.Wait()

	if counter != 100 {
		t.Errorf("Expected 100, got %d", counter)
	}
}

func TestWorkerPool_Wait(t *testing.T) {
	pool := NewPool(2)

	var wg sync.WaitGroup
	results := make(chan int, 10)

	// 提交5个任务
	for i := 0; i < 5; i++ {
		wg.Add(1)
		taskNum := i
		pool.Submit(func() {
			defer wg.Done()
			time.Sleep(20 * time.Millisecond)
			results <- taskNum
		})
	}

	// 等待所有任务完成
	pool.Wait()
	wg.Wait()
	close(results)

	count := 0
	for range results {
		count++
	}

	if count != 5 {
		t.Errorf("Expected 5 results, got %d", count)
	}

	pool.Close()
}

func TestWorkerPool_TaskPanic(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	// 提交会panic的任务
	pool.Submit(func() {
		panic("task panic")
	})

	// 提交正常任务
	var wg sync.WaitGroup
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
	})

	// 等待正常任务完成
	wg.Wait()
	// pool应该能正常关闭而不会panic
}

// TestError 测试用的错误类型
type TestError struct {
	msg string
}

func (e *TestError) Error() string {
	return e.msg
}

func BenchmarkWorkerPool(b *testing.B) {
	pool := NewPool(10)
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(func() {
			time.Sleep(1 * time.Microsecond)
		})
	}
	pool.Wait()
}
