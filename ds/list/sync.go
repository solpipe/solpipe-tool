package list

import "sync"

type SyncList[T any] struct {
	Size  uint32
	head  *SyncNode[T]
	tail  *SyncNode[T]
	mutex *sync.Mutex
}

func CreateSync[T any]() *SyncList[T] {
	g := new(SyncList[T])
	g.Size = 0
	g.head = nil
	g.tail = nil
	g.mutex = &sync.Mutex{}
	return g
}

// attach obj to the end of a linked list
func (g *SyncList[T]) Append(v T) *SyncNode[T] {
	g.mutex.Lock()
	g.Size++
	node := CreateBlankSyncNode(v)
	if g.tail == nil {
		g.head = node
		g.tail = node
	} else {
		oldTail := g.tail
		node.prev = oldTail
		oldTail.next = node
		g.tail = node
	}
	g.mutex.Unlock()
	return node
}

func (g *SyncList[T]) Head() (ans T, is_present bool) {
	g.mutex.Lock()
	if g.head == nil {
		is_present = false
	} else {
		is_present = true
		ans = g.head.value
	}
	g.mutex.Unlock()
	return
}

func (g *SyncList[T]) HeadNode() *SyncNode[T] {
	return g.head
}

func (g *SyncList[T]) TailNode() *SyncNode[T] {
	return g.tail
}

// remove and return the first element of the linked list
func (g *SyncList[T]) Pop() (ans T, is_present bool) {

	head := g.head
	if head == nil {
		is_present = false
	} else {
		ans = head.value
		g.Remove(head)
		is_present = true
	}

	return
}

func (g *SyncList[T]) Tail() (ans T, is_present bool) {
	g.mutex.Lock()
	if g.tail == nil {
		is_present = false
	} else {
		is_present = true
		ans = g.tail.value
	}
	g.mutex.Unlock()
	return
}

func (g *SyncList[T]) Iterate(callback func(obj T, index uint32, delete func()) error) error {
	var i uint32 = 0
	var err error
	for node := g.head; node != nil; node = node.next {
		err = callback(node.value, i, func() { g.Remove(node) })
		if err != nil {
			return err
		}
		i++
	}
	return nil
}

func (g *SyncList[T]) IterateReverse(callback func(obj T, index uint32, delete func()) error) error {
	var i uint32 = g.Size - 1
	var err error
	for node := g.tail; node != nil; node = node.prev {
		err = callback(node.value, i, func() {
			g.Remove(node)
		})
		if err != nil {
			return err
		}
		i--
	}
	return nil
}

func (g *SyncList[T]) Array() []T {
	ans := make([]T, g.Size)
	g.Iterate(func(obj T, index uint32, delete func()) error {
		ans[index] = obj
		return nil
	})
	return ans
}

func (g *SyncList[T]) Remove(node *SyncNode[T]) {
	if node == nil {
		return
	}
	g.mutex.Lock()
	prevNode := node.prev
	nextNode := node.next

	g.Size = g.Size - 1

	// sort out links
	if prevNode == nil && nextNode == nil {
		g.head = nil
		g.tail = nil
	} else if prevNode == nil {
		g.head = nextNode
		nextNode.prev = nil
	} else if nextNode == nil {
		g.tail = prevNode
		prevNode.next = nil
	} else {
		prevNode.next = nextNode
		nextNode.prev = prevNode
	}
	g.mutex.Unlock()
}

func (g *SyncList[T]) Insert(v T, prevNode *SyncNode[T]) *SyncNode[T] {
	if prevNode == nil {
		return nil
	}
	g.mutex.Lock()

	middleNode := CreateBlankSyncNode(v)
	g.Size++
	nextNode := prevNode.Next()
	if nextNode == nil {
		oldTail := g.tail
		g.tail = middleNode
		middleNode.prev = oldTail
		oldTail.next = middleNode
	} else {
		middleNode.prev = prevNode
		middleNode.next = nextNode
		prevNode.next = middleNode
		nextNode.prev = middleNode
	}
	g.mutex.Unlock()
	return middleNode
}

type SyncNode[T any] struct {
	next  *SyncNode[T]
	prev  *SyncNode[T]
	value T
	mutex *sync.Mutex
}

func CreateBlankSyncNode[T any](v T) *SyncNode[T] {
	return &SyncNode[T]{value: v, mutex: &sync.Mutex{}}
}

func (n *SyncNode[T]) Next() *SyncNode[T] {
	return n.next
}

func (n *SyncNode[T]) Prev() *SyncNode[T] {
	return n.prev
}

// no copy is done here so be careful
func (n *SyncNode[T]) Value() T {
	return n.value
}

func (n *SyncNode[T]) ChangeValue(v T) {
	n.mutex.Lock()
	n.value = v
	n.mutex.Unlock()
}
