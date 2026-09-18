package worker

import (
	"context"
	"log/slog"
	"sync"
)

type Task func(ctx context.Context) error

type Pool struct {
	name    string
	workers int
	tasks   chan Task
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewPool(name string, workers int, queueSize int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		name:    name,
		workers: workers,
		tasks:   make(chan Task, queueSize),
		ctx:     ctx,
		cancel:  cancel,
	}
}

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
			slog.Error("Task failed", "pool", p.name, "worker_id", id, "error", err)
		}
	}
}

func (p *Pool) Submit(task Task) {
	select {
	case p.tasks <- task:
	case <-p.ctx.Done():
		slog.Warn("Pool shutting down, task rejected", "pool", p.name)
	}
}

func (p *Pool) Stop() {
	slog.Info("Stopping worker pool", "name", p.name)
	p.cancel()
	close(p.tasks)
	p.wg.Wait()
}
