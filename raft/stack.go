package raft

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrStackEmpty error = errors.New("stack empty")
	ErrStackFull  error = errors.New("stack full")
)

type Stack[T any] struct {
	file  io.ReadWriter
	items []T
	top   int
	size  uint
}

func NewStack[T any](size uint, path string) (*Stack[T], error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		return nil, err
	}

	stack := &Stack[T]{
		file:  file,
		items: []T{},
		top:   -1,
		size:  size,
	}

	// read items from ioReader
	items, err := stack.readItems()
	if err != nil {
		return nil, fmt.Errorf("error reading stack items from file; %v", err)
	}

	if len(items) == 0 {
		// add initial item
		var item T
		err = stack.Push(item, true)
		if err != nil {
			return nil, fmt.Errorf("error adding initial item; %v", err)
		}
	}

	for _, item := range items {
		err = stack.Push(item, false)
		if err != nil {
			return nil, fmt.Errorf("error adding item to stack; %v", err)
		}
	}

	return stack, nil
}

func (stack *Stack[T]) Push(item T, updateWriter bool) error {
	// copying the value of stack so that we can rollback any
	// changes if an error occurs
	new_stack := *stack

	if new_stack.isFull() {
		return ErrStackFull
	}

	new_stack.top += 1
	new_stack.items = append(new_stack.items, item)

	if updateWriter {
		err := new_stack.writeItems()
		if err != nil {
			return fmt.Errorf("error writing stack items to file; %v", err)
		}
	}

	*stack = new_stack
	return nil
}

func (stack *Stack[T]) Pop(updateWriter bool) (T, error) {
	// copying the value of stack so that we can rollback any
	// changes if an error occurs
	new_stack := *stack
	var item T

	if new_stack.isEmpty() {
		return item, ErrStackEmpty
	}

	item = new_stack.items[new_stack.top]
	new_stack.items = new_stack.items[:len(new_stack.items)-1]
	new_stack.top -= 1

	if updateWriter {
		err := new_stack.writeItems()
		if err != nil {
			return item, fmt.Errorf("error writing stack items to file; %v", err)
		}
	}

	*stack = new_stack
	return item, nil
}

func (stack Stack[T]) Peek() (T, error) {
	var item T

	if stack.isEmpty() {
		return item, ErrStackEmpty
	}

	item = stack.items[stack.top]
	return item, nil
}

func (stack Stack[T]) GetItems() []T {
	if stack.isEmpty() {
		return []T{}
	}

	items := stack.items[0 : stack.top+1]
	return items
}

func (stack Stack[T]) isEmpty() bool {
	return stack.top == -1
}

func (stack Stack[T]) isFull() bool {
	return stack.top == int(stack.size)-1
}

func (stack *Stack[T]) readItems() ([]T, error) {
	if stack.file == nil {
		return []T{}, fmt.Errorf("input file not provided in stack declaration")
	}

	items := []T{}
	var item T
	decoder := json.NewDecoder(stack.file)

	for {
		err := decoder.Decode(&item)
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		items = append(items, item)
	}

	return items, nil
}

// writes all stack items to the io.Writer
func (stack Stack[T]) writeItems() error {
	if stack.file == nil {
		return fmt.Errorf("output file not provided in stack declaration")
	}

	items := stack.items[0 : stack.top+1]
	encoder := json.NewEncoder(stack.file)

	for _, item := range items {
		err := encoder.Encode(item)
		if err != nil {
			return err
		}
	}

	return nil
}
