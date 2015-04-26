package critbitgo

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"unsafe"

	"github.com/k-sone/slabgo"
)

var (
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
	internal *internal
	external *external
}

type internal struct {
	child  [2]node
	offset int
	bit    byte
}

type external struct {
	key   []byte
	value interface{}
}

// finding the critical bit.
func (n *external) criticalBit(key []byte) (offset int, bit byte) {
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
func (n *internal) direction(key []byte) int {
	if n.offset < len(key) && key[n.offset]&n.bit != 0 {
		return 1
	}
	return 0
}

// Crit-bit Tree
type Trie struct {
	size int
	root node
	inc  *slabgo.Cache
	exc  *slabgo.Cache
}

// searching the tree.
func (t *Trie) search(key []byte) *node {
	n := &t.root
	for n.internal != nil {
		n = &n.internal.child[n.internal.direction(key)]
	}
	return n
}

// membership testing.
func (t *Trie) Contains(key []byte) bool {
	if n := t.search(key); n.external != nil && bytes.Equal(n.external.key, key) {
		return true
	}
	return false
}

// get member.
func (t *Trie) Get(key []byte) (value interface{}) {
	if n := t.search(key); n.external != nil && bytes.Equal(n.external.key, key) {
		value = n.external.value
	}
	return
}

// insert into the tree (replaceable).
func (t *Trie) insert(key []byte, value interface{}, replace bool) error {
	if klen := len(key); klen > 0 && key[klen-1] == 0 {
		return KeyTailNull
	}

	// an empty tree
	if t.size == 0 {
		t.root.external = newExternal(t, key, value)
		t.size = 1
		return nil
	}

	n := t.search(key)
	newOffset, newBit := n.external.criticalBit(key)

	// already exists in the tree
	if newOffset == -1 {
		if replace {
			n.external.value = value
			return nil
		}
		return KeyExists
	}

	// allocate new node
	newNode := newInternal(t, newOffset, newBit)
	direction := newNode.direction(key)
	newNode.child[direction].external = newExternal(t, key, value)

	// insert new node
	wherep := &t.root
	for in := wherep.internal; in != nil; in = wherep.internal {
		if in.offset > newOffset || (in.offset == newOffset && in.bit < newBit) {
			break
		}
		wherep = &in.child[in.direction(key)]
	}

	if wherep.internal != nil {
		newNode.child[1-direction].internal = wherep.internal
	} else {
		newNode.child[1-direction].external = wherep.external
		wherep.external = nil
	}
	wherep.internal = newNode
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
	if t.size == 0 {
		return false
	}

	var direction int
	var whereq *node // pointer to the grandparent
	var wherep *node = &t.root

	// finding the best candidate to delete
	for in := wherep.internal; in != nil; in = wherep.internal {
		direction = in.direction(key)
		whereq = wherep
		wherep = &in.child[direction]
	}

	// checking that we have the right element
	if !bytes.Equal(wherep.external.key, key) {
		return false
	}

	// removing the node
	if whereq == nil {
		ex := wherep.external
		wherep.external = nil
		freeExternal(t, ex)
	} else {
		in := whereq.internal
		ex := wherep.external
		othern := in.child[1-direction]
		whereq.internal = othern.internal
		whereq.external = othern.external
		freeExternal(t, ex)
		freeInternal(t, in)
	}
	t.size -= 1
	return true
}

// clearing a tree.
func (t *Trie) Clear() {
	t.root.internal = nil
	t.root.external = nil
	t.inc.Destroy()
	t.exc.Destroy()
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
	if t.size == 0 {
		return true
	}

	// walk tree, maintaining top pointer
	p := &t.root
	top := p
	if len(prefix) > 0 {
		for p.internal != nil {
			top = p
			p = &p.internal.child[p.internal.direction(prefix)]
		}

		// check prefix
		if !bytes.Contains(p.external.key, prefix) {
			return true
		}
	}

	return allprefixed(top, handle)
}

func allprefixed(n *node, handle func([]byte, interface{}) bool) bool {
	if n.internal != nil {
		// dealing with an internal node while recursing
		for i := 0; i < 2; i++ {
			if !allprefixed(&n.internal.child[i], handle) {
				return false
			}
		}
	} else {
		// dealing with an external node while recursing
		return handle(n.external.key, n.external.value)
	}
	return true
}

// dump tree. (for debugging)
func (t *Trie) Dump(w io.Writer) {
	if t.root.internal == nil && t.root.external == nil {
		return
	}
	if w == nil {
		w = os.Stdout
	}
	dump(w, &t.root, true, "")
}

func dump(w io.Writer, n *node, right bool, prefix string) {
	var ownprefix string
	if right {
		ownprefix = prefix
	} else {
		ownprefix = prefix[:len(prefix)-1] + "`"
	}

	if in := n.internal; in != nil {
		fmt.Fprintf(w, "%s-- off=%d, bit=%08b (%02x)\n", ownprefix, in.offset, in.bit, in.bit)
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
			dump(w, &in.child[i], right, nextprefix)
		}
	} else {
		fmt.Fprintf(w, "%s-- key=%d (%s)\n", ownprefix, n.external.key, key2str(n.external.key))
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

func newInternal(t *Trie, offset int, bit byte) *internal {
	in := t.inc.Alloc().(*internal)
	in.offset = offset
	in.bit = bit
	return in
}

func newExternal(t *Trie, key []byte, value interface{}) *external {
	ex := t.exc.Alloc().(*external)
	ex.key = key
	ex.value = value
	return ex
}

func freeInternal(t *Trie, in *internal) {
	for _, n := range in.child {
		n.internal = nil
		n.external = nil
	}
	t.inc.FreePtr(unsafe.Pointer(in))
}

func freeExternal(t *Trie, ex *external) {
	ex.key = nil
	ex.value = nil
	t.exc.FreePtr(unsafe.Pointer(ex))
}

// create a tree.
func NewTrie() *Trie {
	return NewTrieWithCapacity(0)
}

// create a tree with the specified initial capacity.
func NewTrieWithCapacity(c int) *Trie {
	var in internal
	var ex external

	m := (c + 255) >> 8
	opts := slabgo.CacheOptions{
		ObjLen: 256,
		Grower: func(s *slabgo.CacheStats) int {
			if m > 0 && m < s.TotalSlabs {
				return m - s.TotalSlabs
			}
			return slabgo.DefaultGrower(s)
		},
		Reaper: func(s *slabgo.CacheStats) int {
			return s.TotalSlabs - s.InuseSlabs - m
		},
	}

	return &Trie{
		inc: slabgo.NewCache(in, opts),
		exc: slabgo.NewCache(ex, opts),
	}
}

func init() {
	buildMsbMatrix()
}
