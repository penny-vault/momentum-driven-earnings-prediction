// Copyright 2021-2026
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	_ "embed"
	"math"
	"sort"
	"time"

	"github.com/penny-vault/pvbt/asset"
	"github.com/penny-vault/pvbt/data"
	"github.com/penny-vault/pvbt/engine"
	"github.com/penny-vault/pvbt/portfolio"
	"github.com/penny-vault/pvbt/tradecron"
	"github.com/penny-vault/pvbt/universe"
	"github.com/rs/zerolog"
)

//go:embed README.md
var description string

// MomentumDrivenEarningsPrediction selects stocks ranked by Zacks Investment
// Research earnings predictions. It includes a momentum-based crash protection
// that exits to a safe-haven asset when market sentiment goes negative.
//
// BLOCKED: This strategy requires pvbt to expose a ZacksRank metric (or
// equivalent) so that the strategy can filter a broad universe to only
// stocks with Zacks rank == 1. See the Compute method for details.
type MomentumDrivenEarningsPrediction struct {
	NumHoldings int               `pvbt:"num-holdings" desc:"Maximum number of stocks to hold" default:"50"`
	Indicator   string            `pvbt:"indicator" desc:"Risk-on/off indicator: None or Momentum" default:"None"`
	OutTicker   universe.Universe `pvbt:"out-ticker" desc:"Safe-haven asset when sentiment is negative" default:"VUSTX"`
	Period      string            `pvbt:"period" desc:"Rebalancing frequency: Weekly or Monthly" default:"Weekly"`
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
		Description: description,
		Source:      "",
		Version:     "1.0.0",
		VersionDate: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
	}
}

// riskOn returns true when the momentum-based risk indicator is positive.
// It computes the average risk-adjusted momentum (1/3/6-month) of VFINX and
// PRIDX and returns true if the max score across both is > 0.
func (s *MomentumDrivenEarningsPrediction) riskOn(ctx context.Context, e *engine.Engine) (bool, error) {
	if s.Indicator != "Momentum" {
		return true, nil
	}

	log := zerolog.Ctx(ctx)

	vfinx := e.Asset("VFINX")
	pridx := e.Asset("PRIDX")
	dgs3mo := e.Asset("DGS3MO")

	indicatorUniverse := e.Universe(vfinx, pridx)
	riskFreeUniverse := e.Universe(dgs3mo)

	priceDF, err := indicatorUniverse.Window(ctx, portfolio.Months(6), data.MetricClose)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch indicator prices")
		return false, err
	}

	riskFreeDF, err := riskFreeUniverse.Window(ctx, portfolio.Months(6), data.MetricClose)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch risk-free rate")
		return false, err
	}

	prices := priceDF.Downsample(data.Monthly).Last()
	riskFree := riskFreeDF.Downsample(data.Monthly).Last()

	if prices.Len() < 7 {
		return true, nil
	}

	// Compute risk-adjusted momentum for 1, 3, 6 month periods.
	rfCol := riskFree.Column(dgs3mo, data.MetricClose)

	riskAdjMom := func(n int) *data.DataFrame {
		mom := prices.Pct(n).MulScalar(100)
		rfSum := riskFree.Rolling(n).Sum().DivScalar(12)
		_ = rfCol // used via rfSum
		return mom.Apply(func(col []float64) []float64 {
			out := make([]float64, len(col))
			rfSumCol := rfSum.Column(dgs3mo, data.MetricClose)
			for i := range col {
				out[i] = col[i] - rfSumCol[i]
			}
			return out
		})
	}

	ram1 := riskAdjMom(1)
	ram3 := riskAdjMom(3)
	ram6 := riskAdjMom(6)

	score := ram1.Add(ram3).Add(ram6).DivScalar(3)
	score = score.Drop(math.NaN()).Last()

	if score.Len() == 0 {
		return true, nil
	}

	// Max score across indicator assets determines risk-on/off.
	maxScore := math.Inf(-1)
	for _, a := range score.AssetList() {
		v := score.Value(a, data.MetricClose)
		if v > maxScore {
			maxScore = v
		}
	}

	return maxScore > 0, nil
}

func (s *MomentumDrivenEarningsPrediction) Compute(ctx context.Context, e *engine.Engine, p portfolio.Portfolio) {
	log := zerolog.Ctx(ctx)

	// Step 1: Check risk indicator.
	isRiskOn, err := s.riskOn(ctx, e)
	if err != nil {
		return
	}

	// Step 2: If risk-off, shift entirely to out-of-market ticker.
	if !isRiskOn {
		outDF, err := s.OutTicker.At(ctx, e.CurrentDate(), data.MetricClose)
		if err != nil {
			log.Error().Err(err).Msg("failed to fetch out-ticker data")
			return
		}

		outAssets := outDF.AssetList()
		if len(outAssets) == 0 {
			return
		}

		alloc := portfolio.Allocation{
			Date:    e.CurrentDate(),
			Members: map[asset.Asset]float64{outAssets[0]: 1.0},
		}
		if err := p.RebalanceTo(ctx, alloc); err != nil {
			log.Error().Err(err).Msg("rebalance to out-ticker failed")
		}
		return
	}

	// Step 3: Select stocks with Zacks rank == 1, sorted by market cap.
	//
	// PVBT TEAM: This step requires a way to query rated/ranked stocks.
	// The legacy implementation queried:
	//   SELECT composite_figi FROM zacks_financials
	//   WHERE zacks_rank=1 AND event_date=$1
	//   ORDER BY market_cap_mil DESC LIMIT $2
	//
	// Possible pvbt approaches:
	//   a) Add a ZacksRank metric to data.Metric and expose it through
	//      the PVDataProvider so strategies can fetch it as a metric.
	//   b) Add a RankedUniverse that accepts a rating source and threshold,
	//      returning time-varying membership based on ranking data.
	//   c) Expose a generic query interface for the ratings table.
	//
	// Once available, the code below would become:
	//   rankedDF, err := zacksUniverse.At(ctx, e.CurrentDate(), data.MarketCap)
	//   // sort by MarketCap descending, take top NumHoldings
	//   // equal-weight and rebalance

	log.Error().Msg("MDEP: Zacks rank data not yet available in pvbt -- " +
		"pvbt needs a ZacksRank metric or ranked universe provider")

	// Placeholder: when Zacks data becomes available, select and rebalance.
	// selectAndRebalance(ctx, e, p, s.NumHoldings)
}

// selectAndRebalance selects the top-N stocks by market cap from a set of
// ranked assets and rebalances to an equal-weight allocation.
// This is ready to use once the ranked asset list is available.
func selectAndRebalance(ctx context.Context, p portfolio.Portfolio, date time.Time, assets []asset.Asset, marketCaps *data.DataFrame, numHoldings int) error {
	type assetCap struct {
		Asset     asset.Asset
		MarketCap float64
	}

	ranked := make([]assetCap, 0, len(assets))
	for _, a := range assets {
		cap := marketCaps.Value(a, data.MarketCap)
		if !math.IsNaN(cap) {
			ranked = append(ranked, assetCap{Asset: a, MarketCap: cap})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].MarketCap > ranked[j].MarketCap
	})

	if len(ranked) > numHoldings {
		ranked = ranked[:numHoldings]
	}

	if len(ranked) == 0 {
		return nil
	}

	weight := 1.0 / float64(len(ranked))
	members := make(map[asset.Asset]float64, len(ranked))
	for _, r := range ranked {
		members[r.Asset] = weight
	}

	return p.RebalanceTo(ctx, portfolio.Allocation{
		Date:    date,
		Members: members,
	})
}
