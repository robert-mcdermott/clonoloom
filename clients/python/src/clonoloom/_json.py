"""Bounded JSON with native integer/decimal values, never a float decoding round trip.

Uses Python's documented parse_int/parse_float hooks. A small recursive encoder emits finite
Decimal values as JSON number tokens, not quoted strings. No scientific computation lives here.
"""
import json
import math
from decimal import Decimal, DecimalException

MAX_RESPONSE = 16 << 20
MAX_REQUEST = 1 << 20


class _Unset:
    def __repr__(self):
        return 'UNSET'


UNSET = _Unset()


def loads(text):
    """Exact values; duplicate keys and non-JSON constants are errors. Keep raw text for bytes."""
    def pairs(items):
        obj = {}
        for key, value in items:
            if key in obj:
                raise ValueError('Duplicate JSON object key')
            obj[key] = value
        return obj

    def decimal(value):
        try:
            number = Decimal(value)
            if not number.is_finite():
                raise ValueError("JSON decimal is outside the finite decoder range")
            return number
        except DecimalException as error:
            raise ValueError("JSON decimal exponent is outside the decoder range") from error

    def constant(value):
        raise ValueError('Non-finite JSON number: ' + value)

    def depth(value, level=0):
        if level > 64:
            raise ValueError('JSON nesting exceeds 64 levels')
        if isinstance(value, dict):
            for item in value.values():
                depth(item, level + 1)
        elif isinstance(value, list):
            for item in value:
                depth(item, level + 1)

    if not isinstance(text, (str, bytes)):
        raise TypeError('Expected JSON text or UTF-8 bytes')
    if isinstance(text, bytes):
        text = text.decode('utf-8')
    if len(text.encode('utf-8')) > MAX_RESPONSE:
        raise ValueError('JSON exceeds the 16 MiB response bound')
    value = json.loads(text, parse_int=int, parse_float=decimal,
                       parse_constant=constant, object_pairs_hook=pairs)
    depth(value)
    return value


def dumps(value, limit=MAX_REQUEST):
    """Encode JSON values and finite Decimal numbers without changing their values."""
    chunks = []
    size = 0

    def emit(text):
        nonlocal size
        raw = text.encode('utf-8')
        size += len(raw)
        if size > limit:
            raise ValueError('JSON exceeds the request size bound')
        chunks.append(raw)

    def encode(item, level=0):
        if level > 64:
            raise ValueError('JSON nesting exceeds 64 levels (or contains a cycle)')
        if item is None:
            emit('null')
        elif type(item) is bool:
            emit('true' if item else 'false')
        elif type(item) is int:
            emit(str(item))
        elif isinstance(item, Decimal):
            if not item.is_finite():
                raise ValueError('Only finite Decimal values are JSON numbers')
            emit(str(item))
        elif type(item) is float:
            if not math.isfinite(item):
                raise ValueError('Only finite float values are JSON numbers')
            emit(json.dumps(item, allow_nan=False))
        elif type(item) is str:
            emit(json.dumps(item, ensure_ascii=True))
        elif type(item) is list:
            emit('[')
            for i, child in enumerate(item):
                if i:
                    emit(',')
                encode(child, level + 1)
            emit(']')
        elif type(item) is dict:
            emit('{')
            for i, (key, child) in enumerate(item.items()):
                if type(key) is not str:
                    raise TypeError('JSON object keys must be strings')
                if i:
                    emit(',')
                emit(json.dumps(key, ensure_ascii=True)); emit(':')
                encode(child, level + 1)
            emit('}')
        else:
            raise TypeError('Expected JSON values or Decimal; use lists for arrays and omit UNSET only at the argument level')

    encode(value)
    return b''.join(chunks)
