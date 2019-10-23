package critbitgo_test

import (
	"net"
	"reflect"
	"testing"

	"github.com/k-sone/critbitgo"
)

func TestNet(t *testing.T) {
	trie := critbitgo.NewNet()
	cidr := "192.168.1.0/24"
	host := "192.168.1.1/32"
	hostIP := net.IPv4(192, 168, 1, 1)

	if _, _, err := trie.GetCIDR(""); err == nil {
		t.Error("GetCIDR() - not error")
	}
	if v, ok, err := trie.GetCIDR(cidr); v != nil || ok || err != nil {
		t.Errorf("GetCIDR() - phantom: %v, %v, %v", v, ok, err)
	}
	if _, _, err := trie.MatchCIDR(""); err == nil {
		t.Error("MatchCIDR() - not error")
	}
	if r, v, err := trie.MatchCIDR(host); r != nil || v != nil || err != nil {
		t.Errorf("MatchCIDR() - phantom: %v, %v, %v", r, v, err)
	}
	if _, _, err := trie.MatchIP(net.IP([]byte{})); err == nil {
		t.Error("MatchIP() - not error")
	}
	if r, v, err := trie.MatchIP(hostIP); r != nil || v != nil || err != nil {
		t.Errorf("MatchIP() - phantom: %v, %v, %v", r, v, err)
	}
	if _, err := trie.ContainedIP(net.IP([]byte{})); err == nil {
		t.Error("ContainedIP() - not error")
	}
	if b, err := trie.ContainedIP(hostIP); b || err != nil {
		t.Errorf("ContainedIP() - phantom: %v, %v", b, err)
	}
	if _, _, err := trie.DeleteCIDR(""); err == nil {
		t.Error("DeleteCIDR() - not error")
	}
	if v, ok, err := trie.DeleteCIDR(cidr); v != nil || ok || err != nil {
		t.Errorf("DeleteCIDR() - phantom: %v, %v, %v", v, ok, err)
	}

	if err := trie.AddCIDR(cidr, &cidr); err != nil {
		t.Errorf("AddCIDR() - %s: error occurred %s", cidr, err)
	}
	if v, ok, err := trie.GetCIDR(cidr); v != &cidr || !ok || err != nil {
		t.Errorf("GetCIDR() - failed: %v, %v, %v", v, ok, err)
	}
	if r, v, err := trie.MatchCIDR(host); r == nil || r.String() != cidr || v != &cidr || err != nil {
		t.Errorf("MatchCIDR() - failed: %v, %v, %v", r, v, err)
	}
	if r, v, err := trie.MatchIP(hostIP); r == nil || r.String() != cidr || v != &cidr || err != nil {
		t.Errorf("MatchIP() - failed: %v, %v, %v", r, v, err)
	}
	if b, err := trie.ContainedIP(hostIP); !b || err != nil {
		t.Errorf("ContainedIP() - failed: %v, %v", b, err)
	}
	if v, ok, err := trie.DeleteCIDR(cidr); v != &cidr || !ok || err != nil {
		t.Errorf("DeleteCIDR() - failed: %v, %v, %v", v, ok, err)
	}
}

func checkMatch(t *testing.T, trie *critbitgo.Net, request, expect string) {
	route, value, err := trie.MatchCIDR(request)
	if err != nil {
		t.Errorf("MatchCIDR() - %s: error occurred %s", request, err)
	}
	if cidr := route.String(); expect != cidr {
		t.Errorf("MatchCIDR() - %s: expected [%s], actual [%s]", request, expect, cidr)
	}
	if value == nil {
		t.Errorf("MatchCIDR() - %s: no value", request)
	}
}

func buildTestNet(t *testing.T) *critbitgo.Net {
	trie := critbitgo.NewNet()

	cidrs := []string{
		"10.0.0.0/8",
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

	for i, cidr := range cidrs {
		if err := trie.AddCIDR(cidr, cidrs[i]); err != nil {
			t.Errorf("AddCIDR() - %s: error occurred %s", cidr, err)
		}
	}
	return trie
}

func TestNetMatch(t *testing.T) {
	trie := buildTestNet(t)

	checkMatch(t, trie, "10.0.0.0/24", "10.0.0.0/8")
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

func TestNetWalk(t *testing.T) {
	trie := buildTestNet(t)

	var c int
	f := func(n *net.IPNet, v interface{}) bool {
		c += 1
		return true
	}

	c = 0
	trie.Walk(nil, f)
	if c != 11 {
		t.Errorf("Walk() - %d: full walk", c)
	}

	_, s, _ := net.ParseCIDR("192.168.1.1/32")
	c = 0
	trie.Walk(s, f)
	if c != 6 {
		t.Errorf("Walk() - %d: has start route", c)
	}

	_, s, _ = net.ParseCIDR("10.0.0.0/0")
	c = 0
	trie.Walk(s, f)
	if c != 0 {
		t.Errorf("Walk() - %d: not found start route", c)
	}
}

func TestNetWalkPrefix(t *testing.T) {
	trie := buildTestNet(t)

	var ret, exp []string
	f := func(n *net.IPNet, _ interface{}) bool {
		ret = append(ret, n.String())
		return true
	}

	ret = []string{}
	exp = []string{"192.168.0.0/16"}
	_, s, _ := net.ParseCIDR("192.168.0.0/24")
	trie.WalkPrefix(s, f)
	if !reflect.DeepEqual(ret, exp) {
		t.Errorf("WalkPrefix() - failed %s", ret)
	}

	ret = []string{}
	exp = []string{
		"192.168.0.0/16", "192.168.1.0/24", "192.168.1.0/28", "192.168.1.0/32",
		"192.168.1.1/32", "192.168.1.2/32", "192.168.1.32/27", "192.168.1.32/30",
	}
	_, s, _ = net.ParseCIDR("192.168.0.0/23")
	trie.WalkPrefix(s, f)
	if !reflect.DeepEqual(ret, exp) {
		t.Errorf("WalkPrefix() - failed %s", ret)
	}

	ret = []string{}
	exp = []string{}
	_, s, _ = net.ParseCIDR("0.0.0.0/16")
	trie.WalkPrefix(s, f)
	if !reflect.DeepEqual(ret, exp) {
		t.Errorf("WalkPrefix() - failed %s", ret)
	}
}

func TestNetWalkMatch(t *testing.T) {
	trie := buildTestNet(t)

	var ret, exp []string
	f := func(n *net.IPNet, _ interface{}) bool {
		ret = append(ret, n.String())
		return true
	}

	ret = []string{}
	exp = []string{"192.168.0.0/16", "192.168.1.0/24"}
	_, s, _ := net.ParseCIDR("192.168.1.0/27")
	trie.WalkMatch(s, f)
	if !reflect.DeepEqual(ret, exp) {
		t.Errorf("WalkMatch() - failed %s", ret)
	}

	ret = []string{}
	exp = []string{
		"192.168.0.0/16", "192.168.1.0/24", "192.168.1.0/28", "192.168.1.1/32",
	}
	_, s, _ = net.ParseCIDR("192.168.1.1/32")
	trie.WalkMatch(s, f)
	if !reflect.DeepEqual(ret, exp) {
		t.Errorf("WalkMatch() - failed %s", ret)
	}

	ret = []string{}
	exp = []string{"10.0.0.0/8"}
	_, s, _ = net.ParseCIDR("10.0.64.0/18")
	trie.WalkMatch(s, f)
	if !reflect.DeepEqual(ret, exp) {
		t.Errorf("WalkMatch() - failed %s", ret)
	}

	ret = []string{}
	exp = []string{}
	_, s, _ = net.ParseCIDR("255.255.255.0/24")
	trie.WalkMatch(s, f)
	if !reflect.DeepEqual(ret, exp) {
		t.Errorf("WalkMatch() - failed %s", ret)
	}
}
