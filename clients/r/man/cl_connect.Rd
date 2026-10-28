\name{cl_connect}
\alias{cl_connect}
\alias{cl_default}
\alias{cl_find_connection}
\alias{cl_tools}
\alias{cl_permissions}
\alias{cl_version}
\title{Connect to the Local ClonoLoom Host}
\usage{
cl_connect(url = NULL, token = cl_omit(), client = "R", timeout = 660,
  data_dir = NULL)
cl_default()
cl_find_connection(data_dir = NULL)
cl_tools(cl = cl_default())
cl_permissions(cl = cl_default())
cl_version(cl = cl_default())
}
\arguments{
\item{url}{Explicit loopback HTTP URL. Never borrows a discovered token.}
\item{token}{Omitted uses the matching discovered token; NULL or empty string disables it.}
\item{client}{Printable caller label, 1--60 characters; not authenticated identity.}
\item{timeout}{Positive total HTTP transfer timeout in seconds, default 660; maximum 2147483.647. Rounded up to milliseconds, at least one millisecond.}
\item{data_dir}{Explicit library directory containing remote.json, ahead of environment settings.}
\item{cl}{A connection; defaults to the last successful connection.}
}
\description{Connects to the normal host, checks API/catalog/policy compatibility and remembers the connection. The host must already be running with remote control enabled.}
\details{Discovery uses explicit URL/token, explicit library, environment CLONOLOOM_URL/CLONOLOOM_TOKEN, environment CLONOLOOM_DATA_DIR, then the platform library. Missing default remote.json falls back to port 8860 without a token; explicit missing or malformed files fail. Redirects and proxies are disabled. Only loopback HTTP is allowed.

Every action checks compatibility again before submission. Tokens do not widen explicit host grants. Project changes require human acceptance in the window. This client neither launches hosts nor accepts proposals. Host replacement between requests remains a race.

Errors inherit clonoloom_error, with code, status, action and outcome_unknown. Subclasses are clonoloom_connection_error, clonoloom_compatibility_error, clonoloom_protocol_error, clonoloom_permission_error, clonoloom_argument_error and clonoloom_action_error. Timeouts after submission may leave completed work: inspect the project before resubmitting. No automatic retries or transactional rollback.}
\value{cl_connect/cl_default return a clonoloom connection. cl_find_connection returns a URL/token list. cl_tools returns the live tool list; cl_permissions a named grant list; cl_version the host version string.}
