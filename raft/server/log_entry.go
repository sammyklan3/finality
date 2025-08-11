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
	logIndex atomic.Uint64

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
	raftLogs := readLogsFromFile(path)
	stack := Stack{
		items: []raft.Log{},
		keys:  make(map[uint64]any),
		top:   -1,
		size:  1000,
		path:  path,
	}

	err := stack.Push(raftLogs...)
	if err != nil {
		return stack, err
	}

	// Create initial log entry if stack is empty
	if stack.isEmpty() {
		raftLog := NewLog("INIT", 0)
		stack.Push(raftLog)
		stack.Commit()
	} else {
		prevLog, _ := stack.Peek()
		logIndex.Store(prevLog.Index + 1)
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

	numDeletedItems := 0
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
		numDeletedItems++
	}

	if numDeletedItems == 0 {
		return nil
	}

	items := stack.items[:len(stack.items)]
	return writeLogsToFile(stack.path, items...)
}

func (stack Stack) GetTopOf(bottomIndex, topIndex uint64) []raft.Log {
	items := []raft.Log{}
	startCollecting := false

	// Check if bottomIndex item exists
	if _, ok := stack.keys[bottomIndex]; !ok {
		return nil
	}

	// Check if topIndex item exists
	if topIndex != 0 {
		_, ok := stack.keys[topIndex]
		if !ok {
			return nil
		}
	}

	for {
		item, err := stack.Pop()
		if err != nil {
			return items
		}

		if item.Index == topIndex || topIndex == 0 {
			startCollecting = true
		}

		if item.Index == bottomIndex {
			break
		}

		if startCollecting {
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

	raftLogs := []raft.Log{}
	raftLog := raft.Log{}
	decoder := json.NewDecoder(file)

	for {
		err := decoder.Decode(&raftLog)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil
		}
		raftLogs = append(raftLogs, raftLog)
	}
	return raftLogs
}

func writeLogsToFile(path string, raftLogs ...raft.Log) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	for _, raftLog := range raftLogs {
		raftLog.Committed = true

		err := encoder.Encode(raftLog)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewLog(msg string, term uint64) raft.Log {
	oldIndex := logIndex.Load()
	logIndex.Add(1)

	return raft.Log{
		Index: oldIndex,
		Msg:   msg,
		Term:  term,
	}
}
