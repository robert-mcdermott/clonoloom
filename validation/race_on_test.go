//go:build race

package validation

// raceDetector: the tests run with the race detector, which multiplies their time tenfold, so
// budgets are measured only without it (CI's data job).
const raceDetector = true
