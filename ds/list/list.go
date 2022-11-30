package list

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/ds/radix"
)

type Generic[T any] struct {
	Size uint32
	head *Node[T]
	tail *Node[T]
}

func CreateGeneric[T any]() *Generic[T] {
	g := new(Generic[T])
	g.Size = 0
	g.head = nil
	g.tail = nil
	return g
}

// attach obj to the end of a linked list
func (g *Generic[T]) Append(obj T) *Node[T] {
	g.Size++
	node := &Node[T]{next: nil, prev: nil, value: obj}
	if g.tail == nil {
		g.head = node
		g.tail = node
	} else {
		oldTail := g.tail
		node.prev = oldTail
		oldTail.next = node
		g.tail = node
	}
	return node
}

func (g *Generic[T]) Prepend(obj T) *Node[T] {
	g.Size++
	node := &Node[T]{next: nil, prev: nil, value: obj}
	oldHead := g.head
	g.head = node
	node.next = g.head
	if oldHead != nil {
		oldHead.prev = node
	}
	return node
}

func (g *Generic[T]) Head() (ans T, is_present bool) {
	if g.head == nil {
		is_present = false
	} else {
		is_present = true
		ans = g.head.value
	}
	return
}

func (g *Generic[T]) HeadNode() *Node[T] {
	return g.head
}

func (g *Generic[T]) TailNode() *Node[T] {
	return g.tail
}

// remove and return the first element of the linked list
func (g *Generic[T]) Pop() (ans T, is_present bool) {

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

func (g *Generic[T]) Tail() (ans T, is_present bool) {
	if g.tail == nil {
		is_present = false
	} else {
		is_present = true
		ans = g.tail.value
	}
	return
}

func (g *Generic[T]) Iterate(callback func(obj T, index uint32, delete func()) error) error {
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

func (g *Generic[T]) InsertSorted(newItem T, insertOk func(before *Node[T], after *Node[T], newItem T) bool) *Node[T] {
	var node *Node[T]
	if insertOk(nil, g.HeadNode(), newItem) {
		return g.Prepend(newItem)
	}

	for node = g.HeadNode(); node != nil; node = node.Next() {
		if insertOk(node, node.Next(), newItem) {
			return g.Insert(newItem, node)
		}
	}
	return g.Append(newItem)
}

func ComparePublicKey(a sgo.PublicKey, b sgo.PublicKey) int {
	x := a.Bytes()[:]
	y := b.Bytes()[:]
	ans := 0
	for i := 0; i < len(x); i++ {
		if x[i] < y[i] {
			ans = -1
			break
		} else if x[i] > y[i] {
			ans = 1
			break
		}
	}
	return ans
}

func (g *Generic[T]) IterateReverse(callback func(obj T, index uint32, delete func()) error) error {
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

func (g *Generic[T]) Array() []T {
	ans := make([]T, g.Size)
	g.Iterate(func(obj T, index uint32, delete func()) error {
		ans[index] = obj
		return nil
	})
	return ans
}

func (g *Generic[T]) Remove(node *Node[T]) {
	if node == nil {
		return
	}
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
}

// insert after prevNode
func (g *Generic[T]) Insert(v T, prevNode *Node[T]) *Node[T] {
	if prevNode == nil && 0 < g.Size {
		return nil
	} else if prevNode == nil {
		return g.Append(v)
	}
	middleNode := &Node[T]{value: v}
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

	return middleNode
}

type Node[T any] struct {
	next  *Node[T]
	prev  *Node[T]
	value T
}

func (n *Node[T]) Next() *Node[T] {
	return n.next
}

func (n *Node[T]) Prev() *Node[T] {
	return n.prev
}

// no copy is done here so be careful
func (n *Node[T]) Value() T {
	return n.value
}

func (n *Node[T]) ChangeValue(v T) {
	n.value = v
}

// Create a radix index to do quick inserts and look-ups.
func (g *Generic[T]) RadixIndex(getKey func(T) string) (tree *radix.Tree[*Node[T]], err error) {
	tree = radix.New[*Node[T]]()
	var result bool
	for node := g.HeadNode(); node != nil; node = node.Next() {
		_, result = tree.Insert(getKey(node.Value()), node)
		if !result {
			err = errors.New("insert failed")
			return
		}
	}
	return
}
