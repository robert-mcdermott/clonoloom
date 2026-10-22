# Diversity of the development files' bulk samples, and of the simulated standard study, as
# independent implementations compute it, for validation/diversity_test.go: the samples as
# samples.R reads them; then
# - iNEXT 3.0.2 (Chao et al. 2014): Chao1 with its interval (ChaoRichness), the asymptotic Hill
#   numbers of order 1 and 2 (ChaoShannon and ChaoSimpson, transformed, B = 200) and the sample
#   coverage;
# - immunarch's own functions (Apache-2.0, run from its v0 source in IMMUNARCH_SRC): Chao1 with
#   its interval, the Hill numbers of order 1 and 2, Gini's coefficient, Gini–Simpson and inverse
#   Simpson;
# - vegan 2.7 (estimateR): ACE;
# - R itself: Shannon's entropy, Simpson's index, evenness, clonality, Simpson clonality, the
#   largest clonotypes' shares, the largest frequency, the homeostasis bins (immunarch's) and the
#   Hill profile.
#
#   go run . simulate --preset standard --out validation/cache/simulated-standard
#   R_LIBS_USER=<a library with iNEXT and vegan> IMMUNARCH_SRC=<immunarch's R/ folder> \
#     Rscript validation/reference/diversity.R
suppressMessages({ library(data.table); library(jsonlite); library(iNEXT); library(vegan) })
args <- commandArgs(trailingOnly = FALSE)
here <- dirname(normalizePath(sub("^--file=", "", args[grep("^--file=", args)])))
cache <- Sys.getenv("CLONOLOOM_DATA", file.path(here, "..", "cache"))
src <- Sys.getenv("IMMUNARCH_SRC")
immunarch <- new.env()
for (f in c("v0_tools.R", "v0_diversity.R")) sys.source(file.path(src, f), envir = immunarch)
profile <- c(0, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 3.5, 4)

measures <- function(x) {
  n <- sum(x); p <- x / n; S <- length(x)
  H <- -sum(p * log(p))
  im <- immunarch$chao1(x)
  cr <- ChaoRichness(x, datatype = "abundance", conf = 0.95)
  set.seed(1)
  cs <- ChaoShannon(x, datatype = "abundance", transform = TRUE, conf = 0.95, B = 200)
  set.seed(1)
  cp <- ChaoSimpson(x, datatype = "abundance", transform = TRUE, conf = 0.95, B = 200)
  sorted <- sort(x, decreasing = TRUE)
  top <- function(k) sum(head(sorted, k)) / n
  bins <- c(rare = 1e-5, small = 1e-4, medium = 1e-3, large = 1e-2, hyperexpanded = 1)
  lower <- c(0, head(bins, -1))
  homeo <- sapply(seq_along(bins), function(i) sum(p[p > lower[i] & p <= bins[i]]))
  names(homeo) <- names(bins)
  hill <- sapply(profile, function(q) if (q == 1) exp(H) else if (q == 0) S else sum(p^q)^(1 / (1 - q)))
  list(
    abundance = n, clonotypes = S, singletons = sum(x == 1), doubletons = sum(x == 2),
    richness = S, shannon = H, hill1 = exp(H), simpson = sum(p^2), hill2 = 1 / sum(p^2),
    evenness = H / log(S), clonality = 1 - H / log(S), simpsonClonality = sqrt(sum(p^2)),
    top10 = top(10), top100 = top(100), top1000 = top(1000), maxFrequency = max(p),
    homeostasis = as.list(homeo), profile = setNames(as.list(hill), format(profile)),
    coverage = iNEXT:::Chat.Ind(x, n),
    immunarch = list(hill1 = as.numeric(immunarch$diversity_eco(x, 1)), hill2 = as.numeric(immunarch$diversity_eco(x, 2)),
      gini = as.numeric(immunarch$gini_coef(x)), giniSimpson = as.numeric(immunarch$gini_simpson(x)),
      inverseSimpson = as.numeric(immunarch$inverse_simpson(x)),
      chao1 = as.numeric(im[1]), chao1SD = as.numeric(im[2]), chao1Lower = as.numeric(im[3]), chao1Upper = as.numeric(im[4])),
    inext = list(chao1 = cr[1, "Estimator"], chao1SE = cr[1, "Est_s.e."], chao1Lower = cr[1, "95% Lower"], chao1Upper = cr[1, "95% Upper"],
      hill1 = cs[1, "Estimator"], hill1SE = cs[1, "Est_s.e"], hill1Lower = cs[1, "95% Lower"], hill1Upper = cs[1, "95% Upper"],
      hill2 = cp[1, "Estimator"], hill2SE = cp[1, "Est_s.e."], hill2Lower = cp[1, "95% Lower"], hill2Upper = cp[1, "95% Upper"]),
    vegan = list(ace = as.numeric(estimateR(x)["S.ACE"]))
  )
}

source(file.path(here, "samples.R"))
out <- lapply(bulk_samples(cache), function(samples) lapply(samples, measures))
write_json(list(generated_by = sprintf("R %s, data.table %s, iNEXT %s, vegan %s, immunarch (v0 source)", getRversion(), packageVersion("data.table"), packageVersion("iNEXT"), packageVersion("vegan")), files = out),
  file.path(here, "diversity.json"), auto_unbox = TRUE, digits = NA, pretty = TRUE, na = "null")
