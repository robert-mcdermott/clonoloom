// A PNG's physical size: its pHYs chunk (pixels per meter), so that a figure rendered at 300 dots
// per inch prints at its size. Pure: bytes in, bytes out.

let table = null;

// crc32 is the PNG (and zlib) checksum of bytes.
export function crc32(bytes) {
  if (!table) {
    table = new Uint32Array(256);
    for (let n = 0; n < 256; n += 1) {
      let c = n;
      for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      table[n] = c >>> 0;
    }
  }
  let crc = 0xffffffff;
  for (const b of bytes) crc = table[(crc ^ b) & 0xff] ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}

// withDPI answers the PNG with a pHYs chunk of dpi dots per inch after its header (any pHYs it had
// left out).
export function withDPI(png, dpi) {
  const signature = 8;
  const view = new DataView(png.buffer, png.byteOffset, png.byteLength);
  if (png.length < 33 || String.fromCharCode(...png.slice(12, 16)) !== 'IHDR') throw new Error('not a PNG');
  const afterHeader = signature + 8 + view.getUint32(signature) + 4;
  const ppm = Math.round(dpi / 0.0254);
  const chunk = new Uint8Array(21);
  const c = new DataView(chunk.buffer);
  c.setUint32(0, 9);
  chunk.set([0x70, 0x48, 0x59, 0x73], 4); // pHYs
  c.setUint32(8, ppm);
  c.setUint32(12, ppm);
  chunk[16] = 1; // the unit: meters
  c.setUint32(17, crc32(chunk.subarray(4, 17)));
  // The other chunks, without an earlier pHYs.
  const parts = [png.subarray(0, afterHeader), chunk];
  let at = afterHeader;
  while (at + 8 <= png.length) {
    const length = view.getUint32(at);
    const type = String.fromCharCode(...png.slice(at + 4, at + 8));
    const end = at + 12 + length;
    if (type !== 'pHYs') parts.push(png.subarray(at, end));
    at = end;
  }
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let k = 0;
  for (const p of parts) {
    out.set(p, k);
    k += p.length;
  }
  return out;
}
