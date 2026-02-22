# Tamarin Prover — wgmesh formal verification
#
# Install everything:
#   make install
#
# Prove all lemmas (batch mode):
#   make prove
#
# Interactive GUI (browser at http://localhost:3001):
#   make interactive
#
# Parse/syntax check only:
#   make check

TAMARIN := tamarin-prover
MODEL   := wgmesh.spthy

INSTALL_BIN := /usr/local/bin

TAMARIN_VERSION := 1.10.0
TAMARIN_URL     := https://github.com/tamarin-prover/tamarin-prover/releases/download/$(TAMARIN_VERSION)/tamarin-prover-$(TAMARIN_VERSION).arm64_sonoma.bottle.tar.gz

MAUDE_VERSION := 3.5.1
MAUDE_URL     := https://github.com/maude-lang/Maude/releases/download/Maude$(MAUDE_VERSION)/Maude-$(MAUDE_VERSION)-macos-arm64.zip

.PHONY: install install-tamarin install-maude install-graphviz prove interactive check

install: install-maude install-tamarin install-graphviz
	@echo ""
	@echo "All installed:"
	@$(TAMARIN) --version
	@maude --version 2>&1 | head -1

# ── graphviz ─────────────────────────────────────────────────────────────────

install-graphviz:
	brew install graphviz

# ── maude ────────────────────────────────────────────────────────────────────
# Prebuilt arm64 binary from github.com/maude-lang/Maude

install-maude:
	@echo "Installing Maude $(MAUDE_VERSION) (arm64)..."
	curl -fsSL -o /tmp/maude.zip "$(MAUDE_URL)"
	cd /tmp && unzip -o maude.zip maude
	install -m 755 /tmp/maude "$(INSTALL_BIN)/maude"
	@echo "Maude installed: $$(maude --version 2>&1 | head -1)"

# ── tamarin-prover ───────────────────────────────────────────────────────────
# Homebrew bottle (arm64_sonoma) from github.com/tamarin-prover/tamarin-prover

install-tamarin:
	@echo "Installing tamarin-prover $(TAMARIN_VERSION) (arm64)..."
	curl -fsSL -o /tmp/tamarin.tar.gz "$(TAMARIN_URL)"
	cd /tmp && tar xzf tamarin.tar.gz tamarin-prover/$(TAMARIN_VERSION)/bin/tamarin-prover
	install -m 755 /tmp/tamarin-prover/$(TAMARIN_VERSION)/bin/tamarin-prover "$(INSTALL_BIN)/tamarin-prover"
	@echo "Tamarin installed: $$($(TAMARIN) --version)"

# ── verification ─────────────────────────────────────────────────────────────

prove:
	$(TAMARIN) --prove $(MODEL) 2>&1 | tee prove.log
	@echo ""
	@grep -E 'verified|falsified|analysis' prove.log || true

interactive:
	$(TAMARIN) interactive $(MODEL)

check:
	$(TAMARIN) $(MODEL)
