// Package worker реализует пул горутин фиксированного размера для конкурентной обработки задач.
// Поддерживает плавную остановку через контекст и распределяет задачи через буферизованный канал.
package worker

import (
	"context"
	"log/slog"
	"sync"
)

// Task представляет собой функцию для обработке в пуле горутин.
// Принимает контект и должна обрабатывать отмену, возвращать ошибку
type Task func(ctx context.Context) error

// Pool структура которая содержит конфигурацию пула горутин обработки задач
type Pool struct {
	workers int
	tasks   chan Task
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *slog.Logger
}

// NewPool конструктор Pool, который возвращает сконфигурированный указатель на структуру
func NewPool(ctx context.Context, workers int, queueSize int, log *slog.Logger) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	return &Pool{
		workers: workers,
		tasks:   make(chan Task, queueSize),
		ctx:     ctx,
		cancel:  cancel,
		logger:  log,
	}
}

// Start запускает сконфигурированное количество воркеров для обработки задач
func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Go(func() {
			p.worker(i)
		})
	}
}

func (p *Pool) worker(id int) {
	for task := range p.tasks {
		if p.ctx.Err() != nil {
			return
		}
		if err := task(p.ctx); err != nil {
			p.logger.Error("task failed pool order processing", "worker_id", id, "error", err)
		}
	}
}

// Submit отправляет новую задачу в пул обработчиков.
// Метод ожидает освобождения места в буфере или завершения работы по одному из контекстов.
// task — задача для обработки в пуле.
// p.ctx — внутренний контекст пула для передачи сигнала завершения (например, при вызове Stop).
func (p *Pool) Submit(task Task) {
	select {
	case p.tasks <- task:
	case <-p.ctx.Done():
		p.logger.Warn("pool shutting down, task rejected pool order processing")
	}
}

// Stop завершает работу пула обработчиков, логирует завершение через p.logger.
// shutdownCtx — контекст завершения работы приложения.
func (p *Pool) Stop(shutdownCtx context.Context) {
	p.logger.Info("stopping worker pool pool order processing")
	close(p.tasks)
	consumersDoneChan := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(consumersDoneChan)
	}()

	select {
	case <-consumersDoneChan:
		p.logger.Info("worker pool stopped gracefully")
	case <-shutdownCtx.Done():
		p.logger.Warn("worker pool shutdown timed out, forcing cancel")
		p.cancel()
	}
}
