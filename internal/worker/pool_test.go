package worker_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/mersikovs/gomart/internal/worker"
)

func TestPool_Submit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		workers      int
		queueSize    int
		ctxCancelled bool
		task         worker.Task
		wantExecuted bool
	}{
		{
			name:         "успех_задача_выполняется",
			workers:      2,
			queueSize:    10,
			ctxCancelled: false,
			task: func(ctx context.Context) error {
				return nil
			},
			wantExecuted: true,
		},
		{
			name:         "ошибка_пул_уже_остановлен_контекст_отменен",
			workers:      2,
			queueSize:    10,
			ctxCancelled: true,
			task: func(ctx context.Context) error {
				return nil
			},
			wantExecuted: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			if tt.ctxCancelled {
				cancel()
			} else {
				defer cancel()
			}

			p := worker.NewPool(ctx, tt.workers, tt.queueSize, slog.Default())

			p.Start()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			defer p.Stop(shutdownCtx)

			done := make(chan struct{}, 1)

			wrappedTask := func(taskCtx context.Context) error {
				_ = tt.task(taskCtx)
				select {
				case done <- struct{}{}:
				default:
				}
				return nil
			}

			p.Submit(wrappedTask)

			if tt.wantExecuted {
				select {
				case <-done:
				case <-time.After(1 * time.Second):
					t.Fatal("задача не выполнилась за 1 секунду")
				}
			} else {
				select {
				case <-done:
					t.Fatal("задача не должна была выполниться")
				case <-time.After(200 * time.Millisecond):
				}
			}
		})
	}
}
