\name{cl_native}
\alias{cl_native}
\alias{cl_records}
\alias{as.data.frame.clonoloom_result}
\alias{print.clonoloom}
\alias{print.clonoloom_result}
\title{Inspect Exact Native Evidence and Record Tables}
\usage{
cl_native(result)
cl_records(result, key = NULL)
\method{as.data.frame}{clonoloom_result}(x, row.names = NULL,
  optional = FALSE, key = NULL, ...)
\method{print}{clonoloom}(x, ...)
\method{print}{clonoloom_result}(x, ...)
}
\arguments{
\item{result, x}{An action result; print.clonoloom takes a connection.}
\item{key}{Explicit record-array key; otherwise the root array or rows.}
\item{row.names}{Optional unique row names.}
\item{optional}{Generic argument; column names are always preserved.}
\item{...}{Unused generic arguments.}
}
\description{Decodes original native resultJSON without double conversion, or selects record rows without guessing among arbitrary lists.}
\details{as.data.frame uses list columns retaining original number tokens, booleans, lists and NULL. Missing keys become NULL only in this convenience view; records and raw_json retain absent versus null. No values are filtered, rounded or inferred. Converting list columns, arithmetic and plotting are explicit caller choices. These objects do not replace original certificates or establish scientific validity.}
\value{cl_native returns native JSON values; cl_records a record array; as.data.frame a data frame with list columns. Print methods return their input invisibly.}
