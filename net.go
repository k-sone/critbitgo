package critbitgo

import (
	"net"
)

// IP routing table.
type Net struct {
	trie *Trie
}

// Add a route.
func (n *Net) Add(r *net.IPNet, value interface{}) {
	n.trie.Set(netIPNetToKey(r), value)
}

// Add a route.
// If `s` is not CIDR notation, returns an error.
func (n *Net) AddCIDR(s string, value interface{}) (err error) {
	var r *net.IPNet
	if _, r, err = net.ParseCIDR(s); err == nil {
		n.Add(r, value)
	}
	return
}

// Delete a specific route.
// If a route is not found, `ok` is false.
func (n *Net) Delete(r *net.IPNet) (value interface{}, ok bool) {
	return n.trie.Delete(netIPNetToKey(r))
}

// Delete a specific route.
// If `s` is not CIDR notation or a route is not found, `ok` is false.
func (n *Net) DeleteCIDR(s string) (value interface{}, ok bool, err error) {
	var r *net.IPNet
	if _, r, err = net.ParseCIDR(s); err == nil {
		value, ok = n.Delete(r)
	}
	return
}

// Get a specific route.
// If a route is not found, `ok` is false.
func (n *Net) Get(r *net.IPNet) (value interface{}, ok bool) {
	return n.trie.Get(netIPNetToKey(r))
}

// Get a specific route.
// If `s` is not CIDR notation or a route is not found, `ok` is false.
func (n *Net) GetCIDR(s string) (value interface{}, ok bool, err error) {
	var r *net.IPNet
	if _, r, err = net.ParseCIDR(s); err == nil {
		value, ok = n.Get(r)
	}
	return
}

// Return a specific route by using the longest prefix matching.
// If a route is not found, `route` is nil.
func (n *Net) Match(r *net.IPNet) (route *net.IPNet, value interface{}) {
	if k, v := n.match(netIPNetToKey(r)); k != nil {
		route = netKeyToIPNet(k)
		value = v
	}
	return
}

// Return a specific route by using the longest prefix matching.
// If `s` is not CIDR notation, or a route is not found, `route` is nil.
func (n *Net) MatchCIDR(s string) (route *net.IPNet, value interface{}, err error) {
	var r *net.IPNet
	if _, r, err = net.ParseCIDR(s); err == nil {
		route, value = n.Match(r)
	}
	return
}

// Return a specific route by using the longest prefix matching.
// If `ip` is invalid IP, or a route is not found, `route` is nil.
func (n *Net) MatchIP(ip net.IP) (route *net.IPNet, value interface{}, err error) {
	var key []byte
	if v4 := ip.To4(); v4 != nil {
		key = append(v4, 32)
	} else if ip.To16() != nil {
		key = append(ip, 128)
	} else {
		err = &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
		return
	}

	if k, v := n.match(key); k != nil {
		route = netKeyToIPNet(k)
		value = v
	}
	return
}

func (n *Net) match(key []byte) ([]byte, interface{}) {
	if n.trie.size > 0 {
		if node := lookup(&n.trie.root, key, false); node != nil {
			return node.external.key, node.external.value
		}
	}
	return nil, nil
}

func lookup(p *node, key []byte, backtracking bool) *node {
	if p.internal != nil {
		var direction int
		if p.internal.offset == len(key)-1 {
			// selecting the larger side when comparing the mask
			direction = 1
		} else if backtracking {
			direction = 0
		} else {
			direction = p.internal.direction(key)
		}

		if c := lookup(&p.internal.child[direction], key, backtracking); c != nil {
			return c
		}
		if direction == 1 {
			// search other node
			return lookup(&p.internal.child[0], key, true)
		}
		return nil
	} else {
		nlen := len(p.external.key)
		if nlen != len(key) {
			return nil
		}

		// check mask
		mask := p.external.key[nlen-1]
		if mask > key[nlen-1] {
			return nil
		}

		// compare both keys with mask
		div := int(mask >> 3)
		for i := 0; i < div; i++ {
			if p.external.key[i] != key[i] {
				return nil
			}
		}
		if mod := uint(mask & 0x07); mod > 0 {
			bit := 8 - mod
			if p.external.key[div] != key[div]&(0xff>>bit<<bit) {
				return nil
			}
		}
		return p
	}
}

// Deletes all routes.
func (n *Net) Clear() {
	n.trie.Clear()
}

// Returns number of routes.
func (n *Net) Size() int {
	return n.trie.Size()
}

// Create IP routing table
func NewNet() *Net {
	return &Net{NewTrie()}
}

func netIPNetToKey(n *net.IPNet) []byte {
	// +--------------+------+
	// | ip address.. | mask |
	// +--------------+------+
	ones, _ := n.Mask.Size()
	return append(n.IP, byte(ones))
}

func netKeyToIPNet(k []byte) *net.IPNet {
	iplen := len(k) - 1
	return &net.IPNet{
		IP:   net.IP(k[:iplen]),
		Mask: net.CIDRMask(int(k[iplen]), iplen*8),
	}
}
