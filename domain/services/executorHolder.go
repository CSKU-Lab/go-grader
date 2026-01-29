package services

import "sync/atomic"

type ExecutorHolder struct {
	current atomic.Value
}

func NewExecutorHolder(exec *ExecutorService) *ExecutorHolder {
	h := &ExecutorHolder{}
	h.current.Store(exec)
	return h
}

func (h *ExecutorHolder) Get() *ExecutorService {
	return h.current.Load().(*ExecutorService)
}

func (h *ExecutorHolder) Swap(exec *ExecutorService) {
	h.current.Store(exec)
}
