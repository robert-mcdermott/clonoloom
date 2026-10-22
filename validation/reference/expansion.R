# Two-sample tests of a clonotype's counts, computed independently for validation/expansion_test.go:
# for each case (k1 of N1 in the first sample, k2 of N2 in the second), Fisher's exact test of the
# 2×2 table [[k1, k2], [N1 − k1, N2 − k2]] by R's fisher.test, and the binomial test of k1 of
# k1 + k2 at N1 / (N1 + N2) by R's binom.test, each two-sided and one-sided ("less": fewer in the
# first sample than expected, ClonoLoom's "increase"; "greater", its "decrease"); and over the
# cases' two-sided binomial p-values, Benjamini and Hochberg's adjusted p-values (p.adjust) and
# Storey's q-values with π₀ estimated at λ = 0.5 (π₀ = min(1, mean(p >= 0.5) / 0.5), q = π₀ times
# BH's, as the qvalue package computes them at one λ).
#
#   Rscript validation/reference/expansion.R
suppressMessages(library(jsonlite))
args <- commandArgs(trailingOnly = FALSE)
here <- dirname(normalizePath(sub("^--file=", "", args[grep("^--file=", args)])))

fixed <- data.frame(
  k1 = c(0, 5, 3, 10, 1, 50, 400, 1000, 7, 0, 12, 2, 300, 25, 1),
  k2 = c(5, 0, 7, 10, 20, 200, 30, 1500, 0, 9, 12, 2, 0, 60, 1),
  n1 = c(100, 100, 1000, 5000, 12000, 20000, 30000, 100000, 15000, 15000, 10000, 1e6, 18000, 40000, 50),
  n2 = c(100, 100, 1200, 5000, 15000, 15000, 25000, 90000, 20000, 20000, 30000, 1e6, 12000, 20000, 70)
)
set.seed(20261022)
m <- 200
random <- data.frame(
  k1 = rpois(m, rep(c(2, 8, 40, 300), length.out = m)),
  k2 = rpois(m, rep(c(3, 6, 60, 250), length.out = m)) * sample(c(1, 1, 1, 4), m, replace = TRUE),
  n1 = round(10^runif(m, 3.5, 6)),
  n2 = round(10^runif(m, 3.5, 6))
)
cases <- rbind(fixed, random)
cases <- cases[cases$k1 + cases$k2 > 0, ]

fisher <- function(k1, k2, n1, n2, alt) fisher.test(matrix(c(k1, n1 - k1, k2, n2 - k2), 2), alternative = alt)$p.value
binom <- function(k1, k2, n1, n2, alt) binom.test(k1, k1 + k2, n1 / (n1 + n2), alternative = alt)$p.value
out <- lapply(seq_len(nrow(cases)), function(i) {
  c <- cases[i, ]
  list(k1 = c$k1, k2 = c$k2, n1 = c$n1, n2 = c$n2,
       fisher = c(fisher(c$k1, c$k2, c$n1, c$n2, "two.sided"), fisher(c$k1, c$k2, c$n1, c$n2, "less"), fisher(c$k1, c$k2, c$n1, c$n2, "greater")),
       binomial = c(binom(c$k1, c$k2, c$n1, c$n2, "two.sided"), binom(c$k1, c$k2, c$n1, c$n2, "less"), binom(c$k1, c$k2, c$n1, c$n2, "greater")))
})
p <- sapply(out, function(x) x$binomial[1])
bh <- p.adjust(p, "BH")
pi0 <- min(1, mean(p >= 0.5) / 0.5)
result <- list(
  made = paste("R", R.version$major, R.version$minor, sep = "."),
  cases = out,
  bh = bh,
  storey = list(pi0 = pi0, q = pi0 * bh)
)
writeLines(toJSON(result, digits = I(17), auto_unbox = TRUE), file.path(here, "expansion.json"))
cat(sprintf("%d cases; π₀ %.4f\n", length(out), pi0))
