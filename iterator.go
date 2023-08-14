package iavl

// NOTE: This file favors int64 as opposed to int for size/counts.
// The Tree on the other hand favors int.  This is intentional.

import (
	"bytes"
	"errors"
	"fmt"

	dbm "github.com/cosmos/cosmos-db"
)

type PathIterator interface {
	dbm.Iterator

	// Path returns the path of this node within the tree, as a bit string and length in bits.
	Path() []bool
}

type traversal struct {
	tree         *ImmutableTree
	start, end   []byte        // iteration domain
	ascending    bool          // ascending traversal
	inclusive    bool          // end key inclusiveness
	post         bool          // postorder traversal
	delayedNodes *delayedNodes // delayed nodes to be traversed
}

var errIteratorNilTreeGiven = errors.New("iterator must be created with an immutable tree but the tree was nil")

func (node *Node) newTraversal(tree *ImmutableTree, start, end []byte, ascending bool, inclusive bool, post bool) *traversal {
	return &traversal{
		tree:         tree,
		start:        start,
		end:          end,
		ascending:    ascending,
		inclusive:    inclusive,
		post:         post,
		delayedNodes: &delayedNodes{{node, true, nil}}, // set initial traverse to the node
	}
}

type nodePath []bool

// delayedNode represents the delayed iteration on the nodes.
// When delayed is set to true, the delayedNode should be expanded, and their
// children should be traversed. When delayed is set to false, the delayedNode is
// already have expanded, and it could be immediately returned.
type delayedNode struct {
	node    *Node
	delayed bool
	path    nodePath
}

type delayedNodes []delayedNode

func (nodes *delayedNodes) pop() delayedNode {
	node := (*nodes)[len(*nodes)-1]
	*nodes = (*nodes)[:len(*nodes)-1]
	return node
}

func (nodes *delayedNodes) push(node delayedNode) {
	*nodes = append(*nodes, node)
}

func (nodes *delayedNodes) length() int {
	return len(*nodes)
}

func copyPath(p nodePath) (ret nodePath) {
	ret = make(nodePath, len(p))
	copy(ret, p)
	return ret
}

// `traversal` returns the delayed execution of recursive traversal on a tree.
//
// `traversal` will traverse the tree in a depth-first manner. To handle locating
// the next element, and to handle unwinding, the traversal maintains its future
// iteration under `delayedNodes`. At each call of `next()`, it will retrieve the
// next element from the `delayedNodes` and acts accordingly. The `next()` itself
// defines how to unwind the delayed nodes stack. The caller can either call the
// next traversal to proceed, or simply discard the `traversal` struct to stop iteration.
//
// At the each step of `next`, the `delayedNodes` can have one of the three states:
// 1. It has length of 0, meaning that their is no more traversable nodes.
// 2. It has length of 1, meaning that the traverse is being started from the initial node.
// 3. It has length of 2>=, meaning that there are delayed nodes to be traversed.
//
// When the `delayedNodes` are not empty, `next` retrieves the first `delayedNode` and initially check:
// 1. If it is not an delayed node (node.delayed == false) it immediately returns it.
//
// A. If the `node` is a branch node:
//  1. If the traversal is postorder, then append the current node to the t.delayedNodes,
//     with `delayed` set to false. This makes the current node returned *after* all the children
//     are traversed, without being expanded.
//  2. Append the traversable children nodes into the `delayedNodes`, with `delayed` set to true. This
//     makes the children nodes to be traversed, and expanded with their respective children.
//  3. If the traversal is preorder, (with the children to be traversed already pushed to the
//     `delayedNodes`), returns the current node.
//  4. Call `traversal.next()` to further traverse through the `delayedNodes`.
//
// B. If the `node` is a leaf node, it will be returned without expand, by the following process:
//  1. If the traversal is postorder, the current node will be append to the `delayedNodes` with `delayed`
//     set to false, and immediately returned at the subsequent call of `traversal.next()` at the last line.
//  2. If the traversal is preorder, the current node will be returned.
func (t *traversal) next() (node *Node, path []bool, err error) {
	// End of traversal.
	if t.delayedNodes.length() == 0 {
		return
	}

	d := t.delayedNodes.pop()
	node, delayed, path := d.node, d.delayed, d.path

	// Already expanded, immediately return.
	if !delayed || node == nil {
		return
	}

	afterStart := t.start == nil || bytes.Compare(t.start, node.key) < 0
	startOrAfter := afterStart || bytes.Equal(t.start, node.key)
	beforeEnd := t.end == nil || bytes.Compare(node.key, t.end) < 0
	if t.inclusive {
		beforeEnd = beforeEnd || bytes.Equal(node.key, t.end)
	}

	// case of postorder. A-1 and B-1
	// Recursively process left sub-tree, then right-subtree, then node itself.
	if t.post && (!node.isLeaf() || (startOrAfter && beforeEnd)) {
		t.delayedNodes.push(delayedNode{node, false, d.path})
	}

	// case of branch node, traversing children. A-2.
	if !node.isLeaf() {
		// if node is a branch node and the order is ascending,
		// We traverse through the left subtree, then the right subtree.
		// if node is a branch node and the order is not ascending
		// We traverse through the right subtree, then the left subtree.
		for i := 0; i < 2; i++ {
			childI := i
			if t.ascending {
				childI = 1 - childI
			}
			if childI == 0 && afterStart {
				// push the delayed traversal for the left nodes,
				leftNode, err := node.getLeftNode(t.tree)
				if err != nil {
					return nil, nil, err
				}
				t.delayedNodes.push(delayedNode{leftNode, true, append(copyPath(d.path), false)})
			} else if childI == 1 && beforeEnd {
				// push the delayed traversal for the right nodes,
				rightNode, err := node.getRightNode(t.tree)
				if err != nil {
					return nil, nil, err
				}
				t.delayedNodes.push(delayedNode{rightNode, true, append(copyPath(d.path), true)})
			}
		}
	}

	// case of preorder traversal. A-3 and B-2.
	// Process root then (recursively) processing left child, then process right child
	if !t.post && (!node.isLeaf() || (startOrAfter && beforeEnd)) {
		return
	}

	// Keep traversing and expanding the remaning delayed nodes. A-4.
	return t.next()
}

// Iterator is a dbm.Iterator for ImmutableTree
type Iterator struct {
	start, end []byte
	key, value []byte
	path       nodePath
	valid      bool
	err        error
	t          *traversal
}

var _ dbm.Iterator = (*Iterator)(nil)

// Returns a new iterator over the immutable tree. If the tree is nil, the iterator will be invalid.
func NewIterator(start, end []byte, ascending bool, tree *ImmutableTree) *Iterator {
	iter := &Iterator{
		start: start,
		end:   end,
	}

	if tree == nil {
		iter.err = errIteratorNilTreeGiven
	} else {
		iter.valid = true
		iter.t = tree.root.newTraversal(tree, start, end, ascending, false, false)
		// Move iterator before the first element
		iter.Next()
	}
	return iter
}

// Domain implements dbm.Iterator.
func (iter *Iterator) Domain() ([]byte, []byte) {
	return iter.start, iter.end
}

// Valid implements dbm.Iterator.
func (iter *Iterator) Valid() bool {
	return iter.valid
}

// Key implements dbm.Iterator
func (iter *Iterator) Key() []byte {
	return iter.key
}

// Path implements PathIterator
func (iter *Iterator) Path() []bool {
	return iter.path
}

// Value implements dbm.Iterator
func (iter *Iterator) Value() []byte {
	return iter.value
}

// Next implements dbm.Iterator
func (iter *Iterator) Next() {
	if iter.t == nil {
		return
	}

	node, path, err := iter.t.next()
	// TODO: double-check if this error is correctly handled.
	if node == nil || err != nil {
		iter.t = nil
		iter.valid = false
		return
	}

	if node.subtreeHeight == 0 {
		iter.key, iter.value = node.key, node.value
		iter.path = path
		return
	}

	iter.Next()
}

// Close implements dbm.Iterator
func (iter *Iterator) Close() error {
	iter.t = nil
	iter.valid = false
	return iter.err
}

// Error implements dbm.Iterator
func (iter *Iterator) Error() error {
	return iter.err
}

// IsFast returnts true if iterator uses fast strategy
func (iter *Iterator) IsFast() bool {
	return false
}

var _ PathIterator = (*Iterator)(nil)

func NewPathIterator(start, end []byte, ascending bool, tree *ImmutableTree) PathIterator {
	return NewIterator(start, end, ascending, tree)
}

type differenceIterator struct {
	a, b   PathIterator
	yieldA bool
	err    error
}

var _ PathIterator = (*differenceIterator)(nil)
var _ dbm.Iterator = (*differenceIterator)(nil)

// NewDifferenceIterator returns an iterator over the exclusive-or of two iterators, i.e.
// items that differ between the two.
// Items from iterator A are yielded with Value() == nil
// Updates (items with matching Key()) are yielded once, with Value() == B.Value()
func NewDifferenceIterator(a, b PathIterator) dbm.Iterator {
	di := &differenceIterator{a: a, b: b}
	di.seek()
	return di
}

// Domain returns the start (inclusive) and end (exclusive) limits of the iterator.
// CONTRACT: start, end readonly []byte
func (di *differenceIterator) Domain() (start []byte, end []byte) {
	startA, endA := di.a.Domain()
	startB, endB := di.b.Domain()
	// find the maximum domain
	if bytes.Compare(startA, startB) < 0 {
		start = startA
	} else {
		start = startB
	}
	if bytes.Compare(endA, endB) < 0 {
		end = endB
	} else {
		end = endA
	}
	return
}

// Valid returns whether the current iterator is valid. Once invalid, the Iterator remains
// invalid forever.
func (di *differenceIterator) Valid() bool {
	return di.a.Valid() || di.b.Valid()
}

// Next moves the iterator to the next key in the database, as defined by order of iteration.
// If Valid returns false, this method will panic.
func (di *differenceIterator) Next() {
	if di.yieldA {
		di.a.Next()
	} else {
		di.b.Next()
	}
	di.seek()
}

// Maintains the invariant condition of the iterator, ie. both member iterators
// point to an item not in the other set.
func (di *differenceIterator) seek() {
	for {
		if !di.b.Valid() {
			di.yieldA = true
			return
		}
		if !di.a.Valid() {
			di.yieldA = false
			return
		}

		switch bytes.Compare(di.a.Key(), di.b.Key()) {
		case -1: // A < B
			di.yieldA = true
			return
		case 1: // B < A
			di.yieldA = false
			return
		case 0:
			if bytes.Equal(di.a.Value(), di.b.Value()) {
				di.b.Next()
			}
			di.a.Next()
		}
	}
}

func (di *differenceIterator) Path() []bool {
	if di.yieldA {
		return di.a.Path()
	}
	return di.b.Path()
}

// Key returns the key at the current position. Panics if the iterator is invalid.
// CONTRACT: key readonly []byte
func (di *differenceIterator) Key() (key []byte) {
	if di.yieldA {
		return di.a.Key()
	}
	return di.b.Key()
}

// Value returns the value at the current position. Panics if the iterator is invalid.
// If the item is present only in A, this returns nil.
// CONTRACT: value readonly []byte
func (di *differenceIterator) Value() (value []byte) {
	if di.yieldA {
		return nil
	}
	return di.b.Value()
}

// Error returns the last error encountered by the iterator, if any.
func (di *differenceIterator) Error() error {
	return di.err
}

// Close closes the iterator, relasing any allocated resources.
func (di *differenceIterator) Close() error {
	err := di.a.Close()
	if err != nil {
		err = fmt.Errorf("error closing iterator A: %w", err)
	}

	errB := di.b.Close()
	if errB != nil {
		errB = fmt.Errorf("error closing iterator B: %w", errB)
		if err != nil {
			err = fmt.Errorf("%s; %s", err, errB)
		} else {
			err = errB
		}
	}
	di.err = err
	return err
}
