package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/atvirokodosprendimai/wgmesh/pkg/analytics"
	"github.com/atvirokodosprendimai/wgmesh/pkg/promo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Get storage paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	wgmeshDir := filepath.Join(homeDir, ".wgmesh")
	promosPath := filepath.Join(wgmeshDir, "promos.json")
	analyticsPath := filepath.Join(wgmeshDir, "analytics.log")

	// Load promo store
	store, err := promo.NewStore(promo.StoreConfig{StoragePath: promosPath})
	if err != nil {
		return fmt.Errorf("load promo store: %w", err)
	}

	// Create analytics calculator
	calc := analytics.NewCalculator(analyticsPath)

	// Display dashboard
	return displayDashboard(store, calc)
}

func displayDashboard(store *promo.Store, calc *analytics.Calculator) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header
	fmt.Println("=== wgmesh Outreach Dashboard ===")
	fmt.Println()

	// Overall funnel metrics
	funnel, err := calc.ComputeFunnelMetrics()
	if err != nil {
		return fmt.Errorf("compute funnel metrics: %w", err)
	}

	fmt.Fprintln(w, "Overall Conversion Funnel:")
	fmt.Fprintln(w, "  Signups\tActivations\tConversions")
	fmt.Fprintf(w, "  %d\t%d\t%d\n", funnel.SignupCount, funnel.ActivationCount, funnel.ConversionCount)
	fmt.Fprintln(w)

	// Campaign stats
	campaigns := store.ListCampaigns()
	if len(campaigns) == 0 {
		fmt.Println("No campaigns found. Create campaigns with promo codes.")
		return nil
	}

	sort.Slice(campaigns, func(i, j int) bool {
		return campaigns[i].CreatedAt.After(campaigns[j].CreatedAt)
	})

	fmt.Fprintln(w, "Campaign Performance:")
	fmt.Fprintln(w, "  Campaign\tSource\tGenerated\tRedeemed\tSignups\tConversions\tRate")
	fmt.Fprintln(w, "  -------\t------\t---------\t--------\t-------\t-----------\t----")

	for _, campaign := range campaigns {
		generated, redeemed, _ := store.GetCampaignStats(campaign.ID)
		metrics, _ := calc.ComputeCampaignMetrics(campaign.ID)

		source := string(campaign.Source)
		rate := fmt.Sprintf("%.1f%%", metrics.ConversionRate)

		fmt.Fprintf(w, "  %s\t%s\t%d\t%d\t%d\t%d\t%s\n",
			truncate(campaign.Name, 20),
			truncate(source, 10),
			generated,
			redeemed,
			metrics.Signups,
			metrics.Conversions,
			rate)
	}

	fmt.Fprintln(w)
	fmt.Println("Run 'wgmesh promo create' to generate codes for campaigns.")

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
