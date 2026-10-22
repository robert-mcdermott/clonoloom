# The development files' bulk samples and the simulated standard study, read and grouped into
# clonotypes as validation/reference/qc.R does, for diversity.R and rarefaction.R: data.table reads
# each file and groups each sample's productive rearrangements into clonotypes (locus, amino-acid
# junction, V gene and J gene, alleles dropped, ties kept as sets). bulk_samples(cache) answers
# each file's samples (by the file's path under the cache) and each sample's clonotype abundances,
# for samples of more than one clonotype, all whole numbers.
suppressMessages(library(data.table))
loci <- c("IGH", "IGK", "IGL", "TRA", "TRB", "TRD", "TRG")

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
    if (length(x) > 1 && all(x == round(x))) out[[s]] <- x
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

bulk_samples <- function(cache) {
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
  out
}
