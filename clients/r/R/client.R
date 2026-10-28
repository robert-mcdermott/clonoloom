# S3 connection/result and curl pattern adapted from Apache-2.0 CytoWeave/MaveScape.
# Host grants and window review remain authoritative. No retries or auto-acceptance.
.cl <- new.env(parent = emptyenv())
.cl_error <- function(message, code = "client-error", status = 0L, action = NULL,
                      outcome_unknown = FALSE, kind = "argument") {
  structure(list(message = message, call = NULL, code = code, status = status,
                 action = action, outcome_unknown = outcome_unknown),
            class = c(paste0("clonoloom_", kind, "_error"), "clonoloom_error", "error", "condition"))
}
.cl_url <- function(url) {
  # Restrict syntax before libcurl normalizes it; reject credentials, non-root paths,
  # ambiguous numeric hosts and non-loopback destinations. localhost is pinned to IPv4.
  if (!.cl_scalar_text(url) || grepl("[[:space:][:cntrl:]]", url) || !grepl("^http://(localhost|127\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}|\\[::1\\])(:[0-9]{1,5})?/?$", url))
    stop(.cl_error("URL must be loopback HTTP with a root path", code = "invalid-url"))
  u <- tryCatch(curl::curl_parse_url(url), error = function(e) stop(.cl_error("Invalid URL", code = "invalid-url")))
  if (!is.null(u$port) && (as.integer(u$port) < 1L || as.integer(u$port) > 65535L))
    stop(.cl_error("Invalid port", code = "invalid-url"))
  if (startsWith(u$host, "127.")) {
    original_host <- sub("(:[0-9]+)?/?$", "", substring(url, 8L))
    octets <- strsplit(original_host, ".", fixed = TRUE)[[1]]
    if (any(as.integer(octets) > 255L) || any(nchar(octets) > 1L & startsWith(octets, "0")))
      stop(.cl_error("Invalid loopback address", code = "invalid-url"))
  }
  sub("/$", "", sub("^http://localhost", "http://127.0.0.1", url))
}
.cl_token <- function(token) {
  if (is.null(token) || identical(token, "")) return(NULL)
  if (!.cl_scalar_text(token) || nchar(token, type = "bytes") > 1024L || grepl("[^!-~]", token))
    stop(.cl_error("Token must be printable ASCII", code = "invalid-token"))
  token
}
.cl_default_data_dir <- function() {
  home <- path.expand("~")
  if (.Platform$OS.type == "windows") {
    base <- Sys.getenv("APPDATA")
    return(if (nzchar(base)) file.path(base, "ClonoLoom") else file.path(home, ".clonoloom"))
  }
  if (Sys.info()[["sysname"]] == "Darwin") return(file.path(home, "Library", "Application Support", "ClonoLoom"))
  base <- Sys.getenv("XDG_CONFIG_HOME")
  file.path(if (nzchar(base)) base else file.path(home, ".config"), "ClonoLoom")
}
cl_find_connection <- function(data_dir = NULL) {
  url <- Sys.getenv("CLONOLOOM_URL")
  if (is.null(data_dir) && nzchar(url)) return(list(url = .cl_url(url), token = .cl_token(Sys.getenv("CLONOLOOM_TOKEN"))))
  env_dir <- Sys.getenv("CLONOLOOM_DATA_DIR")
  explicit <- !is.null(data_dir) || nzchar(env_dir)
  folder <- if (!is.null(data_dir)) data_dir else if (nzchar(env_dir)) env_dir else .cl_default_data_dir()
  if (!.cl_scalar_text(folder)) stop(.cl_error("data_dir must be one path"))
  path <- file.path(folder, "remote.json")
  if (!explicit && !file.exists(path)) return(list(url = "http://127.0.0.1:8860", token = NULL))
  tryCatch({
    con <- suppressWarnings(file(path, "rb")); on.exit(close(con))
    info <- .cl_decode(readBin(con, "raw", 65537L), limit = 65536L)
    list(url = .cl_url(info[["url", exact = TRUE]]), token = .cl_token(info[["token", exact = TRUE]]))
  }, error = function(e) stop(.cl_error("Cannot read a valid remote.json from the requested library", code = "discovery-failed", kind = "connection")))
}
cl_connect <- function(url = NULL, token = cl_omit(), client = "R", timeout = 660, data_dir = NULL) {
  found <- if (is.null(url)) cl_find_connection(data_dir) else list(url = url, token = NULL)
  if (!.cl_scalar_text(client) || !nzchar(client) || nchar(client) > 60L || grepl("[[:cntrl:]]", client))
    stop(.cl_error("client must be a printable label of 1-60 characters", code = "invalid-client"))
  if (!is.numeric(timeout) || is.object(timeout) || length(timeout) != 1L || !is.finite(timeout) || timeout <= 0 || timeout > 2147483.647)
    stop(.cl_error("timeout must be positive and at most 2147483.647 seconds", code = "invalid-timeout"))
  cl <- structure(list(url = .cl_url(found$url), token = .cl_token(if (inherits(token, "clonoloom_omit")) found$token else token),
                       client = client, timeout = timeout), class = "clonoloom")
  cl$version <- .cl_refresh(cl)$version
  .cl$default <- cl
  cl
}
cl_default <- function() if (is.null(.cl$default)) cl_connect() else .cl$default

.cl_request <- function(cl, method, path, body = NULL, action = NULL) {
  submitted <- identical(method, "POST")
  handle <- curl::new_handle(timeout_ms = max(1, ceiling(cl$timeout * 1000)), proxy = "", followlocation = FALSE,
                            customrequest = method)
  headers <- list(Accept = "application/json")
  if (submitted && !is.null(cl$token)) headers[["X-ClonoLoom-Token"]] <- .cl_token(cl$token)
  if (!is.null(body)) {
    headers[["Content-Type"]] <- "application/json"
    curl::handle_setopt(handle, postfields = body)
  }
  do.call(curl::handle_setheaders, c(list(handle), headers))
  chunks <- list(); size <- 0L; overflow <- FALSE
  response <- tryCatch(suppressWarnings(curl::curl_fetch_stream(paste0(cl$url, path), function(chunk) {
    size <<- size + length(chunk)
    if (size > 16L * 1024L * 1024L) { overflow <<- TRUE; return(FALSE) }
    chunks[[length(chunks) + 1L]] <<- chunk
    TRUE
  }, handle = handle)), error = function(e) {
    if (overflow) stop(.cl_error("Response exceeds 16 MiB", code = "response-too-large", action = action, kind = "protocol", outcome_unknown = submitted))
    stop(.cl_error("ClonoLoom connection failed or timed out; inspect state before resubmitting", code = "connection-failed", action = action, kind = "connection", outcome_unknown = submitted))
  })
  status <- response$status_code
  fail <- function(message, code) stop(.cl_error(message, code, status, action, submitted, "protocol"))
  if (overflow) fail("Response exceeds 16 MiB", "response-too-large")
  if (status %in% c(301L, 302L, 303L, 307L, 308L)) fail("Redirect refused; no token forwarded", "redirect-refused")
  if (is.null(response$type) || !grepl("^application/json([ ;]|$)", response$type, ignore.case = TRUE)) fail("Host did not return application/json", "invalid-response")
  raw <- do.call(c, c(list(raw()), chunks))
  answer <- tryCatch(.cl_decode(raw), error = function(e) fail("Host returned invalid JSON", "invalid-json"))
  if (!is.list(answer) || is.null(names(answer))) fail("Host response must be an object", "invalid-response")
  if (status != 200L) {
    code <- if (.cl_scalar_text(answer[["code", exact = TRUE]])) answer[["code", exact = TRUE]] else "http-error"
    message <- if (.cl_scalar_text(answer[["error", exact = TRUE]])) answer[["error", exact = TRUE]] else if (.cl_scalar_text(answer[["message", exact = TRUE]])) answer[["message", exact = TRUE]] else paste("HTTP", status)
    stop(.cl_error(message, code, status, action, submitted && status >= 500L, if (status %in% c(401L, 403L)) "permission" else "action"))
  }
  list(value = answer, raw = raw)
}
.cl_refresh <- function(cl) {
  info <- .cl_request(cl, "GET", "/api/remote/tools")$value
  expected <- list(api = .cl_api, catalogVersion = .cl_catalog, policyVersion = .cl_policy)
  for (key in names(expected)) {
    if (!inherits(info[[key]], "clonoloom_number") || !identical(unclass(info[[key]]), expected[[key]]))
      stop(.cl_error("Host API/catalog/policy differs; update host or client before actions", code = "incompatible-host", kind = "compatibility"))
  }
  if (!.cl_scalar_text(info[["version", exact = TRUE]]) || !inherits(info[["tools", exact = TRUE]], "clonoloom_array") ||
      !is.list(info[["allowed", exact = TRUE]]) || is.null(names(info[["allowed", exact = TRUE]])) || !isTRUE(info[["allowed", exact = TRUE]][["read", exact = TRUE]]) ||
      !all(vapply(info[["allowed", exact = TRUE]], function(x) is.logical(x) && length(x) == 1L && !is.na(x), logical(1))))
    stop(.cl_error("Invalid host catalog or permissions", code = "invalid-catalog", kind = "protocol"))
  info
}
cl_tools <- function(cl = cl_default()) .cl_refresh(cl)$tools
cl_permissions <- function(cl = cl_default()) .cl_refresh(cl)$allowed
cl_version <- function(cl = cl_default()) .cl_refresh(cl)$version
cl_call <- function(.action, ..., .args = list(), cl = cl_default()) {
  if (!.cl_scalar_text(.action) || !nzchar(.action) || nchar(.action) > 128L)
    stop(.cl_error("Action name must contain 1-128 characters", code = "invalid-arguments"))
  body <- tryCatch({
    args <- c(list(...), cl_object(.args))
    if (length(args) && (is.null(names(args)) || any(!nzchar(names(args))) || anyDuplicated(names(args)))) stop("Action arguments need unique names")
    args <- args[!vapply(args, inherits, logical(1), "clonoloom_omit")]
    .cl_encode(list(action = .action, args = cl_object(args), client = cl$client))
  }, error = function(e) stop(.cl_error(conditionMessage(e), code = "invalid-arguments", action = .action)))
  .cl_refresh(cl)
  response <- .cl_request(cl, "POST", "/api/remote/action", body, .action)
  answer <- response$value
  if (!is.logical(answer[["ok", exact = TRUE]]) || length(answer[["ok", exact = TRUE]]) != 1L ||
      ("message" %in% names(answer) && !.cl_scalar_text(answer[["message", exact = TRUE]])) ||
      ("code" %in% names(answer) && !.cl_scalar_text(answer[["code", exact = TRUE]])))
    stop(.cl_error("Invalid action result envelope", "invalid-response", 200L, .action, TRUE, "protocol"))
  if (!answer[["ok", exact = TRUE]]) {
    code <- if (is.null(answer[["code", exact = TRUE]])) "action-failed" else answer[["code", exact = TRUE]]
    known <- c("rejected", "stale-project", "busy", "invalid-arguments", "permission-denied", "unauthorized", "unknown-action")
    stop(.cl_error(if (is.null(answer[["message", exact = TRUE]])) "Action failed" else answer[["message", exact = TRUE]], code, 200L, .action,
                   !code %in% known, if (code %in% c("permission-denied", "unauthorized")) "permission" else "action"))
  }
  structure(list(action = .action, message = if (is.null(answer[["message", exact = TRUE]])) "" else answer[["message", exact = TRUE]],
                 data = answer[["data", exact = TRUE]], raw_json = response$raw), class = "clonoloom_result")
}
cl_native <- function(result) {
  if (!is.list(result$data) || !.cl_scalar_text(result$data[["resultJSON", exact = TRUE]])) stop("Result has no native resultJSON")
  tryCatch(.cl_decode(result$data[["resultJSON", exact = TRUE]]), error = function(e)
    stop(.cl_error("Invalid native resultJSON", "invalid-json", action = result$action, kind = "protocol")))
}
cl_records <- function(result, key = NULL) {
  value <- if (is.list(result$data) && "resultJSON" %in% names(result$data)) cl_native(result) else result$data
  if (!is.null(key)) {
    if (!.cl_scalar_text(key) || !key %in% names(value)) stop("No such record key")
    value <- value[[match(key, names(value))]]
  } else if (is.list(value) && !is.null(names(value))) value <- value[["rows", exact = TRUE]]
  if (!inherits(value, "clonoloom_array") || !all(vapply(value, function(row) is.list(row) && !is.null(names(row)), logical(1))))
    stop("Select a record array explicitly with cl_records(result, key)")
  value
}
as.data.frame.clonoloom_result <- function(x, row.names = NULL, optional = FALSE, key = NULL, ...) {
  rows <- cl_records(x, key)
  keys <- unique(unlist(lapply(rows, names), use.names = FALSE))
  columns <- lapply(keys, function(key) I(lapply(rows, function(row) {
    index <- match(key, names(row))
    if (is.na(index)) NULL else row[[index]]
  })))
  names(columns) <- keys
  if (is.null(row.names)) row.names <- seq_along(rows)
  if (length(row.names) != length(rows) || anyNA(row.names) || anyDuplicated(row.names)) stop("Invalid row.names")
  structure(columns, class = "data.frame", row.names = as.character(row.names))
}
print.clonoloom <- function(x, ...) {
  cat("<ClonoLoom ", x$version, " at ", x$url, ">\n", sep = ""); invisible(x)
}
print.clonoloom_result <- function(x, ...) {
  cat(x$action, ": ", x$message, "\n", sep = ""); invisible(x)
}
