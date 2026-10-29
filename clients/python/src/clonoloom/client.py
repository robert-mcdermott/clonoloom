"""Local HTTP client, adapted from Apache-2.0 CytoWeave/MaveScape client.py patterns.

ClonoLoom's host grants and window review remain authoritative. No auto-acceptance, retries,
process launch, scientific engine, or MCP dependency. Exact JSON behavior is in _json.py.
"""
from __future__ import annotations

import http.client
import ipaddress
import math
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any

from ._json import MAX_RESPONSE, UNSET, dumps, loads
from ._tools import API_VERSION, CATALOG_VERSION, POLICY_VERSION, Tools

DEFAULT_URL = 'http://127.0.0.1:8860'
TOKEN_HEADER = 'X-ClonoLoom-Token'


class ClonoLoomError(Exception):
    """Typed failure: code, HTTP status (0 for local errors), action, and outcome_unknown.

    outcome_unknown means a submitted action may have completed. Never retry automatically.
    """
    def __init__(self, message, *, code='client-error', status=0, action=None, outcome_unknown=False):
        super().__init__(message)
        self.code, self.status, self.action = code, status, action
        self.outcome_unknown = outcome_unknown


class ConnectionError(ClonoLoomError):
    """Discovery, connection or timeout failure."""


class ProtocolError(ClonoLoomError):
    """Malformed, oversized or incompatible host response."""


class CompatibilityError(ProtocolError):
    """Host API/catalog/policy cannot safely be used by this client."""


class ActionError(ClonoLoomError):
    """Host refused or failed an action; code retains the host reason."""


class PermissionDenied(ActionError):
    """Host requires an explicit capability grant or token."""


class ArgumentError(ClonoLoomError):
    """Arguments cannot be represented faithfully as bounded JSON."""


class Result:
    """Action message/data plus original UTF-8 HTTP response bytes in raw_json.

    Numeric values are int/Decimal. native() decodes data.resultJSON without float conversion;
    it does not discard that original string or alter the response bytes.
    """
    def __init__(self, action, message, data, raw_json=b''):
        self.action, self.message, self.data, self.raw_json = action, message, data, raw_json

    def __repr__(self):
        return f'<clonoloom.Result {self.action}: {self.message}>'

    def __getitem__(self, key):
        return self.data[key]

    def native(self):
        if not isinstance(self.data, dict) or not isinstance(self.data.get('resultJSON'), str):
            raise KeyError(f'{self.action} has no native resultJSON')
        try:
            return loads(self.data['resultJSON'])
        except (ValueError, TypeError, RecursionError) as error:
            raise ProtocolError('Invalid native resultJSON', code='invalid-json', action=self.action) from error

    def records(self, key=None):
        """Explicit key, a root list, or unambiguous rows; no guessing among arbitrary lists."""
        value = self.native() if isinstance(self.data, dict) and 'resultJSON' in self.data else self.data
        if key is not None:
            if not isinstance(value, dict) or key not in value:
                raise KeyError(key)
            value = value[key]
        elif isinstance(value, dict):
            value = value.get('rows')
        if not isinstance(value, list) or not all(isinstance(row, dict) for row in value):
            raise TypeError('Select a list of record objects explicitly with records(key)')
        return value

    def frame(self, key=None):
        """Optional pandas frame with object columns: int/Decimal/None values stay exact.

        Missing record keys become None only in this convenience view; records/raw_json preserve
        absent-vs-null distinctions. Later pandas calculations/coercion are the caller's choice.
        """
        try:
            import pandas
        except ImportError as error:
            raise ImportError('frame() needs pandas; use uv add "clonoloom[pandas]" or records()') from error
        rows = self.records(key)
        columns = dict.fromkeys(k for row in rows for k in row)
        return pandas.DataFrame({k: pandas.Series([row.get(k) for row in rows], dtype=object)
                                 for k in columns}, index=range(len(rows)))


def default_data_dir():
    """Go os.UserConfigDir convention, matching ClonoLoom's host."""
    home = Path.home()
    if sys.platform == 'win32':
        return Path(os.environ['APPDATA']) / 'ClonoLoom' if os.environ.get('APPDATA') else home / '.clonoloom'
    if sys.platform == 'darwin':
        return home / 'Library' / 'Application Support' / 'ClonoLoom'
    return Path(os.environ.get('XDG_CONFIG_HOME') or home / '.config') / 'ClonoLoom'


def _url(value):
    try:
        if not isinstance(value, str) or any(ord(c) < 33 or ord(c) == 127 for c in value):
            raise ValueError()
        u = urllib.parse.urlsplit(value)
        if u.scheme != 'http' or u.username is not None or u.password is not None or u.query or u.fragment or u.path not in ('', '/'):
            raise ValueError()
        host = '127.0.0.1' if u.hostname == 'localhost' else u.hostname
        if not ipaddress.ip_address(host).is_loopback:
            raise ValueError()
        port = u.port if u.port is not None else 80
        if not 1 <= port <= 65535:
            raise ValueError()
        return f'http://[{host}]:{port}' if ':' in host else f'http://{host}:{port}'
    except (TypeError, ValueError):
        raise ConnectionError('Use a loopback HTTP URL without credentials, path, query or fragment', code='invalid-url') from None


def _token(value):
    if value is not None and (not isinstance(value, str) or len(value) > 1024 or any(ord(c) < 33 or ord(c) > 126 for c in value)):
        raise ConnectionError('Invalid local connection token', code='invalid-token')
    return value


def find_connection(data_dir=None):
    """Explicit library > environment URL > environment/default library > read-only fallback.

    An explicit library must contain a valid remote.json and ignores environment URL/token.
    An environment URL uses only its environment token. A missing default connection file
    falls back to the default address without a token; malformed files never fall back.
    """
    if data_dir is None and 'CLONOLOOM_URL' in os.environ:
        return {'url': _url(os.environ['CLONOLOOM_URL']), 'token': _token(os.environ.get('CLONOLOOM_TOKEN'))}
    explicit = data_dir is not None or 'CLONOLOOM_DATA_DIR' in os.environ
    folder = Path(data_dir if data_dir is not None else os.environ.get('CLONOLOOM_DATA_DIR', default_data_dir())).expanduser()
    try:
        with (folder / 'remote.json').open('rb') as stream:
            raw = stream.read(65537)
        if len(raw) > 65536:
            raise ValueError('Connection file exceeds 64 KiB')
        info = loads(raw)
        if not isinstance(info, dict) or 'url' not in info:
            raise ValueError('Connection file needs an object with a URL')
        return {'url': _url(info['url']), 'token': _token(info.get('token'))}
    except FileNotFoundError:
        if not explicit:
            return {'url': DEFAULT_URL, 'token': None}
        raise ConnectionError('No remote.json in the requested library; start ClonoLoom with --remote-control', code='discovery-failed') from None
    except (OSError, ValueError, TypeError, RecursionError) as error:
        raise ConnectionError('Cannot read a valid remote.json from the requested library', code='discovery-failed') from error


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class ClonoLoom(Tools):
    """Connection to the normal local host. Use connect() to check it immediately.

    Explicit URL never borrows a discovered token. token=None/'' disables token use;
    UNSET uses the matching discovered connection token. Actions check compatibility before
    posting. This client neither changes launch grants nor accepts review proposals.
    """
    def __init__(self, url=None, token=UNSET, *, client='Python', timeout=660, data_dir=None):
        found = find_connection(data_dir) if url is None else {'url': url, 'token': None}
        self._url = _url(found['url'])
        self.token = _token(found['token'] if token is UNSET else token)
        if not isinstance(client, str) or not client or len(client) > 60 or any(ord(c) < 32 or ord(c) == 127 for c in client):
            raise ArgumentError('client must be a printable label of 1–60 characters', code='invalid-client')
        if isinstance(timeout, bool) or not isinstance(timeout, (int, float)) or not math.isfinite(timeout) or timeout <= 0:
            raise ArgumentError('timeout must be finite and positive', code='invalid-timeout')
        self.client, self.timeout = client, timeout
        self._opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), _NoRedirect())
        self._catalog = None

    @property
    def url(self):
        return self._url

    def __repr__(self):
        return f'<clonoloom.ClonoLoom {self.url}>'

    def _request(self, method, path, body=None, action=None):
        headers = {'Accept': 'application/json'}
        if body is not None:
            headers['Content-Type'] = 'application/json'
        if method == 'POST' and self.token:
            headers[TOKEN_HEADER] = _token(self.token)
        req = urllib.request.Request(self.url + path, data=body, method=method, headers=headers)
        submitted = method == 'POST'
        try:
            try:
                response = self._opener.open(req, timeout=self.timeout)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                status = response.code
                raw = response.read(MAX_RESPONSE + 1)
                if len(raw) > MAX_RESPONSE:
                    raise ProtocolError('Response exceeds 16 MiB; action may have completed', code='response-too-large', status=status, action=action, outcome_unknown=submitted)
                if status in (301, 302, 303, 307, 308):
                    raise ProtocolError('Redirect refused; no token was forwarded', code='redirect-refused', status=status, action=action, outcome_unknown=submitted)
                if response.headers.get_content_type() != 'application/json':
                    raise ProtocolError('Host did not return application/json', code='invalid-response', status=status, action=action, outcome_unknown=submitted)
                try:
                    value = loads(raw)
                except (ValueError, TypeError, RecursionError) as error:
                    raise ProtocolError('Host returned invalid JSON', code='invalid-json', status=status, action=action, outcome_unknown=submitted) from error
                if not isinstance(value, dict):
                    raise ProtocolError('Host response must be an object', code='invalid-response', status=status, action=action, outcome_unknown=submitted)
                if status != 200:
                    code = value.get('code') if isinstance(value.get('code'), str) else 'http-error'
                    message = value.get('error') or value.get('message') or f'HTTP {status}'
                    cls = PermissionDenied if status in (401, 403) else ActionError
                    raise cls(str(message), code=code, status=status, action=action, outcome_unknown=submitted and status >= 500)
                return value, raw
        except (urllib.error.URLError, OSError, http.client.HTTPException) as error:
            raise ConnectionError('ClonoLoom connection failed or timed out; inspect state before resubmitting an action', code='connection-failed', action=action, outcome_unknown=submitted) from error

    def refresh(self):
        """Check the live API, catalog and permission contract; fail before sending any action."""
        value, _ = self._request('GET', '/api/remote/tools')
        expected = {'api': API_VERSION, 'catalogVersion': CATALOG_VERSION, 'policyVersion': POLICY_VERSION}
        if any(type(value.get(k)) is not int or value[k] != v for k, v in expected.items()):
            raise CompatibilityError('Host API/catalog/policy differs from this client; update the host or client before sending actions', code='incompatible-host')
        if not isinstance(value.get('version'), str) or not isinstance(value.get('tools'), list) or not isinstance(value.get('allowed'), dict):
            raise ProtocolError('Incomplete host action catalog', code='invalid-catalog')
        if value['allowed'].get('read') is not True or any(type(v) is not bool for v in value['allowed'].values()):
            raise ProtocolError('Invalid host permissions', code='invalid-catalog')
        self._catalog = value
        return value

    def tools(self):
        return self.refresh()['tools']

    def version(self):
        return self.refresh()['version']

    def permissions(self):
        """Inspect fixed host grants; this method cannot change them."""
        return self.refresh()['allowed']

    def call(self, action, **args: Any):
        if not isinstance(action, str) or not action or len(action) > 128:
            raise ArgumentError('Action name must contain 1–128 characters', code='invalid-arguments')
        try:
            body = dumps({'action': action, 'args': {k: v for k, v in args.items() if v is not UNSET}, 'client': self.client})
        except (ValueError, TypeError, RecursionError) as error:
            raise ArgumentError(str(error), code='invalid-arguments', action=action) from error
        self.refresh()
        answer, raw = self._request('POST', '/api/remote/action', body, action)
        if type(answer.get('ok')) is not bool or not isinstance(answer.get('message', ''), str) or not isinstance(answer.get('code', ''), str):
            raise ProtocolError('Invalid action result envelope', code='invalid-response', status=200, action=action, outcome_unknown=True)
        if not answer['ok']:
            code = answer.get('code', 'action-failed')
            cls = PermissionDenied if code in ('permission-denied', 'unauthorized') else ActionError
            raise cls(answer.get('message') or answer.get('error') or 'Action failed', code=code, status=200, action=action,
                      outcome_unknown=code not in ('rejected', 'stale-project', 'busy', 'invalid-arguments', 'permission-denied', 'unauthorized', 'unknown-action'))
        return Result(action, answer.get('message', ''), answer.get('data'), raw)


def connect(url=None, token=UNSET, **options):
    connection = ClonoLoom(url, token, **options)
    connection.refresh()
    return connection
