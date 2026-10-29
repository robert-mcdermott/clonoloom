"""Local fake-server controls. Run with uv; no application or data cache is needed."""
import inspect
import json
import os
import random
import sys
import tempfile
import threading
import time
import unittest
from decimal import Decimal, InvalidOperation, localcontext
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from unittest.mock import patch

import clonoloom as cl
from clonoloom._json import dumps, loads

CATALOG = json.loads((Path(__file__).resolve().parents[2] / 'tools.json').read_text())
CATALOG.update(version=cl.__version__, allowed={'read': True, 'edits': False, 'compute': False, 'files': False, 'network': False})


class ClientTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                self.reply('GET')

            def do_POST(self):
                self.reply('POST')

            def reply(self, method):
                body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
                cls.received.append((method, self.path, body, self.headers.get('X-ClonoLoom-Token')))
                status, raw, headers = cls.response(method, self.path, body)
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                for name, value in headers.items():
                    self.send_header(name, value)
                self.end_headers()
                try:
                    self.wfile.write(raw)
                except (BrokenPipeError, ConnectionResetError):
                    pass
        cls.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.url = f'http://127.0.0.1:{cls.server.server_port}'

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown(); cls.server.server_close(); cls.thread.join()

    def setUp(self):
        type(self).received = []
        type(self).response = staticmethod(lambda method, path, body: (200, dumps(CATALOG) if method == 'GET' else b'{"ok":true,"data":null}', {}))

    def endpoint(self, raw, status=200, headers=None):
        type(self).response = staticmethod(lambda method, path, body: (200, dumps(CATALOG), {}) if method == 'GET' else (status, raw, headers or {}))

    def test_compatibility_before_every_post(self):
        client = cl.connect(self.url, 'secret')
        client.get_state()
        self.assertEqual([r[0] for r in self.received], ['GET', 'GET', 'POST'])
        self.assertNotIn('secret', repr(client))
        for key in ('api', 'catalogVersion', 'policyVersion'):
            for value in (None, True, Decimal('2.0'), 999):
                altered = {**CATALOG, key: value}
                type(self).response = staticmethod(lambda *_: (200, dumps(altered), {}))
                before = len(self.received)
                with self.assertRaises(cl.CompatibilityError):
                    client.get_state()
                self.assertEqual([r[0] for r in self.received[before:]], ['GET'])

    def test_exact_native_values_and_original_bytes(self):
        nested = '{"rows":[{"n":9007199254740993,"x":0.12345678901234567890123456789,"missing":null}],"count":9007199254740993}'
        raw = b' {"ok":true,"message":"exact", "data":{"resultJSON":' + json.dumps(nested).encode() + b'}}\n'
        self.endpoint(raw)
        result = cl.ClonoLoom(self.url).query_clonotypes()
        self.assertEqual(result.raw_json, raw)
        self.assertEqual(result.data['resultJSON'], nested)
        self.assertEqual(result.native()['count'], 9007199254740993)
        self.assertEqual(result.records()[0]['x'], Decimal('0.12345678901234567890123456789'))
        self.assertIsNone(result.records()[0]['missing'])
        self.assertEqual(result.records()[0]['n'], 9007199254740993)

    def test_decimal_encoding_no_context_rounding_or_stringification(self):
        with localcontext() as context:
            context.prec = 2
            value = Decimal('123456789.012345678901234567890')
            raw = dumps({'value': value, 'small': Decimal('1E-1000'), 'negativeZero': Decimal('-0.0')})
            self.assertIn(b'123456789.012345678901234567890', raw)
            self.assertEqual(loads(raw)['value'], value)
        with localcontext() as context:
            context.traps[InvalidOperation] = False
            with self.assertRaises(ValueError):
                loads('1e999999999999999999999999999999')
        self.assertEqual(dumps([[], {}, None, True, False, 0]), b'[[],{},null,true,false,0]')
        self.assertEqual(loads(b'[-0.0,1e-99]'), [Decimal('-0.0'), Decimal('1e-99')])

    def test_seeded_json_round_trips(self):
        random_source = random.Random(20261028)
        for _ in range(1000):
            integer = random_source.randrange(-(1 << 100), 1 << 100)
            token = f'{integer}.{random_source.randrange(10**24):024d}e{random_source.randrange(-100, 100):+d}'
            value = {'n': integer, 'd': Decimal(token), 'array': [None, [], {}, True, False, 'μ\n"\\']}
            raw = dumps(value)
            oracle = json.loads(raw, parse_float=Decimal, parse_int=int)
            self.assertEqual(oracle, value)
            self.assertEqual(loads(raw), value)

    def test_omission_null_and_arrays(self):
        client = cl.ClonoLoom(self.url, 'secret')
        client.query_clonotypes(limit=cl.UNSET, where=None)
        sent = loads(self.received[-1][2])
        self.assertEqual(sent['args'], {'where': None})
        client.call('test', a=[], b={}, c=[None], d=[{'n': 9007199254740993}], e=False)
        self.assertEqual(loads(self.received[-1][2])['args'], {'a': [], 'b': {}, 'c': [None], 'd': [{'n': 9007199254740993}], 'e': False})
        self.assertEqual(self.received[-1][3], 'secret')
        self.assertIsNone(self.received[-2][3])

    def test_all_generated_wrappers(self):
        sent = {}
        class Recorder(cl.ClonoLoom):
            def call(self, action, **args):
                sent[action] = args
        recorder = Recorder(self.url, '')
        for tool in CATALOG['tools']:
            method = getattr(recorder, tool['name'])
            signature = inspect.signature(method)
            required = tool['inputSchema'].get('required', [])
            method(*['sentinel'] * len(required))
            args = sent[tool['name']]
            self.assertEqual(set(args), set(tool['inputSchema']['properties']))
            for key in args:
                self.assertEqual(args[key], 'sentinel') if key in required else self.assertIs(args[key], cl.UNSET)
            self.assertIn('Effects:', method.__doc__)
            self.assertEqual(len(signature.parameters), len(args))
        recorder.match_specificity(database='db', cdr3_only=False)
        self.assertIs(sent['match_specificity']['cdr3Only'], False)

    def test_typed_errors_and_no_retry(self):
        for status, code, error_type in [(401, 'unauthorized', cl.PermissionDenied), (403, 'permission-denied', cl.PermissionDenied), (400, 'invalid-arguments', cl.ActionError), (404, 'unknown-action', cl.ActionError), (503, 'window-unavailable', cl.ActionError)]:
            self.endpoint(dumps({'error': 'host reason', 'code': code}), status)
            before = len(self.received)
            with self.assertRaises(error_type) as caught:
                cl.ClonoLoom(self.url).get_state()
            self.assertEqual(caught.exception.code, code)
            self.assertEqual(caught.exception.status, status)
            self.assertEqual(caught.exception.action, 'get_state')
            self.assertEqual(len(self.received) - before, 2)
        self.endpoint(b'{"ok":false,"code":"rejected","message":"person rejected"}')
        with self.assertRaises(cl.ActionError) as caught:
            cl.ClonoLoom(self.url).set_theme(theme='dark')
        self.assertFalse(caught.exception.outcome_unknown)

    def test_redirect_refused_without_forwarding_token(self):
        self.endpoint(b'{}', 307, {'Location': self.url + '/stolen'})
        with self.assertRaises(cl.ProtocolError) as caught:
            cl.ClonoLoom(self.url, 'secret').get_state()
        self.assertEqual(caught.exception.code, 'redirect-refused')
        self.assertEqual([r[1] for r in self.received], ['/api/remote/tools', '/api/remote/action'])

    def test_proxies_are_ignored(self):
        with patch.dict(os.environ, {'http_proxy': 'http://127.0.0.1:1', 'HTTP_PROXY': 'http://127.0.0.1:1', 'no_proxy': '', 'NO_PROXY': ''}):
            cl.connect(self.url, '').get_state()

    def test_malformed_responses(self):
        for raw in [b'[]', b'{', b'\xff', b'{"ok":true,"ok":false}', b'{"ok":1}', b'{"ok":false,"code":[]}', b'{"ok":true,"data":NaN}', b'{"ok":true,"data":Infinity}', b'{"ok":true,"data":1e999999999999999999999999999999}', b'{"ok":true,"data":'+b'['*70+b'0'+b']'*70+b'}']:
            self.endpoint(raw)
            with self.assertRaises(cl.ProtocolError) as caught:
                cl.ClonoLoom(self.url).get_state()
            self.assertTrue(caught.exception.outcome_unknown)

    def test_bounds(self):
        self.endpoint(b' ' * ((16 << 20) + 1))
        with self.assertRaises(cl.ProtocolError) as caught:
            cl.ClonoLoom(self.url).get_state()
        self.assertEqual(caught.exception.code, 'response-too-large')
        before = len(self.received)
        with self.assertRaises(cl.ArgumentError):
            cl.ClonoLoom(self.url).call('test', value='x' * (1 << 20))
        self.assertEqual(len(self.received), before)

    def test_bad_inputs_never_reach_host(self):
        cycle = []; cycle.append(cycle)
        for value in [float('nan'), float('inf'), Decimal('NaN'), Decimal('Infinity'), {1: 'key'}, (1,), {1, 2}, cycle, [cl.UNSET]]:
            with self.assertRaises(cl.ArgumentError):
                cl.ClonoLoom(self.url).call('test', value=value)
        self.assertEqual(self.received, [])
        for url in ['https://127.0.0.1:8860', 'http://example.org', 'http://127.0.0.1:8860/path', 'http://user:secret@127.0.0.1', 'http://127.0.0.1?token=x', 'http://127.0.0.1\n', 'http://127.0.0.1:0']:
            with self.assertRaises(cl.ConnectionError):
                cl.ClonoLoom(url)
        for timeout in (True, 0, -1, float('inf')):
            with self.assertRaises(cl.ArgumentError):
                cl.ClonoLoom(self.url, timeout=timeout)

    def test_timeout_is_unknown_and_not_retried(self):
        def delayed(method, *_):
            if method == 'GET':
                return 200, dumps(CATALOG), {}
            time.sleep(.15)
            return 200, b'{"ok":true}', {}
        type(self).response = staticmethod(delayed)
        with self.assertRaises(cl.ConnectionError) as caught:
            cl.ClonoLoom(self.url, timeout=.04).get_state()
        self.assertTrue(caught.exception.outcome_unknown)
        self.assertEqual(len(self.received), 2)
        time.sleep(.2)

    def test_frames_preserve_objects(self):
        try:
            import pandas
        except ImportError:
            if os.environ.get('CLONOLOOM_REQUIRE_PANDAS'):
                self.fail('pandas test was required')
            self.skipTest('optional pandas not installed')
        rows = [{'n': 9007199254740993, 'p': Decimal('0.100000000000000000001'), 'x': None, 'b': True, 'a': [None]}, {'n': None, 'p': None, 'b': False}]
        result = cl.Result('test', '', {'rows': rows})
        frame = result.frame()
        self.assertTrue(all(str(dtype) == 'object' for dtype in frame.dtypes))
        self.assertIsInstance(frame.at[0, 'n'], int)
        self.assertEqual(frame.at[0, 'n'], rows[0]['n'])
        self.assertEqual(frame.at[0, 'p'], rows[0]['p'])
        self.assertIsNone(frame.at[1, 'n'])
        self.assertIsNone(frame.at[1, 'x'])
        self.assertIs(frame.at[0, 'b'], True)
        self.assertEqual(frame.at[0, 'a'], [None])
        self.assertEqual(result.records(), rows)
        self.assertEqual(cl.Result('test', '', []).frame().shape, (0, 0))
        self.assertEqual(cl.Result('test', '', [{}, {}]).frame().shape, (2, 0))

    def test_records_are_explicit(self):
        self.assertEqual(cl.Result('x', '', {'rows': []}).records(), [])
        self.assertEqual(cl.Result('x', '', {'other': []}).records('other'), [])
        with self.assertRaises(TypeError):
            cl.Result('x', '', {'metadata': [1, 2]}).records()
        with self.assertRaises(TypeError):
            cl.Result('x', '', {'rows': [1]}).records()


class DiscoveryTests(unittest.TestCase):
    def test_precedence_and_no_unrelated_token(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ, {}, clear=True):
            path = Path(folder, 'remote.json')
            with self.assertRaises(cl.ConnectionError):
                cl.find_connection(folder)
            path.write_text('{"url":"http://127.0.0.1:8870","token":"library-token"}')
            os.environ.update(CLONOLOOM_URL='http://127.0.0.1:8871', CLONOLOOM_TOKEN='env-token')
            self.assertEqual(cl.find_connection()['token'], 'env-token')
            self.assertEqual(cl.find_connection(folder)['token'], 'library-token')
            client = cl.ClonoLoom('http://127.0.0.1:8872', data_dir=folder)
            self.assertIsNone(client.token)
            self.assertEqual(cl.ClonoLoom(data_dir=folder).token, 'library-token')
            self.assertIsNone(cl.ClonoLoom(data_dir=folder, token=None).token)
            self.assertEqual(cl.ClonoLoom(data_dir=folder, token='override').token, 'override')
            path.write_text('not JSON')
            with self.assertRaises(cl.ConnectionError):
                cl.find_connection(folder)

    def test_default_fallback_and_os_paths(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ, {}, clear=True):
            with patch('clonoloom.client.default_data_dir', return_value=Path(folder)):
                self.assertEqual(cl.find_connection(), {'url': 'http://127.0.0.1:8860', 'token': None})
            with patch('sys.platform', 'darwin'):
                self.assertEqual(cl.default_data_dir(), Path.home() / 'Library/Application Support/ClonoLoom')
            with patch('sys.platform', 'win32'), patch.dict(os.environ, {'APPDATA': folder}):
                self.assertEqual(cl.default_data_dir(), Path(folder) / 'ClonoLoom')
            with patch('sys.platform', 'linux'), patch.dict(os.environ, {'XDG_CONFIG_HOME': folder}):
                self.assertEqual(cl.default_data_dir(), Path(folder) / 'ClonoLoom')


if __name__ == '__main__':
    unittest.main()
