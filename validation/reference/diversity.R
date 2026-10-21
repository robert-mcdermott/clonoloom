# Diversity of the development files' bulk samples, and of the simulated standard study, as
# independent implementations compute it, for validation/diversity_test.go: data.table reads each
# file and groups each sample's productive rearrangements into clonotypes (locus, amino-acid
# junction, V gene and J gene, alleles dropped, ties kept as sets), as validation/reference/qc.R
# does; then
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
loci <- c("IGH", "IGK", "IGL", "TRA", "TRB", "TRD", "TRG")
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

gene_key <- function(calls) {
  vapply(calls, function(call) {
    if (is.na(call) || call == "") return("")
    parts <- trimws(strsplit(call, "[,| ;]")[[1]])
    parts <- parts[parts != ""]
    paste(sort(unique(toupper(sub("[*].*$", "", parts)))), collapse = "|")
  }, character(1), USE.NAMES = FALSE)
}

locus_of <- function(given, j, v) {
  out <- ifelse(toupper(given) %in% loci, toupper(given), NA_character_)
  for (call in list(j, v)) {
    guess <- toupper(substr(call, 1, 3))
    out <- ifelse(is.na(out) & guess %in% loci, guess, out)
  }
  out
}

nulls <- c("", "null", "NULL", "na", "NA", "unresolved", "unknown")
clean <- function(x) { x <- as.character(x); x[is.na(x) | x %in% nulls] <- ""; x }

tally <- function(sample, locus, junction_aa, v, j, productive, count, junction_nt) {
  out <- list()
  for (s in unique(sample)) {
    at <- sample == s
    w <- count[at]
    w[is.na(w)] <- 0
    p <- productive[at] & junction_aa[at] != "" & w > 0
    keys <- paste(locus[at], junction_aa[at], gene_key(v[at]), gene_key(j[at]), sep = "\r")[p]
    x <- as.numeric(tapply(w[p], keys, sum))
    if (length(x) > 1 && all(x == round(x))) out[[s]] <- measures(x)
  }
  out
}

immunoseq <- function(path) {
  d <- fread(path, sep = "\t", colClasses = "character", na.strings = NULL)
  v3 <- "rearrangement" %in% names(d)
  col <- function(a, b) if (v3) a else b
  templates <- suppressWarnings(as.numeric(clean(d[[col("templates", "estimatedNumberGenomes")]])))
  reads_col <- intersect(c("seq_reads", "reads"), names(d))[1]
  reads <- if (!is.na(reads_col)) suppressWarnings(as.numeric(clean(d[[reads_col]]))) else rep(NA_real_, nrow(d))
  count <- if (all(!is.na(templates) & templates > 0)) templates else reads
  count[!is.na(count) & count < 0] <- NA
  call <- function(seg) {
    if (v3) { r <- clean(d[[paste0(seg, "_resolved")]]); g <- clean(d[[paste0(seg, "_gene")]]); t <- clean(d[[paste0(seg, "_gene_ties")]]) }
    else { r <- clean(d[[paste0(seg, "MaxResolved")]]); g <- clean(d[[paste0(seg, "GeneName")]]); t <- clean(d[[paste0(seg, "GeneNameTies")]]) }
    ifelse(g == "" & t != "", t, r)
  }
  sample <- if ("sample_name" %in% names(d)) clean(d$sample_name) else rep("", nrow(d))
  sample[sample == ""] <- sub("[.](tsv|txt)(\\.gz)?$", "", basename(path))
  locus <- if ("locus" %in% names(d)) c(TCRB = "TRB", TCRA = "TRA", TCRG = "TRG", TCRD = "TRD", IGH = "IGH", IGK = "IGK", IGL = "IGL")[toupper(clean(d$locus))] else NA
  locus[is.na(locus)] <- "TRB"
  aa <- toupper(clean(d[[col("amino_acid", "aminoAcid")]]))
  productive <- tolower(clean(d[[col("frame_type", "sequenceStatus")]])) == "in"
  read <- clean(d[[col("rearrangement", "nucleotide")]])
  start <- suppressWarnings(as.integer(clean(d[[col("v_index", "vIndex")]])))
  len <- suppressWarnings(as.integer(clean(d[[col("cdr3_length", "cdr3Length")]])))
  nt <- ifelse(!is.na(start) & !is.na(len) & start >= 0 & len > 0 & start + len <= nchar(read), toupper(substr(read, start + 1, start + len)), "")
  if ("cdr3_rearrangement" %in% names(d)) { given <- toupper(clean(d$cdr3_rearrangement)); nt[given != ""] <- given[given != ""] }
  tally(sample, unname(locus), aa, call("v"), call("j"), productive, count, nt)
}

airr <- function(path) {
  d <- fread(path, sep = "\t", colClasses = "character", na.strings = NULL, quote = "")
  if ("cell_id" %in% names(d) && any(clean(d$cell_id) != "")) return(NULL)
  # A row's count: its UMIs, else its duplicates, else its consensus reads, whichever it has.
  cols <- intersect(c("umi_count", "duplicate_count", "consensus_count"), names(d))
  count <- if (!length(cols)) rep(1, nrow(d)) else rep(NA_real_, nrow(d))
  for (c in cols) { v <- suppressWarnings(as.numeric(clean(d[[c]]))); count[is.na(count)] <- v[is.na(count)] }
  count[!is.na(count) & count < 0] <- NA
  sample <- if ("repertoire_id" %in% names(d)) clean(d$repertoire_id) else rep("", nrow(d))
  if ("sample_processing_id" %in% names(d)) sample[sample == ""] <- clean(d$sample_processing_id)[sample == ""]
  sample[sample == ""] <- sub("[.](tsv|txt)(\\.gz)?$", "", basename(path))
  get <- function(c) if (c %in% names(d)) clean(d[[c]]) else rep("", nrow(d))
  locus <- locus_of(get("locus"), get("j_call"), get("v_call"))
  productive <- get("productive") %in% c("T", "TRUE", "True", "true", "t", "1")
  tally(sample, locus, toupper(get("junction_aa")), get("v_call"), get("j_call"), productive, count, toupper(get("junction")))
}

out <- list()
for (dataset in sort(list.dirs(cache, recursive = FALSE, full.names = FALSE))) {
  for (path in sort(list.files(file.path(cache, dataset), recursive = TRUE, full.names = TRUE))) {
    if (grepl("[.](gz|part|json|yaml|csv|md)$", path) || grepl("/truth/", path)) next
    header <- strsplit(readLines(path, n = 1, warn = FALSE), "\t")[[1]]
    res <- if (any(c("rearrangement", "aminoAcid") %in% header)) immunoseq(path) else if ("sequence_id" %in% header) airr(path) else NULL
    if (is.null(res) || !length(res)) next
    rel <- substring(normalizePath(path), nchar(normalizePath(cache)) + 2)
    message(rel)
    out[[rel]] <- res
  }
}
write_json(list(generated_by = sprintf("R %s, data.table %s, iNEXT %s, vegan %s, immunarch (v0 source)", getRversion(), packageVersion("data.table"), packageVersion("iNEXT"), packageVersion("vegan")), files = out),
  file.path(here, "diversity.json"), auto_unbox = TRUE, digits = NA, pretty = TRUE, na = "null")
