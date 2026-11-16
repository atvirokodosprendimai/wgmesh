package node

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

// The fixture is frozen: recordings of claims older releases emit, with the
// decision every phase must give. It is never regenerated.
const compatFixture = "../../testdata/compat/admit/v1/cases.json"

type compatMember struct {
	Key    string   `json:"key"`
	IP     string   `json:"ip"`
	Routes []string `json:"routes"`
}

type compatExpect struct {
	Verdict     string   `json:"verdict"`
	Reason      string   `json:"reason"`
	Stored      bool     `json:"stored"`
	Routes      []string `json:"routes"`
	Warnings    []string `json:"warnings"`
	StoredAfter []string `json:"stored_after"`
}

type compatCase struct {
	Name         string                  `json:"name"`
	Subnet       string                  `json:"subnet"`
	NeverRefused bool                    `json:"never_refused"`
	Hijack       bool                    `json:"hijack"`
	Store        []compatMember          `json:"store"`
	Offer        compatMember            `json:"offer"`
	Expect       map[string]compatExpect `json:"expect"`
}

type compatFile struct {
	Subnets map[string]struct {
		MeshNet string `json:"mesh_net"`
	} `json:"subnets"`
	Keys  map[string]map[string][]string `json:"keys"`
	Cases []compatCase                   `json:"cases"`
}

func verdictName(v Verdict) string {
	return map[Verdict]string{Admit: "admit", AdmitPartial: "admit-partial", Refuse: "refuse"}[v]
}

// sameList compares two lists, a nil list and an empty one being equal.
func sameList(a, b []string) bool {
	return (len(a) == 0 && len(b) == 0) || reflect.DeepEqual(a, b)
}

// playCompat builds the case's store and offers its candidate; the warnings
// are those of the offer alone.
func playCompat(t *testing.T, c compatCase, ps *PeerStore, warned *[]string) (Decision, *PeerInfo) {
	t.Helper()
	*warned = nil
	d := ps.Offer(Candidate{
		Info:   &PeerInfo{WGPubKey: c.Offer.Key, MeshIP: c.Offer.IP, RoutableNetworks: append([]string(nil), c.Offer.Routes...)},
		Source: "dht", Hand: FirstHand,
	})
	got, _ := ps.Get(c.Offer.Key)
	return d, got
}

func compatStore(t *testing.T, f compatFile, c compatCase) (*PeerStore, *[]string) {
	t.Helper()
	warned := new([]string)
	ps := NewPeerStore()
	ps.SetPolicy(&Policy{
		MeshNet: mustCIDR(t, f.Subnets[c.Subnet].MeshNet),
		Derive: func(key string, nonce int) string {
			if ips := f.Keys[key][c.Subnet]; nonce < len(ips) {
				return ips[nonce]
			}
			return ""
		},
		OnWarn: func(rule string) { *warned = append(*warned, rule) },
	})
	for _, m := range c.Store {
		ps.SetPeerDirectly(m.Key, &PeerInfo{WGPubKey: m.Key, MeshIP: m.IP, RoutableNetworks: m.Routes})
	}
	return ps, warned
}

func TestAdmitCompatFixtures(t *testing.T) {
	raw, err := os.ReadFile(compatFixture)
	if err != nil {
		t.Fatal(err)
	}
	var f compatFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) < 8 {
		t.Fatalf("fixture holds %d cases, want at least 8", len(f.Cases))
	}
	phases := []struct {
		name      string
		ip, route bool
		then      bool // after the first offer, flip the route switch and offer again
	}{
		{"warn", false, false, false},
		{"ip", true, false, false},
		{"both", true, true, false},
		{"ip-then-both", true, false, true},
	}
	for _, c := range f.Cases {
		for _, ph := range phases {
			t.Run(c.Name+"/"+ph.name, func(t *testing.T) {
				want, ok := c.Expect[ph.name]
				if !ok {
					t.Fatalf("no recorded decision for phase %s", ph.name)
				}
				setSwitches(t, ph.ip, ph.route)
				ps, warned := compatStore(t, f, c)
				d, got := playCompat(t, c, ps, warned)
				if ph.then {
					EnforceRouteBinding.Store(true)
					d, got = playCompat(t, c, ps, warned)
				}

				if v := verdictName(d.Verdict); v != want.Verdict || string(d.Reason) != want.Reason {
					t.Errorf("decision = %s %q, want %s %q", v, d.Reason, want.Verdict, want.Reason)
				}
				if (got != nil) != want.Stored {
					t.Fatalf("offered key stored = %v, want %v", got != nil, want.Stored)
				}
				if got != nil && !sameList(got.RoutableNetworks, want.Routes) {
					t.Errorf("stored routes = %v, want %v", got.RoutableNetworks, want.Routes)
				}
				if !sameList(*warned, want.Warnings) {
					t.Errorf("warnings = %v, want %v", *warned, want.Warnings)
				}
				var keys []string
				for _, p := range ps.GetAll() {
					keys = append(keys, p.WGPubKey)
				}
				sort.Strings(keys)
				if !reflect.DeepEqual(keys, want.StoredAfter) {
					t.Errorf("store holds %v, want %v", keys, want.StoredAfter)
				}

				if c.NeverRefused && d.Verdict == Refuse {
					t.Errorf("%s was refused in phase %s; rotated, nonce and custom-subnet claims are admitted in every phase", c.Name, ph.name)
				}
				if c.Hijack && ph.ip && d.Verdict != Refuse {
					t.Errorf("the hijack was admitted with the IP switch on: %+v", d)
				}
				// Every stored member keeps being admitted when it speaks again.
				for _, m := range c.Store {
					if r := ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: m.Key, MeshIP: m.IP}, Source: "dht", Hand: FirstHand}); r.Verdict == Refuse {
						t.Errorf("stored member %s refused on its next announcement: %+v", m.Key, r)
					}
				}
			})
		}
	}
}
