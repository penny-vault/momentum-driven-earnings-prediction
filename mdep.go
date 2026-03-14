package main

import (
	"context"
	"time"

	"github.com/penny-vault/pvbt/engine"
	"github.com/penny-vault/pvbt/portfolio"
	"github.com/penny-vault/pvbt/tradecron"
	"github.com/penny-vault/pvbt/universe"
	"github.com/rs/zerolog"
)

type MomentumDrivenEarningsPrediction struct {
	NumHoldings int               `pvbt:"num-holdings" desc:"Maximum number of stocks to hold" default:"50"`
	OutTicker   universe.Universe `pvbt:"out-ticker" desc:"Safe-haven asset when sentiment is negative" default:"VUSTX"`
	Period      string            `pvbt:"period" desc:"Rebalancing frequency" default:"Weekly"`
}

func (s *MomentumDrivenEarningsPrediction) Name() string {
	return "Momentum Driven Earnings Prediction"
}

func (s *MomentumDrivenEarningsPrediction) Setup(e *engine.Engine) {
	schedule := "@weekend"
	if s.Period == "Monthly" {
		schedule = "@monthend"
	}
	tc, err := tradecron.New(schedule, tradecron.MarketHours{Open: 930, Close: 1600})
	if err != nil {
		panic(err)
	}
	e.Schedule(tc)
	e.SetBenchmark(e.Asset("VFINX"))
	e.RiskFreeAsset(e.Asset("DGS3MO"))
}

func (s *MomentumDrivenEarningsPrediction) Describe() engine.StrategyDescription {
	return engine.StrategyDescription{
		ShortCode:   "mdep",
		Description: "Invests in stocks ranked by earnings predictions from Zacks Investment Research with crash protection.",
		Source:      "",
		Version:     "1.0.0",
		VersionDate: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
	}
}

func (s *MomentumDrivenEarningsPrediction) Compute(ctx context.Context, e *engine.Engine, p portfolio.Portfolio) {
	log := zerolog.Ctx(ctx)

	// TODO: This strategy requires access to a Zacks financial database
	// (zacks_financials table) to select stocks ranked by earnings predictions.
	// The pvbt framework needs a mechanism for strategies to query external
	// databases before this can be fully implemented.
	//
	// Algorithm outline:
	// 1. Query zacks_financials for stocks with zacks_rank=1, ordered by market_cap_mil DESC
	// 2. Select top N holdings, equal-weight
	// 3. Optionally apply momentum-based risk indicator (1/3/6-month momentum on VFINX, PRIDX)
	// 4. If risk-off, shift 100% to OutTicker
	// 5. Rebalance

	log.Warn().Msg("MDEP strategy requires Zacks database access - not yet implemented")
}
