# Gene usage, junction lengths and amino-acid composition of the development files' bulk samples in
# AIRR format (the samples as samples.R reads them), computed independently for
# validation/usage_test.go. Each sample's clonotypes are its productive rearrangements grouped by
# locus, amino-acid junction, V gene and J gene (samples.R); a clonotype's rearrangements share it in
# proportion to their counts, and a call that ties several genes gives each an equal part. For each
# sample, by clonotypes (each counted once) and by abundance:
# - the V, D, J and C genes' shares at the gene and the family level (IMGT's subgroup: the name to its
#   first "-" or "/", without the "D" of a duplicated gene), and the V–J pairs' at the gene level,
#   "(none)" for no call;
# - the junction lengths' shares (amino acids) and the mean length;
# - each amino acid's share of the junctions' residues (X for any but the twenty).
# immunoSEQ files are left out: their gene names are Adaptive's, which ClonoLoom maps to IMGT's
# (validated with the readers).
#
#   R_LIBS_USER=<a library with jsonlite> Rscript validation/reference/usage.R
suppressMessages({ library(jsonlite) })
args <- commandArgs(trailingOnly = FALSE)
here <- dirname(normalizePath(sub("^--file=", "", args[grep("^--file=", args)])))
cache <- Sys.getenv("CLONOLOOM_DATA", file.path(here, "..", "cache"))
source(file.path(here, "samples.R"))

missing <- c("", "none", "na", "nan", "null", "unresolved", "-", ".")
genes_of <- function(call, level) {
  parts <- trimws(strsplit(call, "[,| ;]")[[1]])
  parts <- parts[!(tolower(parts) %in% missing)]
  g <- unique(toupper(sub("[*].*$", "", parts)))
  g <- g[grepl("^(IG[HKL]|TR[ABDG])[VDJC]", g)]
  if (level == "family") {
    f <- sub("[-/].*$", "", g)
    f <- ifelse(nchar(f) > 5 & grepl("[0-9]D$", f), sub("D$", "", f), f)
    g <- unique(f)
  }
  if (!length(g)) "(none)" else sort(g)
}

# add accumulates each name's parts: by clonotypes (the rearrangement's part of its clonotype) and
# by abundance (its count), a tie's parts split equally.
add <- function(acc, names, part, count) {
  for (n in names) {
    if (is.null(acc[[n]])) acc[[n]] <- c(0, 0)
    acc[[n]] <- acc[[n]] + c(part, count) / length(names)
  }
  acc
}

measure <- function(rows) {
  keys <- clonotype_key(rows)
  rows <- rows[keys != ""]
  rows$key <- keys[keys != ""]
  rows$part <- rows$count / ave(rows$count, rows$key, FUN = sum)
  clonotypes <- length(unique(rows$key))
  abundance <- sum(rows$count)
  per <- function(acc, d) lapply(acc, function(v) v / d)
  genes <- list()
  for (seg in c("v", "d", "j", "c")) for (level in c("gene", "family")) {
    acc <- list()
    calls <- rows[[seg]]
    for (i in seq_len(nrow(rows))) acc <- add(acc, genes_of(calls[i], level), rows$part[i], rows$count[i])
    genes[[paste(toupper(seg), level)]] <- per(acc, c(clonotypes, abundance))
  }
  acc <- list()
  for (i in seq_len(nrow(rows))) {
    v <- genes_of(rows$v[i], "gene"); j <- genes_of(rows$j[i], "gene")
    pairs <- as.vector(outer(sub("[(]none[)]", "", v), sub("[(]none[)]", "", j), paste, sep = "+"))
    acc <- add(acc, pairs, rows$part[i], rows$count[i])
  }
  genes[["VJ gene"]] <- per(acc, c(clonotypes, abundance))
  n <- nchar(rows$junction_aa)
  lengths <- list()
  for (i in seq_len(nrow(rows))) lengths <- add(lengths, as.character(n[i]), rows$part[i], rows$count[i])
  residues <- c(sum(rows$part * n), sum(rows$count * n))
  composition <- list()
  for (i in seq_len(nrow(rows))) {
    aa <- strsplit(rows$junction_aa[i], "")[[1]]
    aa[!(aa %in% strsplit("ACDEFGHIKLMNPQRSTVWY", "")[[1]])] <- "X"
    for (a in aa) {
      if (is.null(composition[[a]])) composition[[a]] <- c(0, 0)
      composition[[a]] <- composition[[a]] + c(rows$part[i], rows$count[i])
    }
  }
  list(clonotypes = clonotypes, abundance = abundance, genes = genes,
    lengths = per(lengths, c(clonotypes, abundance)),
    meanLength = c(sum(rows$part * n) / clonotypes, sum(rows$count * n) / abundance),
    composition = per(composition, residues))
}

all <- bulk_rows(cache)
out <- list()
for (rel in names(all)) {
  rows <- all[[rel]]
  if (attr(rows, "format") != "airr") next
  message(rel)
  samples <- list()
  for (s in unique(rows$sample)) {
    at <- rows[rows$sample == s]
    keys <- clonotype_key(at)
    x <- tapply(at$count[keys != ""], keys[keys != ""], sum)
    if (length(x) > 1 && all(x == round(x))) samples[[s]] <- measure(at)
  }
  if (length(samples)) out[[rel]] <- samples
}
write_json(list(generated_by = sprintf("R %s, data.table %s", getRversion(), packageVersion("data.table")), files = out),
  file.path(here, "usage.json"), auto_unbox = TRUE, digits = NA, pretty = TRUE, na = "null")
