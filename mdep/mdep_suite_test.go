package mdep_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMomentumDrivenEarningsPrediction(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Momentum Driven Earnings Prediction Suite")
}
