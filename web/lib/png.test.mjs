import assert from 'node:assert/strict';
import test from 'node:test';
import { crc32, withDPI } from './png.js';

// The smallest PNG: one transparent pixel.
const PIXEL = Uint8Array.from(Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==', 'base64'));

test('crc32 is the PNG checksum', () => {
  assert.equal(crc32(new TextEncoder().encode('IEND')), 0xae426082);
  assert.equal(crc32(new TextEncoder().encode('123456789')), 0xcbf43926);
});

test('a PNG gets its physical size after its header, once', () => {
  const once = withDPI(PIXEL, 300);
  const twice = withDPI(once, 300);
  assert.equal(once.length, PIXEL.length + 21);
  assert.deepEqual(twice, once);
  const at = 8 + 8 + 13 + 4;
  assert.equal(Buffer.from(once.slice(at + 4, at + 8)).toString(), 'pHYs');
  const view = new DataView(once.buffer);
  // 300 dpi is 11,811 pixels per meter.
  assert.equal(view.getUint32(at + 8), 11811);
  assert.equal(view.getUint32(at + 17), crc32(once.slice(at + 4, at + 17)));
  assert.throws(() => withDPI(new Uint8Array(10), 300), /not a PNG/);
});
