\name{cl_number}
\alias{cl_number}
\alias{cl_omit}
\alias{cl_array}
\alias{cl_object}
\alias{as.double.clonoloom_number}
\alias{Ops.clonoloom_number}
\alias{Math.clonoloom_number}
\alias{Summary.clonoloom_number}
\title{Preserve JSON Numbers, Arrays, Objects and Omitted Values}
\usage{
cl_number(token)
cl_omit()
cl_array(x = list())
cl_object(x = list())
\method{as.double}{clonoloom_number}(x, ...)
}
\arguments{
\item{token}{One complete JSON number as text, such as "9007199254740993".}
\item{x}{For arrays, an unnamed plain vector/list; for objects, a uniquely named plain list. For as.double, an exact number token.}
\item{...}{Unused generic arguments.}
}
\description{Constructs explicit JSON values without silently rounding numbers or collapsing singleton arrays.}
\details{All decoded numbers retain their original tokens, including exponent and negative-zero spelling. They are not arithmetic values: arithmetic/comparison/Math/Summary operations fail. as.character gives exact token text. Explicit as.double/as.numeric warns that precision or range may be lost. R doubles cannot recover an already rounded original value; supply original text to cl_number. Plain finite doubles are encoded with 17 significant digits; integer-valued doubles outside the safe-integer range are refused. NA, NaN, infinity, factors and other unsupported classes are refused.

A plain unnamed list/vector is an array, with a length-one atomic value normally a scalar. Generated array parameters always preserve arrays. Use cl_array explicitly in nested arguments. Named lists are objects; cl_object() constructs an empty object. NULL is JSON null; cl_omit() omits a top-level action argument only. R cannot represent embedded NUL strings, which are refused. Raw response bytes remain authoritative.}
\value{An exact scalar number token, omission marker, explicit array or named object list, respectively. as.double returns a potentially lossy R double with a warning.}
\examples{
cl_number("9007199254740993")
cl_array("one")
cl_object()
}
