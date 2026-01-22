package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Task 表示一个任务
type Task func()

// ResultTask 表示返回结果的任务
type ResultTask func() (interface{}, error)

// Pool worker pool接口
type Pool interface {
	Submit(task Task)
	SubmitWithResult(task ResultTask) (interface{}, error)
	Close()
	Wait()
}

// WorkerPool worker pool实现
type WorkerPool struct {
	workerCount int32
	taskQueue   chan Task
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	closed      int32
}

// NewPool 创建新的worker pool
func NewPool(workerCount int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		workerCount: int32(workerCount),
		taskQueue:   make(chan Task, 1000), // 缓冲队列
		ctx:         ctx,
		cancel:      cancel,
	}

	// 启动workers
	for i := 0; i < workerCount; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// worker worker处理任务
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			p.runTask(task)
		}
	}
}

// runTask 执行任务，捕获panic
func (p *WorkerPool) runTask(task Task) {
	defer func() {
		if r := recover(); r != nil {
			// 记录panic但不中断worker
			fmt.Printf("Worker panic recovered: %v\n", r)
		}
	}()
	task()
}

// Submit 提交任务到pool
func (p *WorkerPool) Submit(task Task) {
	if atomic.LoadInt32(&p.closed) == 1 {
		panic("worker pool is closed")
	}

	select {
	case p.taskQueue <- task:
		// 任务已提交
	case <-p.ctx.Done():
		// pool已关闭
		return
	}
}

// SubmitWithResult 提交带结果的任务
func (p *WorkerPool) SubmitWithResult(task ResultTask) (interface{}, error) {
	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, fmt.Errorf("worker pool is closed")
	}

	type result struct {
		value interface{}
		err   error
	}

	resultChan := make(chan result, 1)

	p.Submit(func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- result{nil, fmt.Errorf("task panic: %v", r)}
			}
		}()

		value, err := task()
		resultChan <- result{value, err}
	})

	select {
	case res := <-resultChan:
		return res.value, res.err
	case <-p.ctx.Done():
		return nil, fmt.Errorf("worker pool closed before task completed")
	}
}

// Wait 等待所有任务完成
func (p *WorkerPool) Wait() {
	// 等待队列中的所有任务被处理
	for len(p.taskQueue) > 0 {
		// 简单等待，避免忙等待
		select {
		case <-time.After(10 * time.Millisecond):
		case <-p.ctx.Done():
			return
		}
	}
}

// Close 关闭worker pool
func (p *WorkerPool) Close() {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		// 已经关闭
		return
	}

	// 关闭任务队列
	close(p.taskQueue)

	// 取消所有workers
	p.cancel()

	// 等待所有workers退出
	p.wg.Wait()
}

// ActiveWorkerCount 获取活跃worker数量
func (p *WorkerPool) ActiveWorkerCount() int32 {
	return atomic.LoadInt32(&p.workerCount)
}

// QueueLength 获取任务队列长度
func (p *WorkerPool) QueueLength() int {
	return len(p.taskQueue)
}

// IsClosed 检查pool是否已关闭
func (p *WorkerPool) IsClosed() bool {
	return atomic.LoadInt32(&p.closed) == 1
}
