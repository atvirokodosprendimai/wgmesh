#!/usr/bin/env bash
# audit-polar-products.sh — Audit Polar.sh product configurations for cloudroof tiers
# Usage: bash scripts/audit-polar-products.sh
#
# This script catalogs the Polar.sh product IDs used for cloudroof.eu sponsor tiers
# and provides direct links to Polar.sh dashboards for manual verification.

set -euo pipefail

PRODUCTS=(
    "3f5d75de-936b-49d8-a21b-4b79d9fd22c1:Founding Member:5"
    "1927e637-4cfd-4c94-8bee-c5518803bc89:Edge Node:20"
    "eb20683e-55ea-4354-9d8c-070e55a4eff5:Mesh Operator:100"
    "8e8e1c33:Unknown Paying Product:?"
)

echo "═════════════════════════════════════════════════════════════════════"
echo "Product Configuration Audit"
echo "═════════════════════════════════════════════════════════════════════"
echo "Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
echo ""

for product in "${PRODUCTS[@]}"; do
    IFS=':' read -r id name price <<< "$product"
    echo "Product: $name"
    echo "ID: $id"
    echo "Price: \$$price/mo"
    echo "Polar.sh Checkout: https://polar.sh/checkout?productId=$id"
    echo ""
done

echo "═════════════════════════════════════════════════════════════════════"
echo "Manual Verification Steps"
echo "═════════════════════════════════════════════════════════════════════"
echo ""
echo "1. Visit each Polar.sh checkout link above"
echo "2. Check subscriber count displayed on product page"
echo "3. Note product positioning language (sponsorship vs. product purchase)"
echo "4. Record benefit timing (immediate vs. future-delivered)"
echo "5. Document social proof display strategy"
echo ""
echo "Analysis results should be documented in:"
echo "  - docs/polar-products.md"
echo "  - docs/product-8e8e1c33-analysis.md"
echo "  - docs/cloudroof-positioning-analysis.md"
echo "  - docs/benefit-delivery-roadmap.md"
