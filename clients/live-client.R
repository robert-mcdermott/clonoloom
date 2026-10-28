# Run only by the isolated real-host driver; proposal clicks are in that driver.
library(clonoloom)
folder <- Sys.getenv("CLONOLOOM_TEST_LIBRARY")
cl <- cl_connect(data_dir = folder, client = "R research notebook")
grants <- cl_permissions(cl)
stopifnot(identical(sort(names(grants)), sort(c("read", "edits", "compute", "files"))), all(vapply(grants, isTRUE, logical(1))), length(cl_tools(cl)) == 58L)
idle <- function() {
  for (i in seq_len(300)) {
    state <- cl_get_state(cl = cl)$data
    if (!state$importing && !length(state$runningJobs)) return(invisible(NULL))
    Sys.sleep(.1)
  }
  stop("Existing work did not finish")
}
idle()
result <- cl_query_clonotypes(limit = 1000L, cl = cl)
# Python writes original native result bytes after its final reviewed definition edit.
expected <- readBin(Sys.getenv("CLONOLOOM_TEST_PARITY"), "raw", 16L * 1024L * 1024L)
stopifnot(identical(charToRaw(result$data$resultJSON), expected))
native <- cl_native(result)
stopifnot(length(native$rows) > 100L, inherits(native$rows[[1]]$total, "clonoloom_number"), inherits(native$rows[[1]]$frequency, "clonoloom_number"))
selected <- native$rows[[4]]
filtered <- cl_native(cl_query_clonotypes(where = list(field = "id", op = "eq", value = selected$id), limit = 1L, cl = cl))
stopifnot(identical(filtered$count, native$count), identical(filtered$definitionId, native$definitionId), identical(as.character(filtered$matched), "1"), identical(as.character(filtered$rows[[1]]$rank), "1"))
filtered$rows[[1]]$rank <- selected$rank
stopifnot(identical(filtered$rows[[1]], selected))
frame <- as.data.frame(result)
stopifnot(identical(frame$total[[1]], native$rows[[1]]$total), identical(frame$frequency[[1]], native$rows[[1]]$frequency))
refusal <- function(fun, code, kind) {
  error <- tryCatch({ fun(); stop("Expected refusal") }, clonoloom_error = identity)
  stopifnot(inherits(error, kind), identical(error$code, code)); error
}
refusal(function() cl_query_clonotypes(cl = cl_connect(cl$url, token = NULL)), "unauthorized", "clonoloom_permission_error")
refusal(function() cl_open_example("simulated-vaccine", cl = cl), "permission-denied", "clonoloom_permission_error")
refusal(function() cl_query_clonotypes(limit = NULL, cl = cl), "invalid-arguments", "clonoloom_action_error")
refusal(function() cl_query_clonotypes(where = list(field = "total", op = "gte", value = cl_number("9007199254740993")), cl = cl), "invalid-arguments", "clonoloom_action_error")
refusal(function() cl_call("no_such_action", cl = cl), "unknown-action", "clonoloom_action_error")
rejected <- cl_connect(data_dir = folder, client = "R rejection control")
error <- refusal(function() cl_set_definition(preset = "Amino-acid junction only", cl = rejected), "rejected", "clonoloom_action_error")
stopifnot(!error$outcome_unknown)
cancelled <- cl_connect(data_dir = folder, client = "R cancellation control", timeout = 1)
error <- refusal(function() cl_set_definition(preset = "Amino-acid junction only", cl = cancelled), "connection-failed", "clonoloom_connection_error")
stopifnot(error$outcome_unknown)
Sys.sleep(.5); idle()
cl_set_definition(preset = "Amino-acid junction only", cl = cl)
cat("R live checks passed: exact Python/HTTP/native parity, tokens, frames, full denominators, grants/token/null/numeric refusals, human rejection, timeout and reviewed edit.\n")
