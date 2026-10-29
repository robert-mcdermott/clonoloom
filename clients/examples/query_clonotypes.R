# Rscript clients/examples/query_clonotypes.R /path/to/library
# Start the host separately, open a dataset and grant compute explicitly.
library(clonoloom)
args <- commandArgs(trailingOnly = TRUE)
cl <- cl_connect(data_dir = if (length(args)) args[[1]] else NULL,
                 client = "R repertoire query")
result <- cl_query_clonotypes(
  where = list(field = "total", op = "gte", value = cl_number("2")),
  limit = 10L, cl = cl)
evidence <- cl_native(result)
cat("Full table:", as.character(evidence$count), "Matched:", as.character(evidence$matched),
    "Definition:", evidence$definitionId, "\n")
for (row in cl_records(result)) cat(row$id, as.character(row$total), as.character(row$frequency), "\n")
