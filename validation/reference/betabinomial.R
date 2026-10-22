# The beta-binomial test and its variance model (Rytlewski et al. 2019), computed independently for
# validation/betabinomial_test.go, from the model's definition (no code of Rytlewski et al.'s):
# k of n is beta-binomial with mean θ and scale v, α = θv, β = (1 − θ)v, its probabilities
# choose(n, k) B(k + α, n − k + β) / B(α, β); the scale v(n) = min(2 + (b ln n + a)², 10⁷).
# - p-values on 150 cases: two-sided, summing the outcomes no more likely than k (with R's relative
#   tolerance of 10⁻⁷), and one-sided ("less": at most k, ClonoLoom's "increase"; "greater");
# - two sets of clonotypes drawn from the model with known (a, b), as two samples of one repertoire
#   would give them: the log-likelihood at (1, 1) and at the truth, and (a, b) fitted by maximum
#   likelihood (Nelder–Mead from (1, 1), then BFGS from its result), b made positive.
#
#   Rscript validation/reference/betabinomial.R
suppressMessages(library(jsonlite))
args <- commandArgs(trailingOnly = FALSE)
here <- dirname(normalizePath(sub("^--file=", "", args[grep("^--file=", args)])))

ldbb <- function(k, n, theta, v) {
  a <- theta * v; b <- (1 - theta) * v
  lchoose(n, k) + lbeta(k + a, n - k + b) - lbeta(a, b)
}
scale <- function(n, a, b) pmin(2 + (b * log(n) + a)^2, 1e7)
pvalues <- function(k, n, theta, v) {
  d <- exp(ldbb(0:n, n, theta, v))
  d <- d / sum(d)
  dk <- d[k + 1]
  c(sum(d[d <= dk * (1 + 1e-7)]), sum(d[1:(k + 1)]), sum(d[(k + 1):(n + 1)]))
}

set.seed(20261023)
cases <- lapply(1:150, function(i) {
  n <- sample(c(1, 5, 12, 40, 200, 1500, 8000), 1)
  theta <- sample(c(0.2, 0.45, 0.6), 1)
  v <- sample(c(2.5, 10, 80, 1e4), 1)
  k <- min(n, max(0, round(n * theta + sample(c(-3, -1, 0, 1, 2), 1) * sqrt(n * theta * (1 - theta) * (1 + n / v)))))
  list(k = k, n = n, theta = theta, v = v, p = pvalues(k, n, theta, v))
})

draw <- function(m, a, b, theta) {
  n <- pmin(round(5 / runif(m)^1.2), 20000)
  v <- scale(n, a, b)
  p <- rbeta(m, theta * v, (1 - theta) * v)
  k <- rbinom(m, n, p)
  agg <- aggregate(list(count = rep(1, m)), by = list(k = k, n = n), FUN = sum)
  agg[order(agg$n, agg$k), ]
}
fit <- function(obs, theta, a, b) {
  nll <- function(par) -sum(obs$count * ldbb(obs$k, obs$n, theta, scale(obs$n, par[1], par[2])))
  first <- optim(c(1, 1), nll, method = "Nelder-Mead", control = list(reltol = 1e-14, maxit = 20000))
  best <- optim(first$par, nll, method = "BFGS", control = list(reltol = 1e-15, maxit = 2000))
  par <- best$par
  if (par[2] < 0) par <- -par
  list(theta = theta, truth = list(a = a, b = b),
       observations = lapply(seq_len(nrow(obs)), function(i) list(k = obs$k[i], n = obs$n[i], count = obs$count[i])),
       logLikAtOne = -nll(c(1, 1)), logLikAtTruth = -nll(c(a, b)),
       fitted = list(a = par[1], b = par[2], logLik = -best$value))
}
sets <- list(fit(draw(3000, 1.5, 0.5, 0.45), 0.45, 1.5, 0.5), fit(draw(3000, 0.3, 1.2, 0.55), 0.55, 0.3, 1.2))
writeLines(toJSON(list(made = paste("R", R.version$major, R.version$minor, sep = "."), cases = cases, sets = sets), digits = I(17), auto_unbox = TRUE),
           file.path(here, "betabinomial.json"))
for (s in sets) cat(sprintf("truth (%.2f, %.2f): fitted (%.4f, %.4f), log-likelihood %.4f\n", s$truth$a, s$truth$b, s$fitted$a, s$fitted$b, s$fitted$logLik))
