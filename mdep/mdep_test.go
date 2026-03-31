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

		startDate = time.Date(2024, 6, 1, 0, 0, 0, 0, nyc)
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
		Expect(summary.TWRR).To(BeNumerically("~", 0.7963, 0.01))
		Expect(summary.MaxDrawdown).To(BeNumerically(">", -0.25), "max drawdown should be better than -25%")

		Expect(result.Value()).To(BeNumerically("~", 179626, 500))
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
	"2024-06-07": {"ANET", "APP", "CHWY", "COIN", "CSL", "DD", "DDOG", "DELL", "EME", "FIX", "FWONK", "GE", "GFI", "GOOG", "GOOGL", "GRMN", "GS", "IHG", "JD", "KSPI", "LDOS", "MEDP", "MGM", "MNDY", "MS", "NFLX", "NVDA", "OC", "PDD", "PHM", "RBA", "RCL", "RGA", "RMD", "SCCO", "SKX", "SN", "SPOT", "SUZ", "THC", "TROW", "TXRH", "TYL", "VRT", "VTMX", "WAB", "WING", "WRK", "WWD", "ZBRA"},
	"2024-06-14": {"AMKR", "ANET", "ANF", "APH", "APP", "APTV", "AU", "CAVA", "CHWY", "COIN", "CSL", "DD", "DDOG", "DUOL", "EME", "FIX", "FWONK", "GFI", "GOOG", "GOOGL", "GPS", "GRMN", "GS", "JD", "LDOS", "MEDP", "MNDY", "MS", "NFLX", "NVDA", "OC", "PDD", "PHM", "RBA", "RCL", "SCCO", "SKX", "SN", "SPOT", "SUZ", "THC", "TYL", "VTMX", "WAB", "WING", "WMS", "WRK", "WSM", "WWD", "ZBRA"},
	"2024-06-21": {"AER", "AMKR", "ANET", "ANF", "APP", "CAVA", "CEG", "CHWY", "COIN", "CSL", "DDOG", "DUOL", "EME", "F", "FIX", "FWONK", "GOOG", "GOOGL", "GPS", "GRMN", "HES", "HWM", "ING", "JD", "LDOS", "LOGI", "MMYT", "MNDY", "NVDA", "OC", "ONTO", "PDD", "PHM", "RBA", "RCL", "RGA", "SKX", "SN", "SPOT", "THC", "TYL", "VST", "VTMX", "WAB", "WING", "WMS", "WRK", "WSM", "WWD", "ZBRA"},
	"2024-06-28": {"AMKR", "ANET", "ANF", "APP", "AXTA", "BBVA", "BRBR", "CAVA", "CBSH", "CHWY", "COIN", "DBX", "DDOG", "DUOL", "ESLT", "FIX", "FWONK", "GPS", "GRMN", "HWM", "ING", "IP", "JD", "KSPI", "LDOS", "LOGI", "MMYT", "MNDY", "NVDA", "NYT", "OC", "ONTO", "PDD", "RBA", "RCL", "RGA", "SFM", "SN", "SPOT", "THC", "TXRH", "VST", "VTMX", "WING", "WMS", "WRK", "WSM", "WWD", "ZBRA", "ZS"},
	"2024-07-05": {"AIT", "ANF", "APH", "APP", "BBVA", "BRBR", "BWA", "CAMT", "CAVA", "CEG", "CHWY", "COIN", "CRS", "CYBR", "DBX", "DDOG", "DELL", "DUOL", "ESLT", "FRPT", "FTAI", "FWONK", "GEV", "GPS", "HES", "JD", "KD", "LPX", "MNDY", "MTZ", "NVDA", "NYT", "ONTO", "PAYC", "PDD", "RBA", "SAN", "SN", "SPOT", "SQ", "SRPT", "STX", "SUN", "TPL", "VST", "WMS", "WMT", "WRK", "WSM", "ZS"},
	"2024-07-12": {"AEG", "AFRM", "AIT", "AMX", "ANF", "APH", "ASML", "BAP", "BURL", "CAVA", "CHDN", "CHWY", "CVNA", "CYBR", "DDOG", "DELL", "DUOL", "ESLT", "FNV", "FTAI", "FWONK", "GEV", "GL", "GLW", "GPS", "HES", "IBKR", "JD", "MGM", "MNDY", "NBIX", "NVDA", "PAYC", "PDD", "RBLX", "SAN", "SN", "SPOT", "SQ", "STX", "TCOM", "TROW", "TWLO", "VRTX", "VST", "WCN", "WMS", "WMT", "Z", "ZS"},
	"2024-07-19": {"AEG", "AER", "AFRM", "AMZN", "ANET", "BURL", "CAVA", "CHWY", "CYBR", "DDOG", "DHI", "ESLT", "FLR", "FNV", "FTAI", "FWONK", "GE", "GEV", "GLW", "GM", "GMED", "GPS", "IBKR", "IFF", "IP", "JD", "LOGI", "MU", "NTRA", "NVDA", "OHI", "ONON", "PDD", "PODD", "RGA", "SHOP", "SKX", "SN", "SPOT", "SW", "TCOM", "TEF", "TSM", "TTWO", "TWLO", "VST", "WING", "WMS", "Z", "ZG"},
	"2024-07-26": {"AEG", "ANET", "APPF", "BURL", "CART", "CAVA", "CBSH", "CHWY", "DDOG", "ESLT", "EXPE", "FNV", "FTAI", "GE", "GL", "GLW", "GMED", "GTLB", "HCA", "HON", "IBKR", "IP", "ISRG", "MANH", "MU", "OHI", "OKE", "OKTA", "ONON", "PAA", "PDD", "PGR", "PKG", "PODD", "PYPL", "RGA", "SHOP", "SKX", "SMFG", "STX", "THC", "TSM", "TTWO", "TWLO", "VRT", "VST", "WDC", "WING", "WIX", "WMS"},
	"2024-08-02": {"AEG", "ANF", "APH", "ARGX", "CBSH", "CHWY", "CLS", "CRS", "DRS", "DUOL", "EME", "ERIE", "ESI", "FIX", "FNV", "GE", "GPS", "HALO", "HCA", "HII", "IBKR", "ILMN", "ISRG", "JHG", "MANH", "MKSI", "NEM", "NOW", "OHI", "OLLI", "ONON", "PCVX", "PKG", "PSN", "PYPL", "SMFG", "SOFI", "SPOT", "THC", "TSM", "TTWO", "TWLO", "TXRH", "UHS", "UTHR", "VRT", "WDC", "WING", "WMG", "WMS"},
	"2024-08-09": {"AEG", "AER", "ANET", "ANF", "APH", "ARGX", "CART", "CBSH", "CEG", "CHRW", "CHWY", "CRS", "DUOL", "EME", "ERIE", "EXAS", "FIX", "FLUT", "FNV", "FTAI", "GE", "GPS", "HCA", "HLNE", "HWM", "IBKR", "ILMN", "ISRG", "LDOS", "MANH", "MASI", "MKSI", "MTG", "NEM", "NOW", "ONON", "PKG", "PPC", "PYPL", "RCL", "SFM", "SMFG", "SOFI", "SPOT", "THC", "TSM", "TXRH", "UHS", "ULS", "VRT"},
	"2024-08-16": {"AEG", "AER", "ANET", "APP", "AZPN", "CART", "CBSH", "CHRW", "CHWY", "CLS", "CRS", "DUOL", "EME", "ERIE", "ESI", "FIX", "FLUT", "FNV", "GE", "HCA", "HLNE", "HWM", "IBKR", "ILMN", "ISRG", "JXN", "LDOS", "MANH", "MASI", "NCLH", "NEM", "NOW", "NYT", "PARAA", "PKG", "PPC", "PYPL", "RCL", "SFM", "SMFG", "SN", "SOFI", "SPOT", "THC", "TLN", "UHS", "ULS", "VRT", "WAB", "ZM"},
	"2024-08-23": {"AEG", "ANET", "APP", "AXS", "AZPN", "CART", "CBSH", "CHRW", "CRS", "DUOL", "DVA", "EME", "ERIE", "FIX", "GE", "HCA", "HLNE", "HOOD", "HRB", "HWM", "ILMN", "ISRG", "JD", "JXN", "KT", "MANH", "MASI", "MTG", "NCLH", "NEM", "NOW", "NVMI", "NYT", "PARAA", "PPC", "PYPL", "RCL", "SFM", "SMFG", "SN", "SOFI", "SPOT", "STWD", "STX", "THC", "TLN", "TXRH", "UHS", "VRT", "WAB"},
	"2024-08-30": {"AIZ", "ANET", "ANF", "APP", "AXS", "AZPN", "CART", "CBSH", "CHRW", "CRS", "DUOL", "DVA", "EME", "ERIE", "ESI", "FIS", "FIX", "FLUT", "HCA", "HOOD", "HRB", "HWM", "IBKR", "ILMN", "ISRG", "JD", "JXN", "KT", "MANH", "MASI", "MTG", "NCLH", "NEM", "NVMI", "PARAA", "PKG", "PPC", "PYPL", "RCL", "SFM", "SMFG", "SN", "SOFI", "SPOT", "STX", "THC", "TLN", "TT", "UHS", "VRT"},
	"2024-09-06": {"ALLY", "ANF", "APP", "AZPN", "CART", "CBSH", "CHRW", "COOP", "CRS", "DUOL", "E", "EME", "ERIE", "ESI", "FIX", "HCA", "HRB", "HWM", "ILMN", "INSP", "ISRG", "JD", "JXN", "KT", "MANH", "MASI", "MPLX", "MTG", "NCLH", "NEM", "NYT", "OKTA", "PARAA", "PKG", "PPC", "PYPL", "RCL", "SFM", "SMAR", "SN", "SPOT", "STEP", "STX", "THC", "TLN", "TT", "TXRH", "UHS", "UL", "VRT"},
	"2024-09-13": {"ANF", "APP", "AZPN", "CART", "CBSH", "CHRW", "COOP", "CPT", "CRS", "CTLT", "DOCU", "DUOL", "DVA", "EME", "ERIE", "FIX", "FLUT", "HCA", "HRB", "HWM", "ILMN", "INSP", "ISRG", "JD", "KRYS", "KT", "LLY", "MANH", "MASI", "MCO", "MMYT", "MPLX", "MTG", "NCLH", "NVMI", "NYT", "OKTA", "PARAA", "PPC", "PYPL", "RCL", "SFM", "SPOT", "STEP", "STX", "THC", "TLN", "UHS", "UI", "VRT"},
	"2024-09-20": {"AFL", "AFRM", "ANF", "APP", "AZPN", "BPOP", "CART", "CHRW", "CRS", "CTLT", "DOCU", "DUOL", "DVA", "EME", "ERIE", "FIX", "FLUT", "FTNT", "HCA", "HRB", "ILMN", "INSP", "JD", "LLY", "MANH", "MMYT", "MTG", "NCLH", "NYT", "PARAA", "PGR", "PKG", "PPC", "PYPL", "SAP", "SBS", "SFM", "SIRI", "SN", "SOFI", "SPOT", "STEP", "STX", "TAK", "THC", "TLN", "TT", "UHS", "VRT", "WAB"},
	"2024-09-27": {"AER", "AFL", "AFRM", "AIZ", "ANF", "APP", "AU", "AXS", "AZPN", "BPOP", "CHRW", "CM", "CRS", "DOCU", "DRS", "DUOL", "DVA", "FIS", "FIX", "FLUT", "FTNT", "HESM", "HOOD", "HRB", "HWM", "INSP", "JD", "KT", "LLY", "MASI", "MTG", "NCLH", "NTAP", "NYT", "OKTA", "PARAA", "PGR", "PPC", "PYPL", "SBS", "SFM", "SOFI", "STEP", "STX", "TAK", "THC", "TLN", "UHS", "UI", "UL"},
	"2024-10-04": {"AFRM", "AIZ", "ALNY", "ANF", "APP", "ARGX", "AU", "AZPN", "BSY", "CCL", "CM", "CRS", "CTLT", "DOCU", "DUOL", "DVA", "EXAS", "FIS", "FLUT", "FTAI", "FTNT", "HOOD", "HRB", "INSP", "JD", "KRYS", "KT", "LLY", "MASI", "NCLH", "NTAP", "NVMI", "NYT", "OKTA", "PARAA", "PGR", "QFIN", "RKT", "RYAAY", "SN", "STEP", "STX", "TAK", "THC", "TMDX", "TWLO", "UI", "UL", "VRT", "WDAY"},
	"2024-10-11": {"AFRM", "ALNY", "ANF", "ARGX", "AZEK", "BEKE", "BRBR", "CCL", "CM", "COOP", "CRS", "CTLT", "CVNA", "CYBR", "DOCU", "DUOL", "FLUT", "FRPT", "FWONA", "HOOD", "HQY", "HRB", "IFF", "INSP", "JAZZ", "JD", "KBR", "MFC", "MMYT", "MOD", "MPLX", "MTG", "NCLH", "NTAP", "OKTA", "PFSI", "PGR", "ROKU", "RYAAY", "SHOP", "SN", "STEP", "STX", "TRGP", "UI", "UL", "VERX", "VRT", "WDAY", "WPM"},
	"2024-10-18": {"AEM", "AER", "AFRM", "ANF", "BABA", "BRBR", "CCL", "CF", "CHWY", "CM", "CRS", "CSGP", "CTLT", "CW", "CYBR", "DDOG", "DLB", "DOCS", "DOCU", "ERIC", "FFIV", "FLUT", "FN", "FTNT", "FUTU", "FWONA", "GTLB", "HMY", "IP", "JD", "JXN", "LNG", "MMYT", "MOD", "NCLH", "NTAP", "NWG", "OKTA", "PGR", "RYAAY", "SHOP", "SUZ", "TCOM", "TKO", "UI", "UL", "WDAY", "Z", "ZG", "ZM"},
	"2024-10-25": {"AEM", "AFRM", "AGR", "ANF", "BABA", "BLK", "BRBR", "CAVA", "CCL", "CF", "CLS", "CM", "CRS", "CTLT", "CVNA", "CYBR", "DOCU", "DRS", "ERIC", "FLUT", "FND", "FUTU", "FWONA", "GAP", "HLNE", "HMY", "HRB", "IFF", "KB", "KGC", "KNTK", "LTM", "MMYT", "MNDY", "MPWR", "NCLH", "NWG", "OKTA", "PANW", "PGR", "PODD", "ROKU", "RYAAY", "SEIC", "SHOP", "SUZ", "UI", "UMC", "VRT", "ZG"},
	"2024-11-01": {"AEM", "AFRM", "AGR", "APH", "BAH", "BLK", "BRBR", "BSAC", "CCL", "CHE", "CLS", "CRS", "CTLT", "DOCU", "EXAS", "FIX", "FLUT", "FTNT", "FUTU", "FWONA", "HLNE", "HQY", "IFF", "IX", "KNTK", "LDOS", "LTM", "LUV", "MCO", "MMYT", "MNDY", "MPWR", "NCLH", "NOW", "NTRS", "NWG", "PGR", "PKG", "RGA", "RYAAY", "SEIC", "SFM", "SHOP", "SYF", "TGT", "THC", "TSLA", "UI", "VRT", "WF"},
	"2024-11-08": {"AEM", "AGR", "ANET", "APH", "BABA", "BLK", "BSAC", "CAVA", "CCL", "CF", "CLS", "COHR", "CRS", "CVNA", "CYBR", "DRS", "EME", "FUTU", "FWONA", "GRMN", "GWRE", "INGR", "KNTK", "KVYO", "LDOS", "LTM", "LUMN", "LUV", "MASI", "MNDY", "MTZ", "NTRS", "NWG", "PANW", "PGR", "PKG", "PODD", "PSN", "RYAAY", "SEIC", "SFM", "SHOP", "STN", "STT", "SYF", "THC", "TSLA", "TWLO", "VRT", "ZBRA"},
	"2024-11-15": {"AEM", "APH", "APP", "BABA", "BILL", "BLK", "CCL", "CF", "CLS", "CVNA", "CYBR", "DECK", "DOCS", "DRS", "EME", "FIX", "FTNT", "FUTU", "GFI", "GRMN", "INGR", "LDOS", "LUV", "MNDY", "MTSI", "MTZ", "NTRS", "NVDA", "NWG", "PARAA", "PKG", "PSN", "RJF", "RMD", "RYAAY", "SEIC", "SFM", "SRAD", "STT", "SYF", "THC", "TOST", "TSLA", "TSM", "TWLO", "UBS", "ULS", "VRT", "WAB", "ZBRA"},
	"2024-11-22": {"AEM", "APH", "APP", "BABA", "BLK", "BRBR", "CCL", "CF", "CLS", "CRS", "CVNA", "DECK", "DOCS", "DRS", "EME", "FIX", "FTNT", "FUTU", "GFI", "GRMN", "INGR", "LDOS", "LUV", "MASI", "MNDY", "MTSI", "MTZ", "NCLH", "NTRS", "NVDA", "NWG", "PARAA", "PKG", "PSN", "RJF", "RMD", "SEIC", "SFM", "SRAD", "STT", "SYF", "THC", "TOST", "TSLA", "TSM", "TWLO", "ULS", "VRT", "WAB", "ZBRA"},
	"2024-11-29": {"ANF", "APH", "APP", "BAM", "BK", "BLK", "CCL", "CF", "CLS", "CRS", "CVNA", "DBX", "DECK", "DOCS", "DRS", "EME", "FIX", "FTNT", "FUTU", "GAP", "GRMN", "INGR", "JXN", "LDOS", "LPX", "LTM", "MASI", "MTSI", "MTZ", "NCLH", "NTNX", "NWG", "PARAA", "PKG", "PSN", "RJF", "SEIC", "SFM", "STT", "SYF", "TCOM", "THC", "TSLA", "TSM", "TWLO", "ULS", "VIK", "VRT", "VST", "ZBRA"},
	"2024-12-06": {"ANF", "APP", "BAM", "BK", "CCL", "CF", "CHH", "CLS", "CRS", "CVNA", "DBX", "DDS", "DECK", "DOCS", "DRS", "EME", "FAF", "FIX", "FLUT", "FTNT", "GAP", "GBCI", "INGR", "JXN", "LDOS", "LPX", "LTM", "MASI", "MTSI", "MTZ", "NWG", "PARAA", "PKG", "PSN", "RJF", "SFM", "STRL", "SYF", "TAL", "TCOM", "TOST", "TSLA", "TSM", "TWLO", "UI", "ULS", "VST", "ZBRA", "ZION", "ZM"},
	"2024-12-13": {"APP", "BAM", "CACI", "CF", "CLS", "CMA", "COIN", "CRS", "CVNA", "DBX", "DECK", "DOCS", "DRS", "EME", "FLUT", "FTNT", "GAP", "GRAB", "GRMN", "INGR", "LDOS", "LTM", "LUV", "MASI", "MNDY", "MRVL", "MTSI", "MTZ", "NTRS", "NWG", "PARAA", "PKG", "PSN", "RJF", "SEIC", "SFM", "SYF", "TCOM", "TOST", "TSLA", "TSM", "TWLO", "UAL", "UI", "ULS", "VIK", "VRT", "VST", "ZBRA", "ZM"},
	"2024-12-20": {"ALL", "APH", "APP", "CF", "CLS", "COIN", "CVNA", "DBX", "DECK", "DOCS", "DRS", "EME", "FLUT", "FTNT", "GAP", "GME", "GRAB", "GRMN", "INGR", "LDOS", "LTM", "LUV", "MCK", "MRVL", "MTSI", "MTZ", "NTES", "NTRS", "NWG", "PARAA", "PKG", "PSN", "RJF", "SEIC", "SFM", "SMAR", "TCOM", "TOST", "TPR", "TSLA", "TSN", "TWLO", "UI", "ULS", "VIK", "VRT", "VST", "YPF", "ZBRA", "ZM"},
	"2024-12-27": {"AAL", "ALL", "ANF", "APP", "BAM", "COIN", "CRS", "CVNA", "DBX", "DOCS", "DRS", "EME", "FIX", "FMS", "FTI", "FTNT", "GAP", "GME", "GRAB", "GRMN", "KEYS", "LDOS", "LTM", "LUV", "MCK", "MNDY", "MRVL", "MTSI", "MTZ", "NTES", "NTRS", "NWG", "PARAA", "PSN", "RBA", "RJF", "SFM", "SMAR", "TCOM", "TOST", "TPR", "TSN", "TWLO", "UI", "ULS", "VIK", "VST", "YPF", "ZBRA", "ZM"},
	"2025-01-03": {"AAL", "AAON", "APP", "BAM", "BRBR", "COIN", "CTRA", "CVNA", "DASH", "DBX", "DOCS", "DTM", "FNF", "FTI", "FTNT", "GGAL", "GME", "GRAB", "GRMN", "HOOD", "KEYS", "LDOS", "LUV", "MCK", "MNDY", "MRVL", "MTSI", "MTZ", "NCLH", "NTES", "NWG", "PAA", "PARAA", "PPC", "RBA", "RYAAY", "SAN", "SE", "SUZ", "TCOM", "TOST", "TPR", "TSN", "UI", "ULS", "VIK", "VST", "WY", "YPF", "ZM"},
	"2025-01-10": {"AAL", "AFRM", "AMZN", "APP", "AU", "AXON", "BAH", "BEKE", "COIN", "CTRA", "CVNA", "DASH", "DECK", "DVA", "FMS", "FNF", "GME", "GRAB", "GRMN", "HOOD", "HSBC", "KEYS", "MRVL", "MTZ", "NCLH", "NTES", "NWG", "PAA", "PPC", "PRMB", "RBA", "RGA", "ROKU", "SCHW", "SE", "SFM", "SONY", "SUZ", "TCOM", "TOST", "TPR", "TRV", "TSN", "UAL", "UI", "VIK", "VST", "WY", "YPF", "ZM"},
	"2025-01-17": {"AAL", "AMZN", "APP", "AU", "AXON", "BCH", "BILL", "BMRN", "BNTX", "COIN", "CRS", "DECK", "DTM", "DUOL", "EQNR", "FHN", "FLUT", "FOX", "GGAL", "GM", "GME", "GRMN", "GTLB", "HOOD", "HUBS", "JLL", "KEYS", "KMX", "MRVL", "MTZ", "OVV", "PAA", "PCG", "PEN", "PPC", "SE", "SHOP", "SOFI", "SUZ", "TCOM", "TER", "TLN", "TSN", "UAL", "UI", "VFC", "VIK", "WFC", "WPM", "WY", "ZM"},
	"2025-01-24": {"AA", "AAL", "ALK", "APP", "AU", "AXON", "BILL", "BMRN", "BNTX", "C", "CEG", "COHR", "COIN", "CRDO", "DRS", "DTM", "EQNR", "FHN", "FLUT", "FTNT", "GGAL", "GME", "GS", "GTLB", "HOOD", "IBKR", "IP", "JD", "JLL", "JPM", "KMX", "KT", "MRVL", "MS", "NWSA", "PAA", "PPC", "ROKU", "SUN", "SUZ", "TCOM", "TER", "TLN", "TPR", "UAL", "UI", "VFC", "WFC", "WY", "ZG", "ZM"},
	"2025-01-31": {"ALK", "APH", "APP", "AU", "AXON", "BBIO", "BILL", "BMRN", "BNTX", "BOOT", "BZ", "C", "DVA", "EAT", "EQNR", "FFIN", "FHN", "FIVE", "FLUT", "FRPT", "GGAL", "GME", "GS", "HOOD", "IBKR", "JLL", "JPM", "KT", "LTH", "MC", "MRVL", "MS", "NTRS", "NWSA", "ORI", "PAA", "PATH", "PPC", "PRI", "SE", "SMFG", "SSB", "SUN", "SUZ", "TCOM", "TLN", "TWLO", "UAL", "VIRT", "WFC", "WY"},
	"2025-02-07": {"AFRM", "ALK", "ANET", "APH", "ARGX", "AU", "AXON", "C", "CALM", "CLS", "CVNA", "DECK", "EAT", "FFIN", "FHN", "FLUT", "FN", "FOX", "FOXA", "FWONK", "GFI", "GGAL", "GME", "GS", "HOOD", "IBKR", "JPM", "KT", "LTH", "MC", "MS", "NFG", "NLY", "NTRS", "OKTA", "ORI", "PAA", "PIPR", "PNFP", "PPC", "RBC", "SSB", "SUN", "SUZ", "TCOM", "THG", "TWLO", "UAL", "URBN", "VIRT", "WFC"},
	"2025-02-14": {"AFRM", "ANET", "APH", "AR", "ARGX", "AU", "AXON", "AZEK", "BMA", "BSX", "C", "CFR", "CLS", "CVNA", "DECK", "DOCS", "EAT", "FHN", "FLUT", "FN", "FOX", "FWONK", "GFI", "GS", "HOOD", "IBKR", "IDCC", "JPM", "KT", "LTH", "MC", "MS", "NFG", "NLY", "NTRS", "OKTA", "ORI", "PAA", "PNFP", "PPC", "RBC", "RH", "SF", "SSB", "SUZ", "SYF", "TCOM", "THG", "UAL", "VIRT", "WFC"},
	"2025-02-21": {"AFRM", "APH", "APP", "AR", "ARGX", "AU", "AXON", "AXTA", "AZEK", "BAX", "BJ", "C", "CBSH", "CFR", "CLS", "COIN", "CVNA", "DECK", "DOCS", "EAT", "FCNCA", "FHN", "FLUT", "FN", "FOX", "GE", "GFI", "GS", "HOOD", "IBKR", "JPM", "KT", "LTH", "MS", "NFG", "NLY", "NRG", "NTRS", "NVMI", "OKTA", "ORI", "PAA", "PPC", "PRMB", "RBC", "SF", "SRAD", "SUZ", "TAP", "UAL", "WFC"},
	"2025-02-28": {"AFRM", "APP", "AR", "ARGX", "AU", "AXON", "AXTA", "BBVA", "BCS", "C", "CBSH", "CFR", "CLS", "COIN", "CVNA", "DBX", "DECK", "DOCS", "FCNCA", "FHN", "FLUT", "FOX", "GE", "GRMN", "GS", "HOOD", "IBKR", "JPM", "KT", "MASI", "MS", "NLY", "NTRS", "NWG", "OKTA", "ORI", "PAA", "PGR", "PPC", "PRMB", "RBC", "SF", "SFM", "SRAD", "SUZ", "TAP", "TKO", "U", "UAL", "UI", "WFC"},
	"2025-03-07": {"ACIW", "APH", "APP", "AR", "AU", "AXON", "AXTA", "BBVA", "BCS", "CBSH", "CLS", "COIN", "DBX", "DOCS", "EAT", "EME", "FCNCA", "FLUT", "FN", "FOX", "GE", "GRMN", "GS", "HOOD", "IBKR", "JAZZ", "JPM", "KT", "LTH", "MASI", "MS", "NFG", "NVMI", "NWG", "ONON", "OPCH", "ORI", "PAA", "PPC", "PRMB", "PSO", "RBC", "SF", "SFM", "SRAD", "SUZ", "THG", "U", "UAL", "UI", "VIRT"},
	"2025-03-14": {"ACIW", "AFRM", "APP", "AR", "AU", "AVGO", "AXON", "AXTA", "BABA", "BBVA", "BCS", "BK", "CBSH", "COIN", "CRDO", "DBX", "DOCS", "EAT", "EME", "FFIN", "FOX", "GAP", "GE", "GRMN", "GS", "HOOD", "IDCC", "JD", "JPM", "LTH", "MASI", "MS", "NFLX", "NTRS", "NVMI", "NWG", "OPCH", "PGR", "PPC", "PRMB", "SF", "SFM", "SRAD", "TAP", "THG", "UI", "USM", "VIK", "VIRT", "XP", "ZION"},
	"2025-03-21": {"ACIW", "AGO", "APP", "AR", "AROC", "AXON", "AXTA", "BBVA", "BGC", "BIDU", "CALM", "CBSH", "COIN", "CRDO", "CVCO", "DBX", "DOCS", "EAT", "EME", "EXE", "FMS", "FN", "FOX", "GAP", "GE", "GRMN", "IDCC", "JD", "LTH", "LUMN", "MASI", "MAT", "MATX", "MWA", "NVMI", "NWG", "OPCH", "PAA", "PATH", "PGR", "PIPR", "PPC", "PRMB", "SF", "THG", "UI", "URBN", "USM", "VEEV", "VIRT", "XP"},
	"2025-03-28": {"ACIW", "AGO", "APP", "AROC", "AXON", "AXTA", "BBVA", "BGC", "BIDU", "CACC", "CALM", "COIN", "CRDO", "CVCO", "DBX", "DOCS", "EAT", "ERJ", "EXE", "FMS", "FN", "FOX", "GAP", "GME", "GRMN", "IDCC", "JD", "LTH", "MASI", "MAT", "MATX", "NFG", "NVMI", "NWG", "NXT", "OPCH", "PATH", "PIPR", "PPC", "PRMB", "QFIN", "RBC", "RDDT", "SF", "STN", "TAP", "TEO", "UI", "USM", "VIRT", "XP"},
	"2025-04-04": {"ACIW", "AGO", "APP", "APTV", "AROC", "AVGO", "AXON", "AXTA", "BABA", "BBVA", "BCS", "BIDU", "COIN", "CRDO", "DBX", "DOCS", "EME", "ERJ", "EXE", "FMS", "FOX", "FUTU", "GAP", "GME", "IDCC", "JBTM", "JD", "LTH", "MASI", "MAT", "MRVL", "NVMI", "NWG", "OPCH", "PATH", "PRMB", "QFIN", "RDDT", "RL", "SFM", "STN", "TAP", "TEF", "TEO", "THG", "U", "UI", "USM", "VEEV", "VFC", "XP"},
	"2025-04-11": {"ACIW", "AFRM", "AGO", "AVGO", "AXON", "BABA", "BBVA", "BCPC", "BCS", "BIDU", "CALM", "CBZ", "CME", "COIN", "CRDO", "DBX", "EME", "ERJ", "EXE", "FMS", "FUTU", "GAP", "GME", "IDCC", "IX", "JD", "JWN", "LTH", "MASI", "NFG", "NVMI", "NWG", "NWSA", "OPCH", "PATH", "PRMB", "QFIN", "RELX", "SBS", "STN", "STRL", "TAP", "TEF", "TEO", "U", "UHS", "UI", "USM", "VEEV", "VFC", "XP"},
	"2025-04-17": {"ACIW", "AFRM", "AGO", "AVGO", "AXON", "BABA", "BBVA", "BCPC", "BCS", "BIDU", "CALM", "CBZ", "CME", "COIN", "CRDO", "CRS", "DBX", "EME", "ERJ", "EXE", "FMS", "FUTU", "GAP", "GME", "HSBC", "IX", "JD", "LTH", "MASI", "NFG", "NWG", "OPCH", "PATH", "PAYC", "PRMB", "QFIN", "SAN", "SBS", "SFM", "SMCI", "STN", "STRL", "SUZ", "TEF", "TEO", "U", "UHS", "UI", "USM", "VEEV", "XP"},
	"2025-04-25": {"ACIW", "AES", "AFRM", "AGO", "AROC", "AVGO", "AXON", "BABA", "BBVA", "BCPC", "BGC", "CALM", "CELH", "CME", "CRDO", "CWST", "EME", "ERJ", "EXE", "FIX", "FMS", "GAP", "GME", "IDCC", "ING", "IX", "LRN", "MASI", "NWG", "ONON", "OPCH", "PATH", "PAYC", "QFIN", "RYAAY", "SAN", "SAP", "SBS", "SFM", "SMCI", "STN", "STRL", "SUZ", "TEF", "TEO", "U", "UHS", "USM", "VTMX", "WAY", "XP"},
	"2025-05-02": {"AEM", "AFRM", "AROC", "AVGO", "AXON", "BANF", "BCH", "BCS", "BGC", "BSAC", "CALM", "CBZ", "CCEP", "CNC", "CRDO", "CRS", "DB", "ERJ", "ESE", "EXE", "FER", "FOX", "GAP", "GLNG", "GME", "IDCC", "ING", "ITGR", "IX", "JWN", "LRN", "LTM", "NNI", "PAM", "PATH", "PAYC", "PM", "QFIN", "RMBS", "RYAAY", "SBS", "SFM", "SKM", "SMCI", "SPR", "STNE", "SWX", "TEF", "TEO", "U", "VTMX"},
	"2025-05-09": {"AEM", "APH", "AXON", "BANF", "BBVA", "BCH", "BCS", "BLKB", "BSAC", "BTSG", "CALM", "CCEP", "CDE", "CRS", "CVNA", "DB", "EBC", "EHC", "FBP", "FER", "FIX", "GME", "GPOR", "GVA", "HMY", "ITGR", "IX", "JNPR", "LAUR", "LRN", "LTM", "MRX", "NWG", "ODD", "PATH", "PCTY", "PM", "QFIN", "RELY", "RMBS", "RYAAY", "SBS", "SFM", "SPR", "STNE", "SWX", "TAK", "TEO", "TNET", "UBSI", "VTMX"},
	"2025-05-16": {"AEM", "APH", "APP", "AXON", "BANF", "BCH", "BCS", "BL", "BSAC", "CALM", "CPA", "CRS", "DB", "EHC", "FBP", "FER", "FIX", "FOX", "FTAI", "FTS", "FWONK", "GME", "GVA", "HMY", "ITGR", "JNPR", "LIF", "LRN", "LTM", "NNI", "NWG", "ODD", "PCTY", "PI", "PM", "QFIN", "QLYS", "RACE", "RMBS", "RYAAY", "SBS", "SFM", "SMFG", "SPR", "STN", "STNE", "SWX", "TNET", "UBSI", "ULS", "VTMX"},
	"2025-05-23": {"AEM", "APH", "APP", "ATR", "AU", "AXON", "BCH", "BCS", "BILL", "BPOP", "BSAC", "CALM", "CPA", "CRS", "CYBR", "DB", "EHC", "ESLT", "FNV", "FOX", "FTS", "FWONK", "GFI", "GME", "HMY", "HWM", "ITGR", "JNPR", "KGC", "LIF", "LTM", "MDLZ", "MNDY", "NEM", "NNI", "NWG", "PCTY", "PM", "QFIN", "QLYS", "RACE", "RGLD", "RMBS", "RYAAY", "SFM", "SMFG", "SWX", "UBSI", "ULS", "VTMX", "WPM"},
	"2025-05-30": {"AEM", "AGI", "APH", "APP", "ATR", "AU", "AXON", "BCH", "BIRK", "BSAC", "CRS", "CVNA", "CYBR", "DB", "EHC", "ESLT", "FER", "FNV", "FOX", "FTS", "FWONK", "HALO", "HWM", "INTU", "JNPR", "KGC", "LIF", "LRN", "LTM", "MNDY", "MTG", "NEM", "NTES", "NWG", "PAAS", "PCTY", "PM", "QLYS", "RGLD", "RMBS", "RYAAY", "SBS", "SFM", "SMFG", "STN", "SWX", "UBSI", "ULS", "URBN", "VTMX", "WPM"},
	"2025-06-06": {"AHR", "APH", "APP", "AU", "AXON", "BCH", "BILL", "BIRK", "BPOP", "BSAC", "CPA", "CRDO", "CRS", "CVNA", "CYBR", "DB", "EHC", "ESLT", "FER", "FERG", "FNV", "FTS", "FWONK", "HMY", "HWM", "INTU", "JNPR", "LIF", "LRN", "LTM", "MNDY", "MTG", "NEM", "NTES", "NWG", "ODD", "PCTY", "PEGA", "PLMR", "PM", "QLYS", "RACE", "RMBS", "RYAAY", "SBS", "SFM", "SMFG", "STN", "UBSI", "ULS", "URBN"},
	"2025-06-13": {"ALGN", "AMG", "APH", "APP", "AU", "AXON", "BCH", "BMA", "BPOP", "BSAC", "CFR", "CRDO", "CVLT", "CVNA", "CYBR", "DB", "DY", "EHC", "ESLT", "EVR", "FER", "FERG", "FIX", "FNV", "FWONK", "GGAL", "HMY", "HWM", "INTU", "JNPR", "KLAC", "LRN", "LTM", "MNDY", "MTG", "NTES", "NVMI", "NWG", "PAYC", "PCTY", "PEGA", "PM", "QLYS", "RACE", "RMBS", "SFM", "SMFG", "UBSI", "ULS", "URBN", "VEEV"},
	"2025-06-20": {"ALGN", "AMG", "APH", "APP", "ATR", "AU", "AXON", "BCH", "BCS", "BSAC", "CFR", "CRDO", "CRS", "CVLT", "DB", "DELL", "DY", "EHC", "ESLT", "FER", "FERG", "FIX", "FNV", "FWONK", "HMY", "HWM", "INTU", "JNPR", "LRN", "LTM", "MNDY", "MTG", "NEM", "NTES", "NVMI", "NWG", "PAYC", "PCTY", "PEGA", "QLYS", "RACE", "RMBS", "ROK", "SFM", "SHAK", "SMFG", "TWLO", "ULS", "URBN", "VIRT", "WDC"},
	"2025-06-27": {"AEM", "ALGN", "ALSN", "AMG", "APP", "ASR", "AU", "AXON", "BCH", "BPOP", "BSAC", "CFR", "CRDO", "CVLT", "DB", "DELL", "DY", "ESLT", "EVR", "FERG", "FLR", "FWONK", "GFI", "HEI", "HMY", "HWM", "INTU", "JBL", "JNPR", "LTM", "MNDY", "MTG", "NEM", "NTES", "NVMI", "NWG", "PAYC", "PCTY", "PEGA", "RACE", "RMBS", "ROK", "SEZL", "SFM", "SMFG", "SN", "TWLO", "ULS", "URBN", "VEEV", "WPM"},
	"2025-07-03": {"AEM", "ALKS", "AMG", "APP", "ASR", "AU", "AXON", "BBVA", "BMA", "BPOP", "CDE", "CRDO", "DB", "DELL", "DY", "ESE", "EVR", "FERG", "FTDR", "FWONK", "GFI", "HBM", "HEI", "HMY", "HWM", "INTU", "JBL", "LIF", "MNDY", "NEM", "NGG", "NNI", "NTES", "NVMI", "NWG", "PATH", "PAYC", "PEGA", "QLYS", "RACE", "ROK", "SEZL", "SN", "STEP", "TFPM", "ULS", "URBN", "VIPS", "VOD", "VRNA", "WPM"},
	"2025-07-11": {"AEM", "AGI", "ASR", "AU", "BBVA", "BILI", "CIB", "COHR", "CRDO", "CTVA", "DB", "DELL", "DY", "EL", "ESAB", "EVR", "FERG", "FNV", "GFI", "HEI", "HIMS", "HMY", "HUBS", "HWM", "IBKR", "INTU", "JBL", "JHG", "KD", "KGC", "LITE", "MNDY", "NGG", "NTES", "NTRS", "NVMI", "PATH", "PEGA", "QFIN", "RGLD", "SEIC", "STEP", "TD", "UGI", "URBN", "VIPS", "VOD", "VRNA", "WPM", "XP"},
	"2025-07-18": {"AEM", "AER", "AFRM", "AGI", "APG", "ASR", "AU", "BBVA", "BILI", "CIB", "COHR", "CRDO", "CTVA", "CYBR", "DB", "DELL", "DXCM", "EL", "EVR", "FERG", "FNV", "FUTU", "GFI", "HEI", "HIMS", "HMY", "HOOD", "HUBS", "IBKR", "INTU", "IOT", "IVZ", "JBL", "JPM", "KGC", "LEVI", "NGG", "NTRS", "PAAS", "PEGA", "RACE", "RGLD", "SEIC", "SHOP", "TD", "VIPS", "VOD", "VRNA", "WDC", "XP"},
	"2025-07-25": {"AEM", "AFRM", "AMZN", "ANET", "APG", "APH", "AU", "B", "BBVA", "BEN", "CCL", "COHR", "CRDO", "CYBR", "DASH", "DB", "DOCS", "EVR", "FERG", "FNV", "FUTU", "GE", "GFI", "GMAB", "HIMS", "HOOD", "HUBS", "IBKR", "JBL", "JPM", "KGC", "MANH", "MCHP", "MTZ", "NFLX", "NGG", "NTRS", "PAAS", "RACE", "SCHW", "SEIC", "SFM", "SHG", "SHOP", "THC", "TROW", "VIV", "VOD", "WDC"},
	"2025-08-01": {"AEM", "ALLE", "ANET", "APG", "APH", "AS", "B", "CCL", "COHR", "CRDO", "DASH", "DB", "DOCS", "EVR", "FERG", "FIX", "FUTU", "GE", "GFI", "GMAB", "HAS", "HIMS", "HOOD", "HSBC", "IBKR", "JBL", "JPM", "KGC", "KOF", "LTM", "MANH", "MEDP", "META", "NEM", "NFLX", "NGG", "NTRS", "OKTA", "PEGA", "RACE", "ROKU", "SCHW", "SEIC", "SHG", "SHOP", "TEL", "THC", "TROW", "WDC", "WST"},
	"2025-08-08": {"AEM", "ALLE", "APG", "APH", "AXON", "B", "CCL", "CELH", "CF", "CLS", "COHR", "CRS", "DASH", "FFIV", "FIX", "FUTU", "GE", "GEHC", "GFI", "GLW", "GMAB", "HLI", "HOOD", "HPE", "HSBC", "HWM", "IBKR", "ING", "JPM", "KGC", "LECO", "LTM", "MANH", "MTZ", "NEM", "NFLX", "NGG", "NTRS", "PTC", "RACE", "RDDT", "SCHW", "SHG", "SHOP", "TEL", "THC", "TLN", "TROW", "WDC", "WST"},
	"2025-08-15": {"AEM", "ALAB", "ANET", "APG", "APH", "APP", "C", "CCL", "CELH", "CF", "CLS", "COHR", "EVR", "FFIV", "FIX", "FUTU", "FWONK", "GE", "GEHC", "GFI", "GLW", "HLI", "HOOD", "HSBC", "HWM", "IBKR", "JBL", "JPM", "KGC", "LTM", "MANH", "MEDP", "MTZ", "NEM", "NGG", "NVT", "PTC", "QXO", "RDDT", "RL", "SCHW", "SHG", "SUZ", "TEL", "THC", "TROW", "UBS", "VRT", "WDC", "WST"},
	"2025-08-22": {"AEM", "ALAB", "ALLE", "APG", "APH", "APP", "C", "CELH", "CLS", "COHR", "E", "EVR", "FFIV", "FIX", "FWONA", "FWONK", "GE", "GFI", "GS", "HAS", "HLI", "HOOD", "HPE", "HSBC", "IBKR", "JPM", "KGC", "LTM", "MANH", "MEDP", "MTZ", "MU", "NTR", "PBR", "PTC", "RDDT", "SCHW", "SEIC", "SF", "SHG", "SN", "STT", "SUZ", "THC", "TLN", "TROW", "UBS", "WDC", "WST", "WWD"},
	"2025-08-29": {"ADI", "AEM", "AIZ", "ALAB", "APH", "APP", "AVAV", "BZ", "CELH", "CLS", "CNQ", "FFIV", "FIX", "GE", "GEHC", "GLW", "HAS", "HLI", "HOOD", "HPE", "HSBC", "HWM", "IBKR", "IPG", "KGC", "LEVI", "LITE", "LTM", "MANH", "MEDP", "MOS", "MTZ", "MU", "OSK", "PTC", "RDDT", "RYAAY", "SEIC", "SHG", "SKX", "SN", "TEL", "THC", "TME", "TROW", "UBS", "WDC", "WST", "WTS", "XP"},
	"2025-09-05": {"AIZ", "ALAB", "APG", "APH", "APP", "AVAV", "BZ", "CELH", "CLS", "CRS", "CRWD", "DDS", "FFIV", "FIX", "GE", "GFI", "HAS", "HLI", "HOOD", "HSBC", "IBKR", "IMO", "IPG", "KT", "LEVI", "LITE", "LTM", "MANH", "MOS", "MTZ", "MU", "OSK", "PDD", "PTC", "RDDT", "RYAAY", "SBS", "SHG", "SKX", "SN", "SRAD", "TCOM", "THC", "TME", "UBS", "WDC", "WST", "WTS", "XP", "ZM"},
	"2025-09-12": {"ADI", "AEM", "AIZ", "ALAB", "APH", "APP", "BBVA", "BZ", "CELH", "CIEN", "CLS", "CRDO", "CRWD", "DOCU", "DUOL", "EVR", "FFIV", "FIX", "FSV", "GE", "HAS", "HLI", "HOOD", "HSBC", "IBKR", "IPG", "LITE", "LTM", "MANH", "MEDP", "MOS", "MU", "PDD", "PTC", "RDDT", "RYAAY", "SBS", "SHG", "SN", "STRL", "THC", "TLN", "TME", "UBS", "UI", "WDC", "WST", "WTS", "ZM"},
	"2025-09-19": {"ADI", "AIZ", "ALAB", "APH", "APP", "BZ", "CELH", "CIB", "CIEN", "CLS", "CRDO", "CRWD", "DOCU", "DUOL", "EME", "EVR", "FFIV", "FIX", "GFI", "HAS", "HLI", "HOOD", "HSBC", "IPG", "LITE", "LTM", "MANH", "MEDP", "MU", "NCLH", "ONC", "PDD", "PTC", "RDDT", "RL", "RYAAY", "SBS", "SHG", "SN", "SOLV", "STRL", "TEL", "THC", "TME", "UBS", "UI", "WDAY", "WST", "WTS", "ZM"},
	"2025-09-26": {"ADI", "AEM", "AIZ", "ALAB", "APH", "APP", "ARGX", "AWI", "BZ", "CELH", "CIEN", "CLS", "CNQ", "CRDO", "CRS", "CRWD", "DDS", "DOCU", "DUOL", "EME", "EVR", "FFIV", "FIX", "FUTU", "GEHC", "HALO", "HOOD", "HPE", "HSBC", "IDCC", "LECO", "LITE", "LTM", "MAS", "MTZ", "MU", "NEM", "NVT", "PDD", "PTC", "RDDT", "SFD", "SN", "SOLV", "STRL", "TME", "UI", "WDAY", "WTS", "ZM"},
	"2025-10-03": {"ADI", "AEG", "AEM", "AIZ", "ALAB", "ANET", "APP", "BAP", "BCS", "BZ", "CELH", "CIB", "CIEN", "CNQ", "CRDO", "CRWD", "CTVA", "DOCU", "DUOL", "EVR", "FUTU", "FWONA", "GFI", "GMAB", "HALO", "HESM", "HOOD", "JEF", "LITE", "MASI", "MTZ", "MU", "NCLH", "NVT", "PATH", "PDD", "PRIM", "PSX", "SNX", "SOLV", "STRL", "STX", "TIGO", "TME", "UI", "WDAY", "WDC", "WTS", "ZBRA", "ZM"},
	"2025-10-10": {"ADI", "AEG", "AGI", "ALB", "AU", "AYI", "BAP", "BE", "BZ", "CCL", "CDE", "CIEN", "CNQ", "CRDO", "CRWD", "DINO", "DOCU", "EXAS", "FICO", "FOX", "HOOD", "IDCC", "JEF", "LITE", "LNG", "MDB", "MU", "MUFG", "NCLH", "NMR", "NTR", "ONC", "PATH", "PDD", "PGR", "RDDT", "RIO", "SBS", "SMFG", "SNX", "STRL", "STT", "STX", "TCOM", "TKO", "TME", "UI", "WDAY", "WDC", "ZM"},
	"2025-10-17": {"ADI", "AER", "APH", "AU", "AYI", "BBVA", "BE", "CCL", "CDE", "CIEN", "CNQ", "CRDO", "CRWD", "DELL", "DOCU", "EXAS", "FICO", "FOX", "HOOD", "ITUB", "LNG", "LVS", "MBLY", "MDB", "MU", "MUFG", "NBIX", "NCLH", "NMR", "NRG", "ONC", "PDD", "PODD", "RDDT", "SBS", "SHOP", "SMFG", "SNX", "SONY", "STRL", "STT", "STX", "TCOM", "TIMB", "TROW", "UI", "W", "WDAY", "WDC", "ZM"},
	"2025-10-24": {"AMX", "AMZN", "APH", "AS", "ASML", "AU", "BE", "CCL", "CIB", "CIEN", "CNQ", "CRDO", "CRWD", "CVE", "DOCU", "EXAS", "FICO", "FIX", "FN", "FNV", "FOX", "GFI", "IBKR", "KGC", "LNG", "LRCX", "LVS", "MDB", "META", "MS", "MU", "MUFG", "NEM", "NMR", "ONC", "PDD", "PODD", "RDDT", "RY", "SAN", "SBS", "SHOP", "SMFG", "SNX", "SONY", "TCOM", "TROW", "TRV", "UI", "WDC"},
	"2025-10-31": {"AMX", "APH", "AS", "ASML", "AU", "CCL", "CIB", "CIEN", "CNC", "CNQ", "COF", "CRDO", "CVE", "DOCU", "EWBC", "FICO", "FIX", "FNV", "FOX", "FUTU", "GFI", "GLW", "GM", "HCA", "IBKR", "KGC", "LRCX", "LVS", "MCK", "MEDP", "MS", "MU", "MUFG", "NEM", "NWG", "RDDT", "SAN", "SBS", "SHOP", "SMFG", "SNX", "STX", "TER", "TROW", "TRV", "TTWO", "UI", "VRT", "WDC", "WYNN"},
	"2025-11-07": {"AER", "AMX", "APH", "AS", "ASML", "AU", "BCH", "CBOE", "CCL", "CIEN", "CLS", "COF", "CVE", "DOCU", "EWBC", "EXAS", "FICO", "FIX", "FN", "FNV", "FOX", "FOXA", "FUTU", "GFI", "GLW", "GM", "GWRE", "HCA", "HSBC", "KGC", "LRCX", "LVS", "MS", "MU", "NBIX", "NEM", "NVT", "RDDT", "SAN", "SATS", "SF", "SNX", "STX", "TER", "TROW", "TRV", "UBS", "UHS", "VRT", "WDC"},
	"2025-11-14": {"AER", "APH", "AS", "AU", "CBOE", "CCL", "CIEN", "CLS", "CNQ", "EXAS", "EXPD", "FIX", "FN", "FOX", "FOXA", "FUTU", "GFI", "GM", "GMED", "GWRE", "HOOD", "HTHT", "JEF", "KGC", "LITE", "LVS", "MNST", "MS", "MU", "NBIX", "NEM", "RDDT", "RGLD", "RL", "SCCO", "SGI", "SNX", "SOLV", "STX", "SUZ", "TER", "TRMB", "TROW", "TRV", "TTWO", "UBS", "UHS", "VALE", "VRT", "WDC"},
	"2025-11-21": {"AER", "ALL", "APH", "AU", "CBOE", "CIEN", "CLS", "CNQ", "DB", "EXPD", "EXPE", "FIX", "FN", "FOX", "FOXA", "FUTU", "GFI", "GM", "GMED", "GWRE", "HOOD", "IVZ", "JEF", "JHX", "KGC", "LITE", "LVS", "LW", "MNST", "MS", "MU", "NBIX", "ONON", "RDDT", "SANM", "SBS", "SCCO", "SNDK", "SNX", "STRL", "STX", "TIGO", "TIMB", "TROW", "TRV", "TTWO", "UHS", "VALE", "VRT", "WDC"},
	"2025-11-28": {"AEM", "AER", "ALL", "APH", "AS", "AU", "CIB", "CLS", "CNQ", "DB", "EXPD", "EXPE", "FIX", "FNV", "FOX", "FOXA", "FUTU", "GFI", "GM", "GMED", "HOOD", "ISRG", "IVZ", "JEF", "KGC", "LITE", "LVS", "MNST", "MS", "MU", "NBIX", "NEM", "NVDA", "ONON", "PAAS", "RDDT", "RGLD", "SCCO", "SNDK", "STX", "TIMB", "TROW", "TRV", "TTWO", "UHS", "UI", "VALE", "VRT", "WDC", "WPM"},
	"2025-12-05": {"AA", "AEM", "ALL", "APH", "AS", "AU", "BG", "CIB", "CLS", "CRDO", "DB", "DDS", "DY", "EXPD", "EXPE", "FIX", "FNV", "FOX", "FOXA", "FUTU", "GFI", "HOOD", "ILMN", "ISRG", "KGC", "LITE", "LVS", "MDB", "MNST", "MS", "MU", "NBIX", "NEM", "NVDA", "ONON", "RDDT", "RGLD", "SCCO", "SNDK", "STX", "TCOM", "TIMB", "TROW", "TRV", "UHS", "UI", "VALE", "VRT", "WDC", "ZTO"},
	"2025-12-12": {"AEM", "AER", "ALL", "APH", "AS", "AU", "BG", "BHP", "CIB", "CLS", "COHR", "CRDO", "DDS", "DY", "EXPD", "EXPE", "FIVE", "FIX", "FOX", "FOXA", "GFI", "GM", "HOOD", "ILMN", "ISRG", "KGC", "LITE", "LVS", "MDB", "MNST", "MRVL", "MS", "MU", "NBIX", "NEM", "NVDA", "ONON", "RDDT", "RGLD", "RNR", "SANM", "SNDK", "STRL", "TCOM", "TIGO", "TRV", "UHS", "UI", "WDC", "ZTO"},
	"2025-12-19": {"AEM", "AER", "ALL", "APH", "AS", "AU", "B", "BHP", "CIB", "CIEN", "CLS", "COF", "COHR", "CRDO", "CVE", "EL", "EXPD", "EXPE", "FIX", "FOX", "FOXA", "GFI", "GM", "HOOD", "ILMN", "ISRG", "KGC", "LITE", "LVS", "MDB", "MNST", "MRVL", "MS", "MU", "NBIX", "NVDA", "ONON", "PAAS", "PSX", "RDDT", "RGLD", "SCCO", "SNDK", "STX", "TCOM", "TRMB", "UBS", "UI", "WDC", "ZTO"},
	"2025-12-26": {"AA", "AEM", "AER", "APH", "AS", "CIB", "CIEN", "CLS", "COHR", "CRDO", "CVE", "DDS", "DY", "EL", "EXPD", "EXPE", "FIVE", "FIX", "FOX", "FOXA", "GM", "HOOD", "IAG", "ILMN", "IVZ", "KGC", "LITE", "MDB", "MKL", "MNST", "MU", "NBIX", "ONON", "PAAS", "PSX", "RDDT", "RNR", "SCCO", "SNDK", "STRL", "STX", "SU", "TCOM", "TIMB", "TRMB", "UBS", "UHS", "UI", "WDC", "ZTO"},
	"2026-01-02": {"AA", "ADI", "AEM", "APH", "APP", "BG", "CDE", "CIB", "CIEN", "CNM", "COHR", "CRDO", "CVE", "DDS", "DY", "EPAM", "EXAS", "EXPD", "EXPE", "FIVE", "FUTU", "GAP", "GM", "HOOD", "KGC", "LITE", "LLY", "MDB", "MNST", "MTZ", "MU", "ONON", "PAA", "PAAS", "PLTR", "PODD", "PSX", "RNR", "SGI", "SNDK", "STX", "SUN", "TCOM", "TIGO", "UI", "ULTA", "VALE", "WDC", "ZM", "ZTO"},
	"2026-01-09": {"AA", "ADI", "AEM", "APH", "BAP", "BG", "BHP", "CDE", "CIB", "CIEN", "CM", "CNM", "CRDO", "CVE", "CYBR", "DDS", "EXAS", "FIGR", "FIVE", "FUTU", "GAP", "GM", "HSBC", "IOT", "KGC", "LLY", "MDB", "MPC", "MRVL", "MS", "MU", "NVDA", "ONON", "PAAS", "PSX", "REGN", "RELX", "RGLD", "RIO", "SNDK", "SONY", "STX", "SUZ", "TCOM", "TIMB", "UI", "ULTA", "VALE", "WDC", "ZM"},
	"2026-01-16": {"AA", "ADI", "AEM", "ALB", "AS", "BAP", "BG", "BHP", "CASY", "CCJ", "CDE", "CF", "CIB", "CIEN", "CNM", "CRDO", "CVE", "CYBR", "DDS", "DG", "DY", "EL", "EXPE", "FIGR", "FIVE", "FUTU", "GM", "IOT", "ITUB", "KGC", "LTM", "MDB", "MU", "ONTO", "PAAS", "REGN", "RELX", "RGLD", "RIO", "ROST", "SNDK", "STX", "SUZ", "TCOM", "UI", "ULTA", "VALE", "WDC", "WWD", "ZM"},
	"2026-01-23": {"AA", "ADI", "AEM", "ALB", "APH", "BBVA", "BG", "BHP", "BIDU", "BWXT", "C", "CASY", "CCJ", "CDE", "CIEN", "CNM", "CRDO", "CTVA", "CYBR", "DG", "F", "FHN", "FIGR", "FIVE", "FUTU", "GM", "IOT", "ITUB", "JHX", "KGC", "KLAC", "LRCX", "MDB", "MS", "MU", "NVDA", "PAA", "REGN", "RELX", "RGLD", "RIO", "SQM", "SUZ", "TCOM", "TTMI", "ULTA", "VALE", "WDC", "WWD", "ZM"},
	"2026-01-30": {"AA", "ALB", "AMKR", "APH", "AU", "BBVA", "BHP", "C", "CASY", "CBOE", "CCJ", "CDE", "CIEN", "CM", "CRDO", "CTVA", "DG", "F", "FUTU", "GE", "HSY", "IBKR", "IOT", "ISRG", "ITUB", "IVZ", "KLAC", "LRCX", "MCHP", "MDB", "MKSI", "MS", "MU", "NTR", "NTRS", "REGN", "RELX", "RIO", "RY", "RYAAY", "SNDK", "SQM", "STX", "SUZ", "TCOM", "TEL", "ULTA", "VALE", "WDC", "WPM"},
	"2026-02-06": {"AA", "ALB", "AMKR", "AVGO", "BHP", "C", "CDE", "CIEN", "CLS", "CNM", "CX", "DECK", "EDU", "EXEL", "F", "FIVE", "FN", "FUTU", "GE", "GFI", "IBKR", "ING", "ISRG", "IVZ", "KLAC", "LITE", "LRCX", "LTM", "LUV", "MCHP", "MDB", "MKSI", "MOD", "MU", "NTR", "NTRS", "ONON", "RELX", "RIO", "RYAAY", "SHG", "SNDK", "STX", "TEL", "TER", "TSM", "TTMI", "UMBF", "WDC", "WWD"},
	"2026-02-13": {"AA", "ALB", "ATI", "AU", "BBVA", "BHP", "CAH", "CASY", "CBOE", "CCL", "CDE", "CLS", "COHR", "CRDO", "DD", "DECK", "FOX", "GE", "GFI", "HSY", "ING", "ISRG", "KLAC", "LITE", "LRCX", "LSCC", "LTM", "LUV", "MDB", "MKSI", "MPWR", "MS", "MSTR", "MU", "NTR", "NTRS", "NWS", "ONON", "SATS", "SNDK", "SQM", "STX", "SUZ", "TEL", "TER", "TPR", "TSM", "VIV", "WDC", "WWD"},
	"2026-02-20": {"ADI", "ALB", "AU", "BAM", "BBVA", "BCS", "BHP", "BMO", "CBOE", "CLS", "CRDO", "DD", "F", "FDX", "FTI", "FTV", "GE", "HSY", "ING", "ISRG", "KB", "KLAC", "LITE", "LUV", "MDB", "MPWR", "MSTR", "MU", "NTRS", "NWG", "ONON", "PHG", "RBC", "RIO", "SATS", "SHG", "SNDK", "SQM", "STX", "TD", "TEL", "TER", "TPR", "TSM", "VIV", "VRT", "WDC", "WF", "WPM", "WWD"},
	"2026-02-27": {"AG", "ALB", "ALL", "ATI", "BAM", "BBVA", "BHP", "BWXT", "CBOE", "CRDO", "DD", "DECK", "EQX", "EXPE", "FIX", "FTI", "FTV", "GRMN", "HSY", "ING", "ISRG", "JLL", "LITE", "LUV", "MDB", "MSTR", "MU", "NTRS", "NVDA", "NWG", "NWS", "ONON", "PHG", "RIO", "RL", "SNDK", "SQM", "STX", "SUZ", "TD", "TEL", "TER", "TPR", "TSM", "UI", "USFD", "VIV", "WDC", "WPM", "WWD"},
}
