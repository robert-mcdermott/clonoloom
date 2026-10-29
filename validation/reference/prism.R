# Independent pzfx reader import; no application implementation source is inspected.
args <- commandArgs(trailingOnly=TRUE)
stopifnot(length(args)==1, requireNamespace("pzfx", quietly=TRUE), requireNamespace("jsonlite", quietly=TRUE))
for (folder in list.dirs(args[1], recursive=FALSE)) {
  file <- file.path(folder, "data.pzfx")
  expected <- jsonlite::fromJSON(file.path(folder, "expected.json"), simplifyVector=FALSE)
  titles <- pzfx::pzfx_tables(file)
  stopifnot(length(titles)==length(expected))
  for (i in seq_along(expected)) {
    table <- pzfx::read_pzfx(file, table=i)
    want <- expected[[i]]
    stopifnot(identical(titles[i], want$title), identical(as.character(table$ROWTITLE), unlist(want$subjects)), ncol(table)==3)
    # The reader may make syntactic R column names; identities are checked in XML separately.
    for (g in 1:2) {
      values <- vapply(want$values[[g]], function(x) if(is.null(x)) NA_real_ else as.numeric(x), numeric(1))
      stopifnot(identical(as.numeric(table[[g+1]]), values))
    }
  }
  cat(basename(folder), ": pzfx", as.character(utils::packageVersion("pzfx")), "import preserves row identity, numeric values and NA positions.\n")
}
