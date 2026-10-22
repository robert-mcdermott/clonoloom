# Rarefaction and extrapolation of the development files' bulk samples and the simulated standard
# study (the samples as samples.R reads them), as iNEXT 3.0.2 computes them (Chao et al. 2014), for
# validation/rarefaction_test.go:
# - each sample's curves (orders 0, 1 and 2; iNEXT's 40 sizes from 1 to twice the sample): the
#   Hill numbers and the sample coverage by size, and the Hill numbers by coverage, with their
#   intervals from 200 bootstrap replicates for the samples of at most 40,000 cells (iNEXT sums
#   every subsample count, which takes too long for larger ones);
# - two cohorts, the simulated standard study's twelve samples and Emerson et al. 2017's three
#   immunoSEQ subjects, compared at iNEXT's default size and coverage (estimateD, 200 replicates).
#
#   R_LIBS_USER=<a library with iNEXT> Rscript validation/reference/rarefaction.R
suppressMessages({ library(jsonlite); library(iNEXT); library(parallel) })
args <- commandArgs(trailingOnly = FALSE)
here <- dirname(normalizePath(sub("^--file=", "", args[grep("^--file=", args)])))
cache <- Sys.getenv("CLONOLOOM_DATA", file.path(here, "..", "cache"))
source(file.path(here, "samples.R"))

columns <- function(d, cols) setNames(lapply(cols, function(c) d[[c]]), cols)
by_order <- function(d) lapply(split(d, d$Order.q), function(o) columns(o, c("qD", "qD.LCL", "qD.UCL")))

curves <- function(x) {
  n <- sum(x)
  se <- n <= 40000
  set.seed(1)
  est <- iNEXT(x, q = c(0, 1, 2), datatype = "abundance", knots = 40, se = se, nboot = 200, conf = 0.95)$iNextEst
  size <- est$size_based
  first <- size[size$Order.q == 0, ]
  cover <- est$coverage_based
  list(n = n, bootstrap = se,
    size = c(columns(first, c("m", "SC", "SC.LCL", "SC.UCL")), list(orders = by_order(size))),
    coverage = c(columns(cover[cover$Order.q == 0, ], c("SC", "m")), list(orders = by_order(cover))))
}

cohort <- function(all, prefix) {
  xs <- list()
  for (rel in names(all)) if (startsWith(rel, prefix)) for (s in names(all[[rel]])) xs[[s]] <- all[[rel]][[s]]
  at <- function(base) {
    set.seed(1)
    columns(estimateD(xs, q = c(0, 1, 2), datatype = "abundance", base = base, nboot = 200, conf = 0.95),
      c("Assemblage", "m", "Method", "Order.q", "SC", "qD", "qD.LCL", "qD.UCL"))
  }
  list(size = at("size"), coverage = at("coverage"))
}

# Each sample (and each cohort) on a core of its own: each sets its own seed, so the results are
# those of running them one after another.
all <- bulk_samples(cache)
cores <- max(1, detectCores() - 1)
jobs <- unlist(lapply(names(all), function(rel) lapply(names(all[[rel]]), function(s) c(rel, s))), recursive = FALSE)
done <- mclapply(jobs, function(j) curves(all[[j[1]]][[j[2]]]), mc.cores = cores, mc.preschedule = FALSE)
out <- list()
for (k in seq_along(jobs)) out[[jobs[[k]][1]]][[jobs[[k]][2]]] <- done[[k]]
cohorts <- mclapply(c("simulated-standard", "emerson-2017-immunoseq"), function(prefix) cohort(all, paste0(prefix, "/")), mc.cores = 2)
names(cohorts) <- c("simulated-standard", "emerson-2017-immunoseq")
write_json(list(generated_by = sprintf("R %s, data.table %s, iNEXT %s", getRversion(), packageVersion("data.table"), packageVersion("iNEXT")), files = out, cohorts = cohorts),
  file.path(here, "rarefaction.json"), auto_unbox = TRUE, digits = NA, pretty = TRUE, na = "null")
