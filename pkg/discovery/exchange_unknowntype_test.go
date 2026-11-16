package discovery

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

// captureLog redirects the standard logger for the test. Tests that call it
// must not be parallel: the logger is process-wide.
func captureLog(t *testing.T) *lockedBuffer {
	t.Helper()
	buf := &lockedBuffer{}
	old := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return buf
}

// A message type the exchange has no handler for is dropped right after the
// envelope opens: no handler call, no nonce cache entry, and the type text is
// not written to the log. Known traffic afterwards behaves as before.
func TestUnknownTypeDroppedEarly(t *testing.T) {
	pe, _, cfg := newAADExchange(t, "wgmesh-test-unknown-type-1", aadKey(9), "")
	buf := captureLog(t)
	var calls atomic.Int32
	pe.SetAnnounceHandler(func(*crypto.PeerAnnouncement, *net.UDPAddr) { calls.Add(1) })

	original, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcementFor(aadKey(2)), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"", "hello", "HELLO ", "UNRECOGNIZED-TYPE", strings.Repeat("x", 1000)} {
		retyped := retype(t, original, typ)
		// The envelope must open at the crypto level, or the drop below
		// would happen for the wrong reason.
		if env, _, err := crypto.OpenEnvelopeRaw(retyped, cfg.Keys.GossipKey); err != nil || env.MessageType != typ {
			t.Fatalf("test setup: retyped envelope %.20q does not open: %v", typ, err)
		}
		pe.handleMessage(retyped, aadRemote)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("handler called %d times for unknown types, want 0", got)
	}
	if got := pe.nonces.Len(); got != 0 {
		t.Errorf("nonce cache holds %d entries after unknown types, want 0", got)
	}
	if out := buf.String(); strings.Contains(out, "UNRECOGNIZED-TYPE") {
		t.Errorf("the unknown type text reached the log:\n%.300s", out)
	}

	pe.handleMessage(original, aadRemote)
	pe.handleMessage(original, aadRemote)
	if got := calls.Load(); got != 1 {
		t.Errorf("the ANNOUNCE reached the handler %d times when delivered twice, want 1", got)
	}
	if got := pe.nonces.Len(); got != 1 {
		t.Errorf("nonce cache holds %d entries, want 1", got)
	}
}

// Every type the exchange knows must still reach its handler: a payload that
// does not parse is reported by that type's case, which proves the dispatch.
func TestKnownTypesStillReachTheirHandlers(t *testing.T) {
	for _, typ := range []string{
		crypto.MessageTypeHello, crypto.MessageTypeReply, crypto.MessageTypeAnnounce,
		crypto.MessageTypeGoodbye, crypto.MessageTypeRendezvousOffer, crypto.MessageTypeRendezvousStart,
	} {
		t.Run(typ, func(t *testing.T) {
			pe, _, cfg := newAADExchange(t, "wgmesh-test-known-types-1", aadKey(9), "")
			buf := captureLog(t)
			sealed, err := crypto.SealEnvelopeAAD(typ, map[string]any{"timestamp": time.Now().Unix(), "protocol": crypto.ProtocolVersion, "wg_pubkey": 5, "pair_id": 5}, cfg.Keys.GossipKey)
			if err != nil {
				t.Fatal(err)
			}
			pe.handleMessage(sealed, aadRemote)
			out := buf.String()
			if want := "Invalid " + typ; !strings.Contains(out, want) {
				t.Errorf("%s did not reach its handler, log has no %q:\n%.300s", typ, want, out)
			}
			if strings.Contains(out, "unknown type") {
				t.Errorf("%s was dropped as an unknown type", typ)
			}
		})
	}
}

// The helper that gates the dispatch and the dispatch itself are two lists.
// Read both from the source and require that each equals the six known types,
// so a case added to handleMessage cannot be silently dropped as an unknown
// type, and a type added to the helper alone is not let through undispatched.
func TestExchangeMessageTypesMatchTheDispatch(t *testing.T) {
	t.Parallel()
	byName := map[string]string{
		"MessageTypeHello":           crypto.MessageTypeHello,
		"MessageTypeReply":           crypto.MessageTypeReply,
		"MessageTypeAnnounce":        crypto.MessageTypeAnnounce,
		"MessageTypeGoodbye":         crypto.MessageTypeGoodbye,
		"MessageTypeRendezvousOffer": crypto.MessageTypeRendezvousOffer,
		"MessageTypeRendezvousStart": crypto.MessageTypeRendezvousStart,
	}
	file, err := parser.ParseFile(token.NewFileSet(), "exchange.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	// caseNames returns the constant names in the cases of the one switch in
	// fn whose tag is named tag (an identifier or a selector).
	caseNames := func(fn, tag string) map[string]bool {
		names := map[string]bool{}
		for _, decl := range file.Decls {
			d, ok := decl.(*ast.FuncDecl)
			if !ok || d.Name.Name != fn {
				continue
			}
			ast.Inspect(d.Body, func(n ast.Node) bool {
				sw, ok := n.(*ast.SwitchStmt)
				if !ok {
					return true
				}
				name := ""
				switch x := sw.Tag.(type) {
				case *ast.Ident:
					name = x.Name
				case *ast.SelectorExpr:
					name = x.Sel.Name
				}
				if name != tag {
					return true
				}
				for _, c := range sw.Body.List {
					for _, e := range c.(*ast.CaseClause).List {
						if s, ok := e.(*ast.SelectorExpr); ok {
							names[s.Sel.Name] = true
						} else {
							t.Errorf("%s: a case that is not a crypto.MessageType* constant; the guard cannot read it", fn)
						}
					}
				}
				return false
			})
		}
		if len(names) == 0 {
			t.Fatalf("found no case in the %s switch on %s; the guard is blind", fn, tag)
		}
		return names
	}
	for fn, names := range map[string]map[string]bool{
		"handleMessage":         caseNames("handleMessage", "MessageType"),
		"isExchangeMessageType": caseNames("isExchangeMessageType", "messageType"),
	} {
		for name := range names {
			if _, known := byName[name]; !known {
				t.Errorf("%s has a case for %s that this test does not know: add it here, to handleMessage and to isExchangeMessageType together", fn, name)
			}
		}
		for name := range byName {
			if !names[name] {
				t.Errorf("%s has no case for %s: keep handleMessage, isExchangeMessageType and this test in step", fn, name)
			}
		}
	}
	for name, value := range byName {
		if !isExchangeMessageType(value) {
			t.Errorf("isExchangeMessageType drops %s", name)
		}
	}
	for _, other := range []string{"", "hello", "HELLO ", "UNRECOGNIZED-TYPE"} {
		if isExchangeMessageType(other) {
			t.Errorf("isExchangeMessageType(%.20q) = true", other)
		}
	}
}
