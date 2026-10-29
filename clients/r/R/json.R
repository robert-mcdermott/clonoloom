# Exact JSON syntax, not a scientific arithmetic implementation. jsonlite handles
# string escaping/Unicode only; numeric tokens never pass through a double decoder.
.cl_number_pattern <- "\\A-?(0|[1-9][0-9]*)(\\.[0-9]+)?([eE][+-]?[0-9]+)?\\z"
.cl_scalar_text <- function(x) is.character(x) && !is.object(x) && length(x) == 1L && !is.na(x)
cl_number <- function(token) {
  if (!.cl_scalar_text(token) || !grepl(.cl_number_pattern, token, perl = TRUE))
    stop("Supply one complete JSON number token as a character string", call. = FALSE)
  structure(token, class = "clonoloom_number")
}
as.double.clonoloom_number <- function(x, ...) {
  warning("Converting exact JSON to R double may round, overflow or underflow", call. = FALSE)
  as.double(unclass(x))
}
Ops.clonoloom_number <- function(e1, e2) stop("Exact JSON tokens are not arithmetic values; convert explicitly", call. = FALSE)
Math.clonoloom_number <- function(x, ...) stop("Exact JSON tokens are not arithmetic values; convert explicitly", call. = FALSE)
Summary.clonoloom_number <- function(..., na.rm = FALSE) stop("Exact JSON tokens are not arithmetic values; convert explicitly", call. = FALSE)
cl_omit <- function() structure(list(), class = "clonoloom_omit")
cl_array <- function(x = list()) {
  if ((!is.list(x) && !is.atomic(x)) || !is.null(names(x)) ||
      (is.object(x) && !inherits(x, "clonoloom_array")))
    stop("An array needs an unnamed list or plain vector", call. = FALSE)
  structure(as.list(x), class = "clonoloom_array")
}
cl_object <- function(x = list()) {
  if (!is.list(x) || is.object(x) || (length(x) && (is.null(names(x)) || anyNA(names(x)) || anyDuplicated(names(x)))))
    stop("An object needs a uniquely named plain list", call. = FALSE)
  if (!length(x)) names(x) <- character(0)
  x
}
.cl_array_arg <- function(x) {
  if (is.null(x) || inherits(x, "clonoloom_omit")) x else cl_array(x)
}

.cl_decode <- function(input, limit = 16L * 1024L * 1024L) {
  if (is.raw(input)) input <- rawToChar(input)
  if (!.cl_scalar_text(input) || nchar(input, type = "bytes") > limit ||
      is.na(iconv(input, from = "UTF-8", to = "UTF-8"))) stop("Invalid or oversized UTF-8 JSON")
  # Tokenize once. Exact reconstruction rejects gaps, illegal escapes and comments.
  pattern <- '"(?:[^"\\\\\\x00-\\x1f]|\\\\(?:["\\\\/bfnrt]|u[0-9a-fA-F]{4}))*"|-?(?:0|[1-9][0-9]*)(?:\\.[0-9]+)?(?:[eE][+-]?[0-9]+)?|true|false|null|[{}\\[\\],:]|[ \\t\\r\\n]+'
  tokens <- regmatches(input, gregexpr(pattern, input, perl = TRUE, useBytes = TRUE))[[1]]
  if (!identical(charToRaw(paste0(tokens, collapse = "")), charToRaw(input))) stop("Invalid JSON token")
  Encoding(tokens) <- "UTF-8"
  tokens <- tokens[!grepl("^[ \\t\\r\\n]+$", tokens, perl = TRUE)]
  at <- 1L
  take <- function() {
    if (at > length(tokens)) stop("Incomplete JSON")
    token <- tokens[[at]]; at <<- at + 1L; token
  }
  peek <- function() if (at <= length(tokens)) tokens[[at]] else ""
  string <- function(token) {
    if (!startsWith(token, '"')) stop("Object key must be a string")
    if (!grepl("\\", token, fixed = TRUE)) return(substring(token, 2L, nchar(token) - 1L))
    # Validate Unicode escapes before jsonlite can discard an unpaired surrogate/NUL.
    chars <- strsplit(token, "", fixed = TRUE)[[1]]; i <- 2L
    while (i < length(chars)) {
      if (chars[[i]] == "\\") {
        if (chars[[i + 1L]] == "u") {
          point <- strtoi(paste0(chars[seq.int(i + 2L, i + 5L)], collapse = ""), 16L)
          if (point == 0L) stop("R strings cannot preserve NUL")
          if (point >= 0xD800L && point <= 0xDBFFL) {
            if (i + 11L >= length(chars) || chars[[i + 6L]] != "\\" || chars[[i + 7L]] != "u") stop("Unpaired Unicode surrogate")
            low <- strtoi(paste0(chars[seq.int(i + 8L, i + 11L)], collapse = ""), 16L)
            if (is.na(low) || low < 0xDC00L || low > 0xDFFFL) stop("Unpaired Unicode surrogate")
            i <- i + 6L
          } else if (point >= 0xDC00L && point <= 0xDFFFL) stop("Unpaired Unicode surrogate")
          i <- i + 6L
        } else i <- i + 2L
      } else i <- i + 1L
    }
    value <- jsonlite::fromJSON(token, simplifyVector = FALSE)
    if (!.cl_scalar_text(value) || is.na(iconv(value, from = "UTF-8", to = "UTF-8"))) stop("Invalid JSON string")
    value
  }
  parse <- function(depth = 0L) {
    if (depth > 64L) stop("JSON nesting exceeds 64")
    token <- take()
    if (token == "{") {
      out <- cl_object(); keys <- character()
      if (peek() == "}") { take(); return(out) }
      repeat {
        key <- string(take())
        if (key %in% keys) stop("Duplicate JSON object key")
        keys <- c(keys, key)
        if (take() != ":") stop("Missing colon")
        out[length(out) + 1L] <- list(parse(depth + 1L)); names(out) <- keys
        sep <- take()
        if (sep == "}") return(out)
        if (sep != ",") stop("Missing object separator")
      }
    }
    if (token == "[") {
      out <- cl_array()
      if (peek() == "]") { take(); return(out) }
      repeat {
        out[length(out) + 1L] <- list(parse(depth + 1L))
        sep <- take()
        if (sep == "]") return(out)
        if (sep != ",") stop("Missing array separator")
      }
    }
    if (startsWith(token, '"')) return(string(token))
    if (token == "null") return(NULL)
    if (token == "true") return(TRUE)
    if (token == "false") return(FALSE)
    cl_number(token)
  }
  answer <- parse()
  if (at <= length(tokens)) stop("Trailing JSON")
  answer
}

.cl_encode <- function(value, limit = 1024L * 1024L) {
  used <- 0L
  emit <- function(text) {
    used <<- used + nchar(text, type = "bytes")
    if (used > limit) stop("JSON request exceeds its byte limit")
    text
  }
  string <- function(x) {
    if (is.na(x) || is.na(iconv(x, from = "", to = "UTF-8"))) stop("Invalid string or NA; use NULL for JSON null")
    emit(as.character(jsonlite::toJSON(enc2utf8(x), auto_unbox = TRUE)))
  }
  encode <- function(x, depth = 0L) {
    if (depth > 64L) stop("JSON nesting exceeds 64")
    if (inherits(x, "clonoloom_number")) return(emit(unclass(cl_number(unclass(x)))))
    if (inherits(x, "clonoloom_omit")) stop("Omission is only allowed for top-level action arguments")
    if (is.null(x)) return(emit("null"))
    if (is.object(x) && !inherits(x, "clonoloom_array")) stop("Unsupported R class; supply plain JSON values")
    if (is.list(x)) {
      object <- !is.null(names(x))
      if (object && (inherits(x, "clonoloom_array") || anyNA(names(x)) || anyDuplicated(names(x)))) stop("Invalid or duplicate object keys")
      open <- emit(if (object) "{" else "[")
      parts <- vapply(seq_along(x), function(i) {
        sep <- if (i > 1L) emit(",") else ""
        key <- if (object) paste0(string(names(x)[[i]]), emit(":")) else ""
        paste0(sep, key, encode(x[[i]], depth + 1L))
      }, character(1))
      return(paste0(open, paste0(parts, collapse = ""), emit(if (object) "}" else "]")))
    }
    if (!is.null(names(x)) || !(is.character(x) || is.logical(x) || is.integer(x) || is.double(x))) stop("Unsupported JSON value")
    if (length(x) != 1L) return(encode(as.list(x), depth))
    if (anyNA(x)) stop("NA/NaN cannot stand for evidence; use NULL explicitly")
    if (is.character(x)) return(string(x))
    if (is.logical(x)) return(emit(if (x) "true" else "false"))
    if (!is.finite(x)) stop("Non-finite JSON number")
    if (is.integer(x)) return(emit(as.character(x)))
    if (x == trunc(x) && abs(x) > 9007199254740991) stop("Unsafe R double integer; use cl_number with original text")
    emit(gsub(",", ".", sprintf("%.17g", x), fixed = TRUE))
  }
  charToRaw(enc2utf8(encode(value)))
}
