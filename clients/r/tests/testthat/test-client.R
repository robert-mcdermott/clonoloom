test_that("exact JSON tokens, shape and missingness survive round trips", {
  text <- '{"n":9007199254740993,"d":0.12345678901234567890123456789,"exp":-0.00e+9999,"x":[null,true,false,{},[],"雪"],"null":null}'
  value <- .cl_decode(text)
  expect_identical(as.character(value$n), "9007199254740993")
  expect_identical(as.character(value$d), "0.12345678901234567890123456789")
  expect_identical(rawToChar(.cl_encode(value)), text)
  expect_identical(.cl_decode(.cl_encode(value)), value)
  expect_true("null" %in% names(value)); expect_null(value$null)
  expect_identical(rawToChar(.cl_encode(cl_array("one"))), '["one"]')
  expect_identical(rawToChar(.cl_encode(cl_object())), '{}')
  expect_identical(rawToChar(.cl_encode(list())), '[]')
  expect_identical(rawToChar(.cl_encode(c(1L, 2L))), '[1,2]')
  expect_identical(rawToChar(.cl_encode(list(x = NULL))), '{"x":null}')
  expect_warning(as.numeric(value$n), "may round")
  expect_error(value$n + 1, "not arithmetic")
  expect_error(value$n > cl_number("1"), "not arithmetic")
  expect_error(sum(value$n), "not arithmetic")
  expect_error(log(value$n), "not arithmetic")
  set.seed(20261028)
  for (i in seq_len(1000)) {
    digits <- paste0(sample(0:9, 40, replace = TRUE), collapse = "")
    token <- paste0(sample(c("", "-"), 1), sample(1:9, 1), digits, ".", digits, "e", sample(-1000:1000, 1))
    expect_identical(rawToChar(.cl_encode(.cl_decode(token))), token)
  }
})

test_that("JSON syntax, encoding and bounds fail rather than repairing evidence", {
  bad <- c('', '{} x', '{"a":1,"a":2}', '{"a":1,"\\u0061":2}', '[1,]', '{"a":1,}',
           '01', '1.', '+1', '.1', 'NaN', 'Infinity', 'true false', '"\\x01"',
           '"\\u0000"', '"\\ud800"', '"\\udc00"')
  for (text in bad) expect_error(.cl_decode(text), info = text)
  expect_identical(.cl_decode('"\\\\u0000"'), '\\u0000')
  expect_identical(.cl_decode('"\\ud83d\\ude00"'), '\U0001f600')
  expect_error(.cl_decode(as.raw(c(34, 255, 34))))
  expect_error(.cl_decode(paste0(strrep('[', 65), '0', strrep(']', 65))), "nesting")
  expect_error(.cl_decode('123', limit = 2L), "oversized")
  for (x in list(NA, NaN, Inf, factor("x"), Sys.Date(), 9007199254740992, list(cl_omit()), c(a = 1L), list(a = 1, a = 2))) expect_error(.cl_encode(x))
  expect_error(.cl_encode(strrep("x", 1024 * 1024)), "limit")
  expect_error(cl_number("1.2x")); expect_error(cl_number(1)); expect_error(cl_array(c(a = 1)))
  expect_error(cl_number("1\n")); expect_error(.cl_url("http://127.0.0.1\n"))
  expect_error(cl_object(list(1))); expect_error(cl_array(cl_number("1")))
  for (x in c(.1, .Machine$double.xmin, .Machine$double.eps, -2.9)) {
    token <- rawToChar(.cl_encode(x))
    expect_identical(jsonlite::fromJSON(token), x)
  }
})

test_that("record frames retain tokens, lists, nulls and empty records", {
  result <- structure(list(data = .cl_decode('{"rows":[{"n":9007199254740993,"p":0.1234567890123456789,"x":null,"b":true,"a":[1]},{"n":null}]}')), class = "clonoloom_result")
  frame <- as.data.frame(result)
  expect_identical(frame$n[[1]], cl_number("9007199254740993"))
  expect_identical(frame$p[[1]], cl_number("0.1234567890123456789"))
  expect_null(frame$n[[2]]); expect_null(frame$p[[2]])
  expect_true(frame$b[[1]]); expect_s3_class(frame$a[[1]], "clonoloom_array")
  expect_false("p" %in% names(cl_records(result)[[2]]))
  result$data <- .cl_decode('{"rows":[{},{}]}')
  expect_identical(dim(as.data.frame(result)), c(2L, 0L))
  result$data <- .cl_decode('{"rows":[]}')
  expect_identical(dim(as.data.frame(result)), c(0L, 0L))
  result$data <- .cl_decode('{"unrelated":[{}]}')
  expect_error(cl_records(result)); expect_length(cl_records(result, "unrelated"), 1)
  result$data <- list(resultJSON = '{"rows":[{"n":1}]}')
  expect_identical(cl_native(result)$rows[[1]]$n, cl_number("1"))
  result$data <- list(resultJSON_suffix = '{"rows":[]}')
  expect_error(cl_native(result), "no native")
  result$data <- .cl_decode('{"rows":[{"":1},{}]}')
  expect_identical(as.data.frame(result)[[1]][[1]], cl_number("1"))
  expect_null(as.data.frame(result)[[1]][[2]])
  result$data <- .cl_decode('{"":[{}]}')
  expect_length(cl_records(result, ""), 1L)
})

# Package checks use a controlled local Node server when run by clients/test-r.mjs.
# Offline package checks still cover the codec, generated wrappers and frame/discovery logic.
server <- function() {
  url <- Sys.getenv("CLONOLOOM_FAKE_URL")
  if (!nzchar(url)) skip("Run node clients/test-r.mjs for real HTTP controls")
  url
}
control <- function(config = list()) {
  handle <- curl::new_handle(proxy = "", postfields = charToRaw(as.character(jsonlite::toJSON(config, auto_unbox = TRUE))))
  curl::handle_setheaders(handle, 'Content-Type' = 'application/json')
  invisible(curl::curl_fetch_memory(paste0(server(), "/control"), handle))
}
requests <- function() jsonlite::fromJSON(rawToChar(curl::curl_fetch_memory(paste0(server(), "/requests"), curl::new_handle(proxy = ""))$content), simplifyVector = FALSE)
fake_connection <- function() cl_connect(server(), "test-secret", client = "R tests")

test_that("every generated wrapper uses the shared action contract", {
  seen <- NULL
  local_mocked_bindings(.cl_refresh = function(cl) NULL,
    .cl_request = function(cl, method, path, body, action) {
      seen <<- .cl_decode(body)
      list(value = list(ok = TRUE, message = "ok", data = NULL), raw = charToRaw('{}'))
    })
  con <- structure(list(client = "test"), class = "clonoloom")
  functions <- ls(asNamespace("clonoloom"), pattern = "^cl_")
  core <- c("cl_connect", "cl_default", "cl_find_connection", "cl_call", "cl_tools", "cl_permissions", "cl_version", "cl_native", "cl_records", "cl_number", "cl_omit", "cl_array", "cl_object")
  functions <- setdiff(functions, core)
  expect_length(functions, 58)
  for (name in functions) {
    fn <- get(name, asNamespace("clonoloom"))
    formal <- formals(fn)
    args <- list(cl = con)
    for (key in setdiff(names(formal), "cl")) {
      if (identical(formal[[key]], quote(expr = ))) args[[key]] <- "required"
    }
    do.call(fn, args)
    expect_identical(seen$action, sub("^cl_", "", name))
    expect_length(seen$args, length(args) - 1L)
  }
  cl_query_clonotypes(limit = NULL, cl = con)
  expect_true("limit" %in% names(seen$args)); expect_null(seen$args$limit)
  cl_query_clonotypes(limit = cl_omit(), cl = con)
  expect_length(seen$args, 0)
  cl_open_data("one", cl = con)
  expect_s3_class(seen$args$paths, "clonoloom_array"); expect_length(seen$args$paths, 1)
  cl_open_data(character(), cl = con)
  expect_s3_class(seen$args$paths, "clonoloom_array"); expect_length(seen$args$paths, 0)
  expect_error(cl_call("get_state", x = 1, .args = list(x = 2), cl = con), class = "clonoloom_argument_error")
})

test_that("discovery and explicit overrides never borrow unrelated tokens", {
  folder <- tempfile(); dir.create(folder); on.exit(unlink(folder, recursive = TRUE))
  writeLines('{"url":"http://127.0.0.1:9991","token":"file-secret"}', file.path(folder, "remote.json"))
  withr::local_envvar(c(CLONOLOOM_URL = "http://127.0.0.1:9992", CLONOLOOM_TOKEN = "env-secret", CLONOLOOM_DATA_DIR = folder))
  expect_identical(cl_find_connection(folder)$token, "file-secret")
  expect_identical(cl_find_connection()$token, "env-secret")
  local_mocked_bindings(.cl_refresh = function(cl) list(version = "test"))
  explicit <- cl_connect("http://127.0.0.1:9993")
  expect_null(explicit$token)
  expect_null(cl_connect(data_dir = folder, token = NULL)$token)
  expect_identical(cl_connect(data_dir = folder)$token, "file-secret")
  expect_false(grepl("secret", paste(capture.output(print(cl_connect(data_dir = folder))), collapse = "")))
  withr::local_envvar(c(CLONOLOOM_URL = ""))
  expect_identical(cl_find_connection()$token, "file-secret")
  expect_error(cl_find_connection(file.path(folder, "missing")), class = "clonoloom_connection_error")
  writeLines('{bad', file.path(folder, "remote.json"))
  expect_error(cl_find_connection(folder), class = "clonoloom_connection_error")
  writeLines(strrep(" ", 65537L), file.path(folder, "remote.json"))
  expect_error(cl_find_connection(folder), class = "clonoloom_connection_error")
  withr::local_envvar(c(CLONOLOOM_DATA_DIR = ""))
  local_mocked_bindings(.cl_default_data_dir = function() file.path(folder, "absent"))
  expect_identical(cl_find_connection(), list(url = "http://127.0.0.1:8860", token = NULL))
  for (url in c("https://127.0.0.1", "http://example.org", "http://127.0.0.1/x", "http://u:p@127.0.0.1", "http://127.0.0.1:0", "http://127.0.0.1?a", "http://127.0.0.1#x", "http://127.00.0.1", "http://127.0.0.1:99999")) expect_error(.cl_url(url))
  expect_identical(.cl_url("http://localhost:8860/"), "http://127.0.0.1:8860")
  expect_identical(.cl_url("http://[::1]:8860"), "http://[::1]:8860")
  expect_error(.cl_token("x\ny")); expect_error(cl_connect(timeout = 0))
  expect_error(cl_connect(timeout = 2147484))
})

test_that("real HTTP retains bytes and sends exact tokens without retry", {
  control()
  cl <- fake_connection()
  result <- cl_query_clonotypes(where = list(field = "total", op = "gte", value = cl_number("9007199254740993")), cl = cl)
  expect_identical(cl_native(result)$rows[[1]]$total, cl_number("9007199254740993"))
  expect_identical(cl_native(result)$rows[[1]]$frequency, cl_number("0.12345678901234567890123456789"))
  expect_true(is.raw(result$raw_json))
  req <- requests()
  expect_length(req, 3)
  expect_null(req[[1]]$token); expect_identical(req[[3]]$token, "test-secret")
  expect_match(req[[3]]$body, '"value":9007199254740993', fixed = TRUE)
  expect_identical(cl_permissions(cl), list(read = TRUE, compute = TRUE))
  for (version in c('3', 'null', 'true', '2.0', '"2"')) {
    control(list(api = version))
    expect_error(cl_get_state(cl = cl), class = "clonoloom_compatibility_error")
    expect_length(requests(), 1)
  }
  for (code in c("catalogVersion", "policyVersion")) {
    control(stats::setNames(list('9'), code))
    expect_error(cl_get_state(cl = cl), class = "clonoloom_compatibility_error")
    expect_length(requests(), 1)
  }
})

test_that("HTTP failures, malformed evidence, redirects, bounds and timeouts are explicit", {
  control(); cl <- fake_connection()
  for (status in c(400L, 401L, 403L, 404L, 503L)) {
    control(list(status = status, body = '{"error":"refused","code":"test-refusal"}'))
    err <- tryCatch(cl_get_state(cl = cl), error = identity)
    expect_s3_class(err, if (status %in% c(401L, 403L)) "clonoloom_permission_error" else "clonoloom_action_error")
    expect_identical(err$status, status); expect_identical(err$code, "test-refusal")
    expect_identical(err$outcome_unknown, status >= 500L); expect_length(requests(), 2)
  }
  for (body in c('[]', '{"ok":false,"code":[]}', '{"ok":null}', '{"ok":true,"data":{"x":1,"x":2}}', '{"ok":true,"data":NaN}', '{bad')) {
    control(list(body = body)); expect_error(cl_get_state(cl = cl), class = "clonoloom_protocol_error")
  }
  control(list(body = '{"ok":false,"code":"rejected","message":"No"}'))
  err <- tryCatch(cl_get_state(cl = cl), error = identity)
  expect_false(err$outcome_unknown); expect_identical(err$code, "rejected")
  control(list(status = 302L)); expect_error(cl_get_state(cl = cl), "Redirect refused")
  expect_length(requests(), 2)
  control(list(type = "text/html")); expect_error(cl_get_state(cl = cl), "application/json")
  control(list(oversize = TRUE)); expect_error(cl_get_state(cl = cl), "16 MiB")
  control(list(delay = 1000L)); cl$timeout <- .1
  err <- tryCatch(cl_get_state(cl = cl), error = identity)
  expect_s3_class(err, "clonoloom_connection_error"); expect_true(err$outcome_unknown)
  expect_length(requests(), 2)
  control(); cl$timeout <- 2
  withr::local_envvar(c(http_proxy = "http://127.0.0.1:1", HTTP_PROXY = "http://127.0.0.1:1", ALL_PROXY = "http://127.0.0.1:1"))
  expect_s3_class(cl_get_state(cl = cl), "clonoloom_result")
  control()
  expect_error(cl_call("get_state", text = strrep("x", 1024 * 1024), cl = cl), class = "clonoloom_argument_error")
  expect_length(requests(), 0)
})
