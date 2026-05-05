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

package mdep

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/penny-vault/pvbt/asset"
	"github.com/penny-vault/pvbt/data"
	"github.com/penny-vault/pvbt/engine"
	"github.com/penny-vault/pvbt/portfolio"
	"github.com/penny-vault/pvbt/universe"
)

//go:embed README.md
var description string

// MomentumDrivenEarningsPrediction selects stocks ranked by Zacks Investment
// Research earnings predictions. It includes a momentum-based crash protection
// that exits to a safe-haven asset when market sentiment goes negative.
type MomentumDrivenEarningsPrediction struct {
	NumHoldings int               `pvbt:"num-holdings" desc:"Maximum number of stocks to hold" default:"50"`
	Indicator   string            `pvbt:"indicator" desc:"Risk-on/off indicator: None or Momentum" default:"None"`
	OutTicker   universe.Universe `pvbt:"out-ticker" desc:"Safe-haven asset when sentiment is negative" default:"VUSTX"`
	Period      string            `pvbt:"period" desc:"Rebalancing frequency: Weekly or Monthly" default:"Weekly"`
}

func (s *MomentumDrivenEarningsPrediction) Name() string {
	return "Momentum Driven Earnings Prediction"
}

func (s *MomentumDrivenEarningsPrediction) Setup(_ *engine.Engine) {}

func (s *MomentumDrivenEarningsPrediction) Describe() engine.StrategyDescription {
	schedule := "@weekend"
	if s.Period == "Monthly" {
		schedule = "@monthend"
	}

	return engine.StrategyDescription{
		ShortCode:   "mdep",
		Description: description,
		Source:      "",
		Version:     "1.0.0",
		VersionDate: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
		Schedule:    schedule,
		Benchmark:   "SPY",
	}
}

// riskOn returns true when the momentum-based risk indicator is positive.
// It computes the average risk-adjusted momentum (1/3/6-month) of VFINX and
// PRIDX and returns true if the max score across both is > 0.
func (s *MomentumDrivenEarningsPrediction) riskOn(ctx context.Context, eng *engine.Engine) (bool, float64, error) {
	if s.Indicator != "Momentum" {
		return true, math.NaN(), nil
	}

	vfinx := eng.Asset("VFINX")
	pridx := eng.Asset("PRIDX")
	dgs3mo := eng.Asset("FRED:DGS3MO")

	indicatorUniverse := eng.Universe(vfinx, pridx)
	riskFreeUniverse := eng.Universe(dgs3mo)

	priceDF, err := indicatorUniverse.Window(ctx, portfolio.Months(6), data.MetricClose)
	if err != nil {
		return false, math.NaN(), fmt.Errorf("fetch indicator prices: %w", err)
	}

	riskFreeDF, err := riskFreeUniverse.Window(ctx, portfolio.Months(6), data.MetricClose)
	if err != nil {
		return false, math.NaN(), fmt.Errorf("fetch risk-free rate: %w", err)
	}

	prices := priceDF.Downsample(data.Monthly).Last()
	riskFree := riskFreeDF.Downsample(data.Monthly).Last()

	if prices.Len() < 7 {
		return true, math.NaN(), nil
	}

	// Compute risk-adjusted momentum for 1, 3, 6 month periods.
	riskAdjMom := func(months int) *data.DataFrame {
		mom := prices.Pct(months).MulScalar(100)
		rfSum := riskFree.Rolling(months).Sum().DivScalar(12)

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
		return true, math.NaN(), nil
	}

	// Max score across indicator assets determines risk-on/off.
	maxScore := math.Inf(-1)

	for _, scoreAsset := range score.AssetList() {
		val := score.Value(scoreAsset, data.MetricClose)
		if val > maxScore {
			maxScore = val
		}
	}

	return maxScore > 0, maxScore, nil
}

func (s *MomentumDrivenEarningsPrediction) Compute(ctx context.Context, eng *engine.Engine, strategyPortfolio portfolio.Portfolio, batch *portfolio.Batch) error {
	// Step 1: Check risk indicator.
	isRiskOn, riskScore, err := s.riskOn(ctx, eng)
	if err != nil {
		return fmt.Errorf("risk indicator: %w", err)
	}

	if !math.IsNaN(riskScore) {
		batch.Annotate("risk-score", fmt.Sprintf("%.4f", riskScore))
	}

	// Step 2: If risk-off, shift entirely to out-of-market ticker.
	if !isRiskOn {
		outDF, err := s.OutTicker.At(ctx, data.MetricClose)
		if err != nil {
			return fmt.Errorf("fetch out-ticker: %w", err)
		}

		outAssets := outDF.AssetList()
		if len(outAssets) == 0 {
			return nil
		}

		alloc := portfolio.Allocation{
			Date:          eng.CurrentDate(),
			Members:       map[asset.Asset]float64{outAssets[0]: 1.0},
			Justification: "risk-off: momentum indicator negative",
		}

		return batch.RebalanceTo(ctx, alloc)
	}

	// Step 3: Get Zacks rank 1 stocks using rated universe.
	zacksUniverse := eng.RatedUniverse("zacks-rank", data.RatingEq(1))

	// Step 4: Fetch market cap for these stocks at the current date.
	mcDF, err := zacksUniverse.At(ctx, data.MarketCap)
	if err != nil {
		return fmt.Errorf("fetch market caps: %w", err)
	}

	// Step 5: Sort by market cap descending, take top N.
	type assetCap struct {
		Asset     asset.Asset
		MarketCap float64
	}

	var ranked []assetCap

	for _, stock := range mcDF.AssetList() {
		mc := mcDF.Value(stock, data.MarketCap)
		if !math.IsNaN(mc) && mc > 0 {
			ranked = append(ranked, assetCap{Asset: stock, MarketCap: mc})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].MarketCap > ranked[j].MarketCap
	})

	if len(ranked) > s.NumHoldings {
		ranked = ranked[:s.NumHoldings]
	}

	if len(ranked) == 0 {
		// No qualifying stocks, go to out-of-market.
		outDF, err := s.OutTicker.At(ctx, data.MetricClose)
		if err != nil {
			return fmt.Errorf("fetch out-ticker fallback: %w", err)
		}

		outAssets := outDF.AssetList()
		if len(outAssets) == 0 {
			return nil
		}

		alloc := portfolio.Allocation{
			Date:          eng.CurrentDate(),
			Members:       map[asset.Asset]float64{outAssets[0]: 1.0},
			Justification: "no qualifying Zacks rank 1 stocks",
		}

		return batch.RebalanceTo(ctx, alloc)
	}

	// Step 6: Equal weight and rebalance.
	weight := 1.0 / float64(len(ranked))
	members := make(map[asset.Asset]float64, len(ranked))

	for _, rc := range ranked {
		members[rc.Asset] = weight
	}

	justification := fmt.Sprintf("risk-on: %d Zacks rank 1 stocks by market cap", len(ranked))
	alloc := portfolio.Allocation{
		Date:          eng.CurrentDate(),
		Members:       members,
		Justification: justification,
	}

	return batch.RebalanceTo(ctx, alloc)
}
