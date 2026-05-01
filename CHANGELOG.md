# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.3] - 2026-05-01

### Changed
- Upgrade pvbt dependency to v0.8.1
- Risk-free rate ticker now uses the `FRED:` prefix (`FRED:DGS3MO`) to match pvbt's data-source routing

## [0.1.2] - 2026-04-25

### Changed
- Upgrade pvbt dependency to v0.8.0
- Regenerate testdata snapshot for pvbt's v5 snapshot schema and refresh expected TWRR/value/holdings to match current pv-data

## [0.1.1] - 2026-04-23

### Changed
- Upgrade pvbt dependency to v0.7.7

## [0.1.0] - 2026-04-21

### Added
- Initial release of Momentum Driven Earnings Prediction (MDEP) strategy
- Momentum-based crash protection signal that switches to an out-of-market asset when the signal falls below zero
- Snapshot tests validating allocation output against reference backtest data

[0.1.0]: https://github.com/penny-vault/momentum-driven-earnings-prediction/releases/tag/v0.1.0
[0.1.1]: https://github.com/penny-vault/momentum-driven-earnings-prediction/compare/v0.1.0...v0.1.1
[0.1.2]: https://github.com/penny-vault/momentum-driven-earnings-prediction/compare/v0.1.1...v0.1.2
[0.1.3]: https://github.com/penny-vault/momentum-driven-earnings-prediction/compare/v0.1.2...v0.1.3
