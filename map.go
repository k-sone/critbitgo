package critbitgo

// The map is sorted according to the natural ordering of its keys
type SortedMap struct {
	trie *Trie
}

func (m *SortedMap) Contains(key string) bool {
	return m.trie.Contains(mapStrToKey(key))
}

func (m *SortedMap) Get(key string) (value interface{}) {
	return m.trie.Get(mapStrToKey(key))
}

func (m *SortedMap) Set(key string, value interface{}) {
	m.trie.Set(mapStrToKey(key), value)
}

func (m *SortedMap) Delete(key string) bool {
	return m.trie.Delete(mapStrToKey(key))
}

func (m *SortedMap) Clear() {
	m.trie.Clear()
}

func (m *SortedMap) Size() int {
	return m.trie.Size()
}

// Returns a slice of sorted keys
func (m *SortedMap) Keys() []string {
	keys := make([]string, 0, m.Size())
	m.trie.Allprefixed([]byte{}, func(k []byte, v interface{}) bool {
		keys = append(keys, mapKeyToStr(k))
		return true
	})
	return keys
}

// Executes a provided function for each element that has a given prefix.
// if handle returns `false`, the iteration is aborted.
func (m *SortedMap) Each(prefix string, handle func(key string, value interface{}) bool) bool {
	return m.trie.Allprefixed([]byte(prefix), func(k []byte, v interface{}) bool {
		return handle(mapKeyToStr(k), v)
	})
}

// Create a SortedMap
func NewSortedMap() *SortedMap {
	return &SortedMap{NewTrie()}
}

func mapStrToKey(s string) []byte {
	// avoid a KeyTailNull
	if l := len(s); l > 0 {
		if t := s[l-1]; t == 0x00 || t == 0xff {
			return append([]byte(s), 0xff)
		}
	}
	return []byte(s)
}

func mapKeyToStr(k []byte) string {
	if l := len(k); l > 0 {
		if t := k[l-1]; t == 0xff {
			return string(k[:l-1])
		}
	}
	return string(k)
}
