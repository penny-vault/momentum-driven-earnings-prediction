package mdep_test

import (
	"context"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/penny-vault/momentum-driven-earnings-prediction/mdep"
	"github.com/penny-vault/pvbt/asset"
	"github.com/penny-vault/pvbt/data"
	"github.com/penny-vault/pvbt/engine"
	"github.com/penny-vault/pvbt/portfolio"
)

var _ = Describe("MomentumDrivenEarningsPrediction", func() {
	var (
		ctx       context.Context
		snap      *data.SnapshotProvider
		nyc       *time.Location
		startDate time.Time
		endDate   time.Time
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		nyc, err = time.LoadLocation("America/New_York")
		Expect(err).NotTo(HaveOccurred())

		snap, err = data.NewSnapshotProvider("testdata/snapshot.db")
		Expect(err).NotTo(HaveOccurred())

		startDate = time.Date(2025, 12, 1, 0, 0, 0, 0, nyc)
		endDate = time.Date(2026, 3, 1, 0, 0, 0, 0, nyc)
	})

	AfterEach(func() {
		if snap != nil {
			snap.Close()
		}
	})

	runBacktest := func() portfolio.Portfolio {
		strategy := &mdep.MomentumDrivenEarningsPrediction{}
		acct := portfolio.New(
			portfolio.WithCash(100000, startDate),
			portfolio.WithAllMetrics(),
		)

		eng := engine.New(strategy,
			engine.WithDataProvider(snap),
			engine.WithAssetProvider(snap),
			engine.WithAccount(acct),
		)

		result, err := eng.Backtest(ctx, startDate, endDate)
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("produces expected returns and portfolio value", func() {
		result := runBacktest()

		summary, err := result.Summary()
		Expect(err).NotTo(HaveOccurred())
		Expect(summary.TWRR).To(BeNumerically("~", 0.1050, 0.01))
		Expect(summary.MaxDrawdown).To(BeNumerically(">", -0.10), "max drawdown should be better than -10%")

		Expect(result.Value()).To(BeNumerically("~", 110502, 500))
	})

	It("rebalances weekly on Fridays", func() {
		result := runBacktest()
		txns := result.Transactions()

		rebalanceDates := map[string]bool{}
		for _, t := range txns {
			if t.Type == asset.BuyTransaction {
				dateStr := t.Date.In(nyc).Format("2006-01-02")
				rebalanceDates[dateStr] = true
			}
		}

		for dateStr := range rebalanceDates {
			d, err := time.Parse("2006-01-02", dateStr)
			Expect(err).NotTo(HaveOccurred())
			Expect(d.Weekday()).To(SatisfyAny(
				Equal(time.Friday),
				Equal(time.Thursday),
			), "rebalance on %s was a %s", dateStr, d.Weekday())
		}
	})

	It("holds the expected stocks at every rebalance", func() {
		result := runBacktest()
		txns := result.Transactions()

		// Build running portfolio from transactions.
		held := map[string]float64{}
		var rebalanceDates []string
		seen := map[string]bool{}

		for _, t := range txns {
			dateStr := t.Date.In(nyc).Format("2006-01-02")
			switch t.Type {
			case asset.BuyTransaction:
				held[t.Asset.Ticker] += t.Qty
				if !seen[dateStr] {
					rebalanceDates = append(rebalanceDates, dateStr)
					seen[dateStr] = true
				}
			case asset.SellTransaction:
				held[t.Asset.Ticker] -= t.Qty
				if held[t.Asset.Ticker] < 0.01 {
					delete(held, t.Asset.Ticker)
				}
			}
		}

		// Replay transactions date by date and verify holdings at each rebalance.
		held = map[string]float64{}
		txnIdx := 0

		for _, rebalDate := range rebalanceDates {
			// Process all transactions up to and including this date.
			for txnIdx < len(txns) {
				t := txns[txnIdx]
				tDate := t.Date.In(nyc).Format("2006-01-02")
				if tDate > rebalDate {
					break
				}
				switch t.Type {
				case asset.BuyTransaction:
					held[t.Asset.Ticker] += t.Qty
				case asset.SellTransaction:
					held[t.Asset.Ticker] -= t.Qty
					if held[t.Asset.Ticker] < 0.01 {
						delete(held, t.Asset.Ticker)
					}
				}
				txnIdx++
			}

			expected, ok := expectedHoldings[rebalDate]
			if !ok {
				continue
			}

			var actual []string
			for ticker := range held {
				actual = append(actual, ticker)
			}
			sort.Strings(actual)

			Expect(actual).To(Equal(expected), "holdings mismatch on %s", rebalDate)
		}
	})
})


var expectedHoldings = map[string][]string{
	"2025-12-05": {"AA", "AEM", "ALL", "APH", "AS", "AU", "BG", "CIB", "CLS", "CRDO", "DB", "DDS", "EXPD", "EXPE", "FIX", "FNV", "FOXA", "FUTU", "GFI", "HOOD", "ILMN", "ISRG", "JHX", "KGC", "LITE", "LOGI", "LVS", "MDB", "MNST", "MS", "MU", "NBIX", "NEM", "NVDA", "ONON", "RDDT", "RGLD", "SCCO", "SNDK", "STX", "TCOM", "TROW", "TRV", "UHS", "UI", "VALE", "VRT", "WDC", "ZTO"},
	"2025-12-12": {"AEM", "AER", "ALL", "APH", "AS", "AU", "BG", "BHP", "CIB", "CLS", "COHR", "CRDO", "DDS", "DY", "EXPD", "EXPE", "FIVE", "FIX", "FOXA", "GFI", "GM", "HOOD", "ILMN", "ISRG", "KGC", "LITE", "LOGI", "LVS", "MDB", "MNST", "MRVL", "MS", "MU", "NBIX", "NEM", "NVDA", "ONON", "RDDT", "RGLD", "RNR", "SNDK", "TCOM", "TRV", "UHS", "UI", "WDC", "ZTO"},
	"2025-12-19": {"ABX", "AEM", "AER", "ALL", "APH", "AS", "AU", "BHP", "CIB", "CIEN", "CLS", "COF", "COHR", "CRDO", "CVE", "EL", "EXPD", "EXPE", "FIX", "FOXA", "GFI", "GM", "HOOD", "ILMN", "ISRG", "KGC", "LITE", "LOGI", "LVS", "MDB", "MNST", "MRVL", "MS", "MU", "NBIX", "NVDA", "ONON", "PAAS", "PSX", "RDDT", "RGLD", "SCCO", "SNDK", "STX", "TCOM", "TRMB", "UBS", "UI", "WDC", "ZTO"},
	"2025-12-26": {"AA", "AEM", "AER", "APH", "AS", "CIB", "CIEN", "CLS", "COHR", "CRDO", "CVE", "DDS", "DY", "EL", "EXPD", "EXPE", "FIVE", "FIX", "FOXA", "GM", "HOOD", "IAG", "ILMN", "IVZ", "KGC", "LITE", "LOGI", "MDB", "MNST", "MU", "NBIX", "ONON", "PAAS", "PSX", "RDDT", "RNR", "SCCO", "SNDK", "STX", "SU", "TCOM", "TRMB", "UBS", "UHS", "UI", "WDC", "ZTO"},
	"2026-01-02": {"AA", "ADI", "AEM", "APH", "APP", "BG", "CDE", "CIB", "CIEN", "CNM", "COHR", "CRDO", "CVE", "DY", "EPAM", "EXAS", "EXPD", "EXPE", "FIVE", "FUTU", "GM", "HOOD", "KGC", "LITE", "LLY", "MDB", "MNST", "MTZ", "MU", "ONON", "PAA", "PAAS", "PLTR", "PODD", "PSX", "RNR", "SGI", "SNDK", "STX", "SUN", "TCOM", "UI", "ULTA", "VALE", "WDC", "ZM", "ZTO"},
	"2026-01-09": {"VUSTX"},
	"2026-02-27": {"VUSTX"},
}
