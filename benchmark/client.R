# Transport only. Never round-trip successful scientific JSON through jsonlite doubles.
suppressPackageStartupMessages(library(clonoloom))
input <- file("stdin", "rb")
request <- getFromNamespace(".cl_decode", "clonoloom")(readBin(input, "raw", 1048576L))
close(input)
tryCatch({
  cl <- cl_connect(data_dir = commandArgs(TRUE)[1], client = "Benchmark candidate", timeout = 30)
  result <- cl_call(request$action, .args = request$args, cl = cl)
  cat(rawToChar(result$raw_json))
}, clonoloom_error = function(error) {
  cat(jsonlite::toJSON(list(ok = FALSE, code = error$code, outcomeUnknown = error$outcome_unknown,
                          evidenceOrigin = "r-client-exception"), auto_unbox = TRUE))
})
