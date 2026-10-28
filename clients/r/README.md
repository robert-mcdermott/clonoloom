# ClonoLoom R client

Generated functions and help for the normal local host, with explicit grants and human review.
See [the R guide](../../docs/R.md) for exact number tokens, connection precedence and limits.
Use the development host (API 2/catalog 1/policy 1); published 0.7.0 predates this contract.

```sh
Rscript -e 'install.packages(c("curl", "jsonlite"), repos="https://cloud.r-project.org")'
R CMD INSTALL clients/r
clonoloom --remote-control --allow edits,compute,files
```

```r
library(clonoloom)
cl <- cl_connect(client = "Research notebook")
cl_permissions(cl)
# Open a repertoire in the window before querying.
result <- cl_query_clonotypes(limit = 10L, cl = cl)
cl_records(result)
as.data.frame(result)  # list columns; exact tokens, no numeric inference
```

All JSON numbers retain original tokens. Supply precise numeric arguments as `cl_number("2")`;
`as.character()` returns text and explicit numeric conversion warns about precision loss.
`cl_omit()` omits a top-level argument; NULL sends null. Generated array parameters retain
singleton arrays. No automatic acceptance, retries, host launch or scientific recomputation.
