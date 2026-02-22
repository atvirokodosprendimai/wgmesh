# Tamarin Prover — wgmesh formal verification
#
# Platform: macOS arm64 (Apple Silicon) only.
# Binaries are pinned to arm64_sonoma (Tamarin) and macos-arm64 (Maude).
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
TAMARIN_SHA256  := 68547fe682b5bc86c8549c1be8585814f2d503193ea9d297f584181d64070385

MAUDE_VERSION := 3.5.1
MAUDE_URL     := https://github.com/maude-lang/Maude/releases/download/Maude$(MAUDE_VERSION)/Maude-$(MAUDE_VERSION)-macos-arm64.zip
MAUDE_SHA256  := 95851274f57b3853aab833674e2b770ed800f38fb1f3d03c97dcac56346c13dc

.PHONY: install install-tamarin install-maude install-graphviz prove interactive check

install: _check-platform install-maude install-tamarin install-graphviz
	@echo ""
	@echo "All installed:"
	@$(TAMARIN) --version
	@maude --version 2>&1 | head -1

# ── platform guard ────────────────────────────────────────────────────────────

_check-platform:
	@if [ "$$(uname -s)" != "Darwin" ] || [ "$$(uname -m)" != "arm64" ]; then \
		echo "ERROR: install targets require macOS arm64 (Apple Silicon)."; \
		echo "       Current: $$(uname -s)/$$(uname -m)"; \
		exit 1; \
	fi

# ── graphviz ─────────────────────────────────────────────────────────────────

install-graphviz:
	brew install graphviz

# ── maude ────────────────────────────────────────────────────────────────────
# Prebuilt macos-arm64 binary from github.com/maude-lang/Maude

install-maude:
	@echo "Installing Maude $(MAUDE_VERSION) (macos-arm64)..."
	curl -fsSL -o /tmp/maude.zip "$(MAUDE_URL)"
	@echo "$(MAUDE_SHA256)  /tmp/maude.zip" | shasum -a 256 -c -
	cd /tmp && unzip -o maude.zip maude
	install -m 755 /tmp/maude "$(INSTALL_BIN)/maude"
	@echo "Maude installed: $$(maude --version 2>&1 | head -1)"

# ── tamarin-prover ───────────────────────────────────────────────────────────
# Homebrew bottle (arm64_sonoma) from github.com/tamarin-prover/tamarin-prover

install-tamarin:
	@echo "Installing tamarin-prover $(TAMARIN_VERSION) (arm64_sonoma)..."
	curl -fsSL -o /tmp/tamarin.tar.gz "$(TAMARIN_URL)"
	@echo "$(TAMARIN_SHA256)  /tmp/tamarin.tar.gz" | shasum -a 256 -c -
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
