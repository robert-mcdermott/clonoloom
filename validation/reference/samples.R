# The development files' bulk samples and the simulated standard study, read and grouped into
# clonotypes as validation/reference/qc.R does, for diversity.R and rarefaction.R: data.table reads
# each file and groups each sample's productive rearrangements into clonotypes (locus, amino-acid
# junction, V gene and J gene, alleles dropped, ties kept as sets). bulk_rows(cache) answers each
# file's rows (by the file's path under the cache, its format the attribute "format"): sample,
# locus, amino-acid and nucleotide junction, V, D, J and C calls as given, productive, count (and
# for immunoSEQ, reads).
# bulk_samples(cache) answers each file's samples and each sample's clonotype abundances, for
# samples of more than one clonotype, all whole numbers.
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

# A row's clonotype: its locus, amino-acid junction and V and J genes ("" for a row in none:
# not productive, without a junction or a count).
clonotype_key <- function(rows) {
  w <- rows$count
  w[is.na(w)] <- 0
  ifelse(rows$productive & rows$junction_aa != "" & w > 0,
    paste(rows$locus, rows$junction_aa, gene_key(rows$v), gene_key(rows$j), sep = "\r"), "")
}

tally <- function(rows) {
  out <- list()
  keys <- clonotype_key(rows)
  for (s in unique(rows$sample)) {
    at <- rows$sample == s & keys != ""
    x <- as.numeric(tapply(rows$count[at], keys[at], sum))
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
  data.table(sample = sample, locus = unname(locus), junction_aa = aa, junction = nt, v = call("v"), d = call("d"), j = call("j"), c = "",
    productive = productive, count = count, reads = reads)
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
  data.table(sample = sample, locus = locus, junction_aa = toupper(get("junction_aa")), junction = toupper(get("junction")),
    v = get("v_call"), d = get("d_call"), j = get("j_call"), c = get("c_call"), productive = productive, count = count)
}

bulk_rows <- function(cache) {
  out <- list()
  for (dataset in sort(list.dirs(cache, recursive = FALSE, full.names = FALSE))) {
    for (path in sort(list.files(file.path(cache, dataset), recursive = TRUE, full.names = TRUE))) {
      if (grepl("[.](gz|part|json|yaml|csv|md)$", path) || grepl("/truth/", path)) next
      header <- strsplit(readLines(path, n = 1, warn = FALSE), "\t")[[1]]
      format <- if (any(c("rearrangement", "aminoAcid") %in% header)) "immunoseq" else if ("sequence_id" %in% header) "airr" else ""
      rows <- switch(format, immunoseq = immunoseq(path), airr = airr(path), NULL)
      if (is.null(rows) || !nrow(rows)) next
      setattr(rows, "format", format)
      out[[substring(normalizePath(path), nchar(normalizePath(cache)) + 2)]] <- rows
    }
  }
  out
}

bulk_samples <- function(cache) {
  out <- list()
  all <- bulk_rows(cache)
  for (rel in names(all)) {
    res <- tally(all[[rel]])
    if (!length(res)) next
    message(rel)
    out[[rel]] <- res
  }
  out
}
