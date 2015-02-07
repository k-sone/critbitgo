package critbitgo_test

import (
	"testing"

	"github.com/k-sone/critbitgo"
)

func checkMatch(t *testing.T, trie *critbitgo.Net, request, expect string) {
	cidr, value, err := trie.MatchCIDR(request)
	if err != nil {
		t.Errorf("MatchCIDR() - %s: error occurred %s", request, err)
	}
	if expect != cidr {
		t.Errorf("MatchCIDR() - %s: expected [%s], actual [%s]", request, expect, cidr)
	}
	if value == nil {
		t.Errorf("MatchCIDR() - %s: no value", request)
	}
}

func TestNetMatch(t *testing.T) {
	trie := critbitgo.NewNet()

	cidrs := []string{
		"0.0.0.0/4",
		"192.168.0.0/16",
		"192.168.1.0/24",
		"192.168.1.0/28",
		"192.168.1.0/32",
		"192.168.1.1/32",
		"192.168.1.2/32",
		"192.168.1.32/27",
		"192.168.1.32/30",
		"192.168.2.1/32",
		"192.168.2.2/32",
	}

	for _, cidr := range cidrs {
		if err := trie.AddCIDR(cidr, &cidr); err != nil {
			t.Errorf("AddCIDR() - %s: error occurred %s", cidr, err)
		}
	}

	checkMatch(t, trie, "10.0.0.0/8", "0.0.0.0/4")
	checkMatch(t, trie, "192.168.1.0/24", "192.168.1.0/24")
	checkMatch(t, trie, "192.168.1.0/30", "192.168.1.0/28")
	checkMatch(t, trie, "192.168.1.0/32", "192.168.1.0/32")
	checkMatch(t, trie, "192.168.1.128/26", "192.168.1.0/24")
	checkMatch(t, trie, "192.168.2.128/26", "192.168.0.0/16")
	checkMatch(t, trie, "192.168.1.1/32", "192.168.1.1/32")
	checkMatch(t, trie, "192.168.1.2/32", "192.168.1.2/32")
	checkMatch(t, trie, "192.168.1.3/32", "192.168.1.0/28")
	checkMatch(t, trie, "192.168.1.32/32", "192.168.1.32/30")
	checkMatch(t, trie, "192.168.1.35/32", "192.168.1.32/30")
	checkMatch(t, trie, "192.168.1.36/32", "192.168.1.32/27")
	checkMatch(t, trie, "192.168.1.63/32", "192.168.1.32/27")
	checkMatch(t, trie, "192.168.1.64/32", "192.168.1.0/24")
	checkMatch(t, trie, "192.168.2.2/32", "192.168.2.2/32")
	checkMatch(t, trie, "192.168.2.3/32", "192.168.0.0/16")
}
