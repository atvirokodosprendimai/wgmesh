# Tamarin Prover — wgmesh formal verification
#
# Installation (macOS, arm64):
#   make install
#
# Prove all lemmas:
#   make prove
#
# Interactive GUI (browser):
#   make interactive

TAMARIN := tamarin-prover
MODEL   := wgmesh.spthy

# ── install ──────────────────────────────────────────────────────────────────
#
# Tamarin depends on Maude (term rewriting) and GraphViz (proof graphs).
# The tamarin-prover binary is not in Homebrew; we fetch the latest release
# from GitHub for the current platform.

.PHONY: install install-deps install-tamarin

install: install-deps install-tamarin

install-deps:
	brew install maude graphviz

install-tamarin:
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	ARCH=$$(uname -m); \
	ASSET=$$(curl -fsSL https://api.github.com/repos/tamarin-prover/tamarin-prover/releases/latest \
		| grep '"browser_download_url"' \
		| grep "$$OS" \
		| grep "$$ARCH" \
		| head -1 \
		| sed 's/.*"\(https[^"]*\)".*/\1/'); \
	if [ -z "$$ASSET" ]; then \
		echo "No prebuilt binary found for $$OS/$$ARCH."; \
		echo "See https://github.com/tamarin-prover/tamarin-prover/releases"; \
		exit 1; \
	fi; \
	echo "Downloading $$ASSET ..."; \
	curl -fsSL "$$ASSET" -o /tmp/tamarin-prover; \
	install -m 755 /tmp/tamarin-prover /usr/local/bin/tamarin-prover; \
	echo "Installed: $$(tamarin-prover --version)"

# ── verification ─────────────────────────────────────────────────────────────

.PHONY: prove interactive check

# Run the prover in batch mode; exit 0 if all lemmas verified.
prove:
	$(TAMARIN) --prove $(MODEL)

# Open the interactive browser UI on http://localhost:3001
interactive:
	$(TAMARIN) interactive $(MODEL)

# Quick syntax check (parse only, no proof search)
check:
	$(TAMARIN) $(MODEL)
