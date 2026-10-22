# Overlap between the samples of two cohorts of the development data (the samples as samples.R reads
# them), computed independently for validation/overlap_test.go: the simulated standard study's
# twelve samples (six subjects, two samples each) and Emerson et al. 2017's three immunoSEQ subjects
# (counted in reads, as a dataset of the three counts them, since one has no templates). For every
# pair of samples:
# - shared clonotypes; Jaccard's index, Sørensen–Dice's (Tversky's with a = b = 0.5) and the overlap
#   coefficient, by immunarch's own functions (Apache-2.0, run from its v0 source in
#   IMMUNARCH_SRC);
# - the same expected with both samples rarefied to the smallest sample's size (R, by lchoose);
# - the Morisita–Horn index, by immunarch's morisita_index and as 1 − vegan's vegdist "horn";
#   Morisita's index as 1 − vegan's vegdist "morisita";
# - each sample's share of its abundance in the clonotypes the other holds too (R);
# and the clonotypes by how many subjects hold them (the simulated subjects named by their samples'
# names, Emerson's each its own).
#
#   R_LIBS_USER=<a library with vegan> IMMUNARCH_SRC=<immunarch's R/ folder> Rscript validation/reference/overlap.R
suppressMessages({ library(jsonlite); library(vegan); library(dplyr); library(data.table) })
args <- commandArgs(trailingOnly = FALSE)
here <- dirname(normalizePath(sub("^--file=", "", args[grep("^--file=", args)])))
cache <- Sys.getenv("CLONOLOOM_DATA", file.path(here, "..", "cache"))
source(file.path(here, "samples.R"))
immunarch <- new.env()
sys.source(file.path(Sys.getenv("IMMUNARCH_SRC"), "v0_overlap.R"), envir = immunarch)

cohort <- function(all, prefix, reads, subject_of) {
  rows <- rbindlist(all[startsWith(names(all), prefix)], fill = TRUE)
  if (reads) rows$count <- rows$reads
  keys <- clonotype_key(rows)
  rows <- rows[keys != ""]
  rows$key <- keys[keys != ""]
  names <- sort(unique(rows$sample))
  counts <- lapply(names, function(s) { at <- rows[rows$sample == s]; tapply(at$count, at$key, sum) })
  names(counts) <- names
  m <- min(sapply(counts, sum))
  seen <- function(n, c, m) if (m >= n) rep(1, length(c)) else 1 - exp(lchoose(n - c, m) - lchoose(n, m))
  union <- sort(unique(rows$key))
  M <- t(sapply(counts, function(x) { v <- setNames(numeric(length(union)), union); v[names(x)] <- x; v }))
  horn <- as.matrix(vegdist(M, method = "horn"))
  morisita <- as.matrix(vegdist(M, method = "morisita"))
  pairs <- list()
  for (i in seq_along(names)) for (j in seq_along(names)) {
    if (j <= i) next
    x <- counts[[i]]; y <- counts[[j]]
    common <- intersect(names(x), names(y))
    nx <- sum(x); ny <- sum(y)
    ex <- sum(seen(nx, x, m)); ey <- sum(seen(ny, y, m))
    es <- sum(seen(nx, x[common], m) * seen(ny, y[common], m))
    pairs[[length(pairs) + 1]] <- list(a = names[i], b = names[j],
      shared = length(common),
      jaccard = immunarch$jaccard_index.character(names(x), names(y)),
      sorensen = immunarch$tversky_index.character(names(x), names(y), .5, .5),
      overlap = immunarch$overlap_coef.character(names(x), names(y)),
      sharedRarefied = es, jaccardRarefied = es / (ex + ey - es), sorensenRarefied = 2 * es / (ex + ey), overlapRarefied = es / min(ex, ey),
      morisitaHorn = immunarch$morisita_index(data.frame(key = names(x), n = as.numeric(x)), data.frame(key = names(y), n = as.numeric(y)), "n"),
      morisitaHornVegan = 1 - horn[i, j], morisitaVegan = 1 - morisita[i, j],
      abundanceOfA = sum(x[common]) / nx, abundanceOfB = sum(y[common]) / ny)
  }
  subjects <- sapply(names, subject_of)
  units <- tapply(subjects[rows$sample], rows$key, function(s) length(unique(s)))
  list(samples = names, depth = m, pairs = pairs,
    incidence = as.list(sapply(seq_len(length(unique(subjects))), function(k) sum(units == k))))
}

all <- bulk_rows(cache)
out <- list(
  `simulated-standard` = cohort(all, "simulated-standard/", FALSE, function(s) sub("-[0-9]+$", "", s)),
  `emerson-2017-immunoseq` = cohort(all, "emerson-2017-immunoseq/", TRUE, function(s) s))
write_json(list(generated_by = sprintf("R %s, vegan %s, dplyr %s, immunarch (v0 source)", getRversion(), packageVersion("vegan"), packageVersion("dplyr")), cohorts = out),
  file.path(here, "overlap.json"), auto_unbox = TRUE, digits = NA, pretty = TRUE, na = "null")
