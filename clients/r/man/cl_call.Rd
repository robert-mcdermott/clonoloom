\name{cl_call}
\alias{cl_call}
\title{Call a Host Action with Explicit Arguments}
\usage{cl_call(.action, ..., .args = list(), cl = cl_default())}
\arguments{
\item{.action}{Exact host action name.}
\item{...}{Uniquely named arguments using host camelCase names.}
\item{.args}{Additional named list; duplicate or unnamed arguments are refused.}
\item{cl}{Connection from cl_connect.}
}
\description{Calls the shared action catalog after checking live API/catalog/policy compatibility. Generated wrappers provide named parameters and help.}
\details{cl_omit() omits only a top-level argument; NULL sends null. Number tokens from cl_number() are emitted unquoted, without double conversion. Host argument bounds remain authoritative. Requests are limited to 1 MiB, responses to 16 MiB, depth to 64. Malformed or oversized results are failures, never partial successes.}
\value{A clonoloom_result with action, message, data and raw_json bytes. JSON numbers are clonoloom_number character-backed tokens. No scientific method or exact certificate replay changes.}
\seealso{\code{\link{cl_number}}, \code{\link{cl_native}}, \code{\link{cl_connect}}}
