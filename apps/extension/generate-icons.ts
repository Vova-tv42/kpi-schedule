import { deflateSync } from 'node:zlib';
import { writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';

function createPNG(width: number, height: number, draw: (x: number, y: number) => [number, number, number, number]): Buffer {
  // Raw RGBA buffer with filter byte 0 at start of each scanline
  const rowSize = 1 + width * 4;
  const rawData = Buffer.alloc(rowSize * height);

  for (let y = 0; y < height; y++) {
    const rowOffset = y * rowSize;
    rawData[rowOffset] = 0; // Filter: None
    for (let x = 0; x < width; x++) {
      const [r, g, b, a] = draw(x, y);
      const pxOffset = rowOffset + 1 + x * 4;
      rawData[pxOffset] = r;
      rawData[pxOffset + 1] = g;
      rawData[pxOffset + 2] = b;
      rawData[pxOffset + 3] = a;
    }
  }

  const compressedData = deflateSync(rawData);

  // PNG Signature
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

  // IHDR Chunk
  const ihdrData = Buffer.alloc(13);
  ihdrData.writeUInt32BE(width, 0);
  ihdrData.writeUInt32BE(height, 4);
  ihdrData.writeUInt8(8, 8); // Bit depth
  ihdrData.writeUInt8(6, 9); // Color type (RGBA)
  ihdrData.writeUInt8(0, 10); // Compression
  ihdrData.writeUInt8(0, 11); // Filter
  ihdrData.writeUInt8(0, 12); // Interlace
  const ihdr = makeChunk('IHDR', ihdrData);

  // IDAT Chunk
  const idat = makeChunk('IDAT', compressedData);

  // IEND Chunk
  const iend = makeChunk('IEND', Buffer.alloc(0));

  return Buffer.concat([signature, ihdr, idat, iend]);
}

function makeChunk(type: string, data: Buffer): Buffer {
  const typeBuf = Buffer.from(type, 'ascii');
  const len = data.length;
  const lenBuf = Buffer.alloc(4);
  lenBuf.writeUInt32BE(len, 0);

  const crcBuf = Buffer.alloc(4);
  const crc = crc32(Buffer.concat([typeBuf, data]));
  crcBuf.writeUInt32BE(crc >>> 0, 0);

  return Buffer.concat([lenBuf, typeBuf, data, crcBuf]);
}

function crc32(buf: Buffer): number {
  let crc = -1;
  for (let i = 0; i < buf.length; i++) {
    crc ^= buf[i];
    for (let j = 0; j < 8; j++) {
      crc = (crc >>> 1) ^ (-(crc & 1) & 0xedb88320);
    }
  }
  return ~crc;
}

// Generate stylized KPI Calendar icons (deep blue background with rounded corners and white/cyan calendar symbol)
function drawIcon(size: number) {
  return (x: number, y: number): [number, number, number, number] => {
    const center = size / 2;
    const radius = size * 0.44;
    const dx = Math.abs(x - center);
    const dy = Math.abs(y - center);
    const cornerDist = Math.hypot(Math.max(0, dx - (radius - size * 0.15)), Math.max(0, dy - (radius - size * 0.15)));

    if (cornerDist > size * 0.15 + 0.5) {
      return [0, 0, 0, 0]; // Transparent outside rounded rect
    }

    // Gradient background: Dark Indigo to Cyan-Blue (KPI Colors: #0c3e74 to #0088cc)
    const t = y / size;
    let r = Math.round(12 * (1 - t) + 0 * t);
    let g = Math.round(62 * (1 - t) + 136 * t);
    let b = Math.round(116 * (1 - t) + 204 * t);
    let a = 255;

    // Calendar header bar
    const calLeft = Math.round(size * 0.2);
    const calRight = Math.round(size * 0.8);
    const calTop = Math.round(size * 0.22);
    const calBottom = Math.round(size * 0.78);
    const headerHeight = Math.round(size * 0.15);

    if (x >= calLeft && x <= calRight && y >= calTop && y <= calBottom) {
      if (y <= calTop + headerHeight) {
        // Red / Accent top bar of calendar
        return [235, 75, 75, 255];
      }
      // White calendar body
      if (x === calLeft || x === calRight || y === calBottom) {
        return [220, 225, 235, 255];
      }
      // Inner grid dots / lines
      const midY = Math.round(size * 0.52);
      const midX = Math.round(size * 0.5);
      if (y >= midY - Math.max(1, Math.round(size * 0.03)) && y <= midY + Math.max(1, Math.round(size * 0.03)) && x >= calLeft + 2 && x <= calRight - 2) {
        return [100, 140, 180, 255];
      }
      if (x >= midX - Math.max(1, Math.round(size * 0.03)) && x <= midX + Math.max(1, Math.round(size * 0.03)) && y >= calTop + headerHeight + 2 && y <= calBottom - 2) {
        return [100, 140, 180, 255];
      }
      return [250, 252, 255, 255];
    }

    return [r, g, b, a];
  };
}

const iconsDir = resolve(import.meta.dirname, 'public/icons');
mkdirSync(iconsDir, { recursive: true });

for (const size of [16, 48, 128]) {
  const png = createPNG(size, size, drawIcon(size));
  writeFileSync(resolve(iconsDir, `icon-${size}.png`), png);
  console.log(`Generated icon-${size}.png (${size}x${size})`);
}
