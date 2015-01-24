package critbitgo

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
)

var (
	msbOnce   sync.Once
	msbMatrix [256]byte // The matrix of most significant bit

	KeyExists   error = errors.New("A key already exists")
	KeyTailNull error = errors.New("Can't use a key with the NULL termination")
)

func buildMsbMatrix() {
	for i := 0; i < len(msbMatrix); i++ {
		b := byte(i)
		b |= b >> 1
		b |= b >> 2
		b |= b >> 4
		msbMatrix[i] = b &^ (b >> 1)
	}
}

type node struct {
	// internal or external
	internal bool

	// internal node attributes
	child  [2]*node
	offset int
	bit    byte

	// external node attributes
	key   []byte
	value interface{}
}

// finding the critical bit.
func (n *node) criticalBit(key []byte) (offset int, bit byte) {
	nlen := len(n.key)
	klen := len(key)
	mlen := nlen
	if nlen > klen {
		mlen = klen
	}

	// find first differing byte and bit
	for offset = 0; offset < mlen; offset++ {
		if a, b := key[offset], n.key[offset]; a != b {
			bit = msbMatrix[a^b]
			return
		}
	}

	if nlen < klen {
		bit = msbMatrix[key[offset]]
	} else if nlen > klen {
		bit = msbMatrix[n.key[offset]]
	} else {
		// two keys are equal
		offset = -1
	}
	return
}

// calculate direction.
func (n *node) direction(key []byte) int {
	if n.offset < len(key) && key[n.offset]&n.bit != 0 {
		return 1
	}
	return 0
}

// Crit-bit Tree
type Trie struct {
	root *node
	size int
}

// searching the tree.
func (t *Trie) search(key []byte) *node {
	n := t.root
	for n.internal {
		n = n.child[n.direction(key)]
	}
	return n
}

// membership testing.
func (t *Trie) Contains(key []byte) bool {
	if t.root == nil {
		return false
	}
	if n := t.search(key); n != nil && bytes.Equal(n.key, key) {
		return true
	}
	return false
}

// get member.
func (t *Trie) Get(key []byte) (value interface{}) {
	if t.root == nil {
		return
	}
	if n := t.search(key); n != nil && bytes.Equal(n.key, key) {
		value = n.value
	}
	return
}

// insert into the tree (replaceable).
func (t *Trie) insert(key []byte, value interface{}, replace bool) error {
	if klen := len(key); klen > 0 && key[klen-1] == 0 {
		return KeyTailNull
	}

	// an empty tree
	if t.root == nil {
		t.root = &node{
			key:   key,
			value: value,
		}
		t.size = 1
		return nil
	}

	n := t.search(key)
	newOffset, newBit := n.criticalBit(key)

	// already exists in the tree
	if newOffset == -1 {
		if replace {
			n.value = value
			return nil
		}
		return KeyExists
	}

	// allocate new node
	newNodes := [2]node{
		{
			internal: true,
			offset:   newOffset,
			bit:      newBit,
		},
		{
			key:   key,
			value: value,
		},
	}
	newNode := &newNodes[0]
	direction := newNode.direction(key)
	newNode.child[direction] = &newNodes[1]

	// insert new node
	var wherep **node = &t.root
	for p := *wherep; p.internal; p = *wherep {
		if p.offset > newOffset || (p.offset == newOffset && p.bit < newBit) {
			break
		}
		wherep = &p.child[p.direction(key)]
	}

	newNode.child[1-direction] = *wherep
	*wherep = newNode
	t.size += 1
	return nil
}

// insert into the tree.
func (t *Trie) Insert(key []byte, value interface{}) error {
	return t.insert(key, value, false)
}

// set into the tree.
func (t *Trie) Set(key []byte, value interface{}) error {
	return t.insert(key, value, true)
}

// deleting elements.
func (t *Trie) Delete(key []byte) bool {
	// an empty tree
	if t.root == nil {
		return false
	}

	var othern *node  // other child of the parent
	var whereq **node // pointer to the grandparent

	// finding the best candidate to delete
	var wherep **node = &t.root
	for p := *wherep; p.internal; p = *wherep {
		direction := p.direction(key)
		whereq = wherep
		wherep = &p.child[direction]
		othern = p.child[1-direction]
	}

	// checking that we have the right element
	if !bytes.Equal((*wherep).key, key) {
		return false
	}

	// removing the node
	if whereq == nil {
		t.root = nil
	} else {
		*whereq = othern
	}
	t.size -= 1
	return true
}

// clearing a tree.
func (t *Trie) Clear() {
	t.root = nil
	t.size = 0
}

// return the number of key in a tree.
func (t *Trie) Size() int {
	return t.size
}

// fetching elements with a given prefix.
// handle is called with arguments key and value (if handle returns `false`, the iteration is aborted)
func (t *Trie) Allprefixed(prefix []byte, handle func(key []byte, value interface{}) bool) bool {
	// an empty tree
	if t.root == nil {
		return true
	}

	// walk tree, maintaining top pointer
	p := t.root
	top := p
	for p.internal {
		top = p
		p = p.child[p.direction(prefix)]
	}

	// check prefix
	if !bytes.Contains(p.key, prefix) {
		return true
	}

	return allprefixed(top, handle)
}

func allprefixed(n *node, handle func([]byte, interface{}) bool) bool {
	if n.internal {
		// dealing with an internal node while recursing
		for i := 0; i < 2; i++ {
			if !allprefixed(n.child[i], handle) {
				return false
			}
		}
	} else {
		// dealing with an external node while recursing
		return handle(n.key, n.value)
	}
	return true
}

// dump tree. (for debugging)
func (t *Trie) Dump(w io.Writer) {
	if t.root == nil {
		return
	}
	if w == nil {
		w = os.Stdout
	}
	dump(w, t.root, true, "")
}

func dump(w io.Writer, n *node, right bool, prefix string) {
	var ownprefix string
	if right {
		ownprefix = prefix
	} else {
		ownprefix = prefix[:len(prefix)-1] + "`"
	}

	if n.internal {
		fmt.Fprintf(w, "%s-- off=%d, bit=%08b (%02x)\n", ownprefix, n.offset, n.bit, n.bit)
		for i := 0; i < 2; i++ {
			var nextprefix string
			switch i {
			case 0:
				nextprefix = prefix + " |"
				right = true
			case 1:
				nextprefix = prefix + "  "
				right = false
			}
			dump(w, n.child[i], right, nextprefix)
		}
	} else {
		fmt.Fprintf(w, "%s-- key=%d (%s)\n", ownprefix, n.key, key2str(n.key))
	}
	return
}

func key2str(key []byte) string {
	for _, c := range key {
		if !strconv.IsPrint(rune(c)) {
			return hex.EncodeToString(key)
		}
	}
	return string(key)
}

// create a tree.
func NewTrie() *Trie {
	msbOnce.Do(buildMsbMatrix)
	return &Trie{}
}
