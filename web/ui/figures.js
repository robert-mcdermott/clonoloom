// Figures for a paper (clonoloom-spec/requirements.md, O1): every plot card's button saves its plot
// as a figure standing alone (web/lib/plot.js, figure): the plot on white with its title, legend
// and caption (what was measured, the clonotype definition, the abundance unit, how many samples,
// by which ClonoLoom), as SVG (vector), PNG (300 dpi) or PDF (the PNG on a page of the figure's
// size, api_figures.go).

import { h, icon, downloadBlob } from './dom.js';
import { showMenu, toast } from './overlays.js';
import { figure } from '../lib/plot.js';
import { withDPI } from '../lib/png.js';

const DPI = 300;

// A file name from a figure's name.
const fileName = (name) => (String(name).replace(/[^\w.-]+/g, '_').replace(/^_+|_+$/g, '') || 'figure');

// figureButton answers a button that saves a figure; spec() answers it when asked: { name, title,
// plot (a plot's SVG), legend [{ label, className }], caption [lines] }.
export function figureButton(spec) {
  const button = h('button.icon-button.figure-export', {
    type: 'button', title: 'Save as a figure', 'aria-label': 'Save as a figure',
    onclick: () => showMenu(button, [
      { section: 'Save as a figure' },
      { label: 'SVG (vector)', icon: 'download', onSelect: () => save('svg') },
      { label: `PNG (${DPI} dpi)`, icon: 'download', onSelect: () => save('png') },
      { label: `PDF (${DPI}-dpi image)`, icon: 'download', onSelect: () => save('pdf') },
    ]),
  }, icon('download'));
  async function save(format) {
    try {
      const s = spec();
      downloadBlob(await figureBlob(s, format), `${fileName(s.name ?? s.title)}.${format}`);
    } catch (error) {
      toast(error.message, { kind: 'error' });
    }
  }
  return button;
}

// figureBlob makes a figure's file: svg, png or pdf.
export async function figureBlob(s, format) {
  const fig = figure(s.plot, { title: s.title, legend: s.legend ?? [], caption: s.caption ?? [], colorVision: document.documentElement.hasAttribute('data-cvd') });
  if (format === 'svg') return new Blob([fig.svg], { type: 'image/svg+xml' });
  const png = await rasterize(fig);
  if (format === 'png') return new Blob([png], { type: 'image/png' });
  const response = await fetch(`api/figure/pdf?width=${fig.width}&height=${fig.height}&title=${encodeURIComponent(s.title ?? '')}`, { method: 'POST', headers: { 'Content-Type': 'image/png' }, body: png });
  if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error ?? `The PDF could not be made (HTTP ${response.status}).`);
  return response.blob();
}

// exportable adds a figure button to a plot card's caption (its pickers' row when it has one), and
// answers the card.
export function exportable(card, spec) {
  const caption = card.querySelector('figcaption');
  if (caption) (caption.querySelector('.plot-pickers') ?? caption).append(figureButton(spec));
  return card;
}

// The caption's last line: the program and the day.
export const madeBy = (version) => `Made with ClonoLoom ${version}, ${new Date().toISOString().slice(0, 10)}.`;

// rasterize draws a figure at DPI dots per inch (its pixels are 1/96 inch), with that size written
// into the PNG.
async function rasterize(fig) {
  const scale = DPI / 96;
  const img = new Image();
  img.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(fig.svg)}`;
  await img.decode();
  const canvas = document.createElement('canvas');
  canvas.width = Math.round(fig.width * scale);
  canvas.height = Math.round(fig.height * scale);
  canvas.getContext('2d').drawImage(img, 0, 0, canvas.width, canvas.height);
  const blob = await new Promise((done, fail) => canvas.toBlob((b) => (b ? done(b) : fail(new Error('The figure could not be drawn.'))), 'image/png'));
  return withDPI(new Uint8Array(await blob.arrayBuffer()), DPI);
}
