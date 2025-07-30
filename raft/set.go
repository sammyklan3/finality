package raft

import "sync"

type Set[T comparable] struct {
	items map[T]any
	mu    *sync.RWMutex
}

func (s Set[T]) Contains(key T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.items[key]
	return ok
}

func (s *Set[T]) Add(items ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range items {
		s.items[item] = nil
	}
}

func (s *Set[T]) Remove(items ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range items {
		delete(s.items, item)
	}
}

func (s Set[T]) GetItems() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := []T{}
	for key := range s.items {
		items = append(items, key)
	}
	return items
}

func NewSet[T comparable](items ...T) *Set[T] {
	set := Set[T]{
		items: make(map[T]any),
		mu:    &sync.RWMutex{},
	}

	for _, item := range items {
		set.items[item] = nil
	}
	return &set
}
