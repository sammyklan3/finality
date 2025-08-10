package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync/atomic"

	"github.com/sammyklan3/finality/raft"
)

var (
	log_index atomic.Uint64

	ErrStackEmpty error = errors.New("stack empty")
	ErrStackFull  error = errors.New("stack full")
	ErrNotFound   error = errors.New("log entry not found on stack")
)

type Stack struct {
	items []raft.Log
	keys  map[uint64]any
	top   int
	size  uint

	path string
}

func NewLogEntries(path string) (Stack, error) {
	raft_logs := readLogsFromFile(path)
	stack := Stack{
		items: []raft.Log{},
		keys:  make(map[uint64]any),
		top:   -1,
		size:  1000,
		path:  path,
	}

	err := stack.Push(raft_logs...)
	if err != nil {
		return stack, err
	}

	// Create initial log entry if stack is empty
	if stack.isEmpty() {
		new_log := newLog("INIT", 0)
		stack.Push(new_log)
		stack.Commit()
	} else {
		prev_log, _ := stack.Peek()
		log_index.Store(prev_log.Index + 1)
	}
	return stack, err
}

func (stack *Stack) Push(items ...raft.Log) error {
	for _, item := range items {
		if stack.isFull() {
			return ErrStackFull
		}

		// Check if item already in stack
		if _, ok := stack.keys[item.Index]; ok {
			continue
		}

		stack.top++
		stack.items = append(stack.items, item)
		stack.keys[item.Index] = nil
	}
	return nil
}

func (stack *Stack) Pop() (raft.Log, error) {
	var item raft.Log

	if stack.isEmpty() {
		return item, ErrStackEmpty
	}

	item = stack.items[stack.top]
	stack.items = stack.items[:len(stack.items)-1]
	delete(stack.keys, item.Index)
	stack.top--
	return item, nil
}

func (stack Stack) Peek() (raft.Log, error) {
	var item raft.Log
	if stack.isEmpty() {
		return item, ErrStackEmpty
	}

	item = stack.items[stack.top]
	return item, nil
}

func (stack Stack) EntryExists(index uint64) bool {
	_, ok := stack.keys[index]
	return ok
}

func (stack *Stack) Truncate(index uint64) error {
	if _, ok := stack.keys[index]; !ok {
		return ErrNotFound
	}

	num_deleted_items := 0
	for {
		item, err := stack.Peek()
		if err != nil {
			return err
		}

		if item.Index == index {
			break
		}
		_, err = stack.Pop()
		if err != nil {
			return err
		}
		num_deleted_items++
	}

	if num_deleted_items == 0 {
		return nil
	}

	items := stack.items[:len(stack.items)]
	return writeLogsToFile(stack.path, items...)
}

func (stack Stack) GetTopOf(bottom_index, top_index uint64) []raft.Log {
	items := []raft.Log{}
	start_collecting := false

	// Check if bottom_index item exists
	if _, ok := stack.keys[bottom_index]; !ok {
		return nil
	}

	// Check if top_index item exists
	if top_index != 0 {
		_, ok := stack.keys[top_index]
		if !ok {
			return nil
		}
	}

	for {
		item, err := stack.Pop()
		if err != nil {
			return items
		}

		if item.Index == top_index || top_index == 0 {
			start_collecting = true
		}

		if item.Index == bottom_index {
			break
		}

		if start_collecting {
			items = append(items, item)
		}
	}
	return items
}

// Saves raft logs that exist on the stack onto a file.
// Make sure to Push() items onto the stack before committing them;
// otherwise they will NOT be committed
func (stack Stack) Commit() error {
	items := stack.items[:len(stack.items)]
	return writeLogsToFile(stack.path, items...)
}

func (stack Stack) isEmpty() bool {
	return stack.top < 0
}

func (stack Stack) isFull() bool {
	return stack.top == int(stack.size)-1
}

func readLogsFromFile(path string) []raft.Log {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0700)
	if err != nil {
		return nil
	}
	defer file.Close()

	raft_logs := []raft.Log{}
	raft_log := raft.Log{}
	decoder := json.NewDecoder(file)

	for {
		err := decoder.Decode(&raft_log)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil
		}
		raft_logs = append(raft_logs, raft_log)
	}
	return raft_logs
}

func writeLogsToFile(path string, raft_logs ...raft.Log) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	for _, raft_log := range raft_logs {
		raft_log.Committed = true

		err := encoder.Encode(raft_log)
		if err != nil {
			return err
		}
	}
	return nil
}

func newLog(msg string, term uint64) raft.Log {
	old_index := log_index.Load()
	log_index.Add(1)

	return raft.Log{
		Index: old_index,
		Msg:   msg,
		Term:  term,
	}
}
