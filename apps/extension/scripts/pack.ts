import { readdirSync, statSync, readFileSync, writeFileSync, existsSync } from 'node:fs';
import { resolve, relative, join } from 'node:path';
import { deflateRawSync } from 'node:zlib';

function makeCRCTable(): Uint32Array {
  const table = new Uint32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let k = 0; k < 8; k++) {
      c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    }
    table[i] = c >>> 0;
  }
  return table;
}

const crcTable = makeCRCTable();

function crc32(buf: Buffer): number {
  let crc = 0xffffffff;
  for (let i = 0; i < buf.length; i++) {
    crc = crcTable[(crc ^ buf[i]) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function collectFiles(dir: string, baseDir: string): { relativePath: string; fullPath: string }[] {
  const results: { relativePath: string; fullPath: string }[] = [];
  const entries = readdirSync(dir);

  for (const entry of entries) {
    const full = join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) {
      results.push(...collectFiles(full, baseDir));
    } else if (stat.isFile()) {
      const rel = relative(baseDir, full).replace(/\\/g, '/');
      if (!rel.endsWith('.zip')) {
        results.push({ relativePath: rel, fullPath: full });
      }
    }
  }

  return results;
}

export function createZipArchive(sourceDir: string, outputFile: string): void {
  const files = collectFiles(sourceDir, sourceDir);
  const localHeaders: Buffer[] = [];
  const cdHeaders: Buffer[] = [];
  let currentOffset = 0;

  for (const file of files) {
    const rawData = readFileSync(file.fullPath);
    const compressed = deflateRawSync(rawData);
    const nameBuf = Buffer.from(file.relativePath, 'utf8');
    const fileCrc = crc32(rawData);

    // Local file header (30 bytes + nameBuf)
    const localHeader = Buffer.alloc(30 + nameBuf.length);
    localHeader.writeUInt32LE(0x04034b50, 0); // signature
    localHeader.writeUInt16LE(20, 4);          // version needed
    localHeader.writeUInt16LE(0, 6);           // flags
    localHeader.writeUInt16LE(8, 8);           // compression (deflate)
    localHeader.writeUInt16LE(0, 10);          // mod time
    localHeader.writeUInt16LE(0, 12);          // mod date
    localHeader.writeUInt32LE(fileCrc, 14);    // crc32
    localHeader.writeUInt32LE(compressed.length, 18); // comp size
    localHeader.writeUInt32LE(rawData.length, 22);    // uncomp size
    localHeader.writeUInt16LE(nameBuf.length, 26);    // name length
    localHeader.writeUInt16LE(0, 28);                 // extra length
    nameBuf.copy(localHeader, 30);

    // Central directory header (46 bytes + nameBuf)
    const cdHeader = Buffer.alloc(46 + nameBuf.length);
    cdHeader.writeUInt32LE(0x02014b50, 0); // signature
    cdHeader.writeUInt16LE(20, 4);          // version made by
    cdHeader.writeUInt16LE(20, 6);          // version needed
    cdHeader.writeUInt16LE(0, 8);           // flags
    cdHeader.writeUInt16LE(8, 10);          // compression (deflate)
    cdHeader.writeUInt16LE(0, 12);          // mod time
    cdHeader.writeUInt16LE(0, 14);          // mod date
    cdHeader.writeUInt32LE(fileCrc, 16);    // crc32
    cdHeader.writeUInt32LE(compressed.length, 20); // comp size
    cdHeader.writeUInt32LE(rawData.length, 24);    // uncomp size
    cdHeader.writeUInt16LE(nameBuf.length, 28);    // name length
    cdHeader.writeUInt16LE(0, 30);                 // extra length
    cdHeader.writeUInt16LE(0, 32);                 // comment length
    cdHeader.writeUInt16LE(0, 34);                 // disk start
    cdHeader.writeUInt16LE(0, 36);                 // internal attr
    cdHeader.writeUInt32LE(0, 38);                 // external attr
    cdHeader.writeUInt32LE(currentOffset, 42);     // local header offset
    nameBuf.copy(cdHeader, 46);

    localHeaders.push(localHeader, compressed);
    cdHeaders.push(cdHeader);

    currentOffset += localHeader.length + compressed.length;
  }

  const cdOffset = currentOffset;
  let cdSize = 0;
  for (const h of cdHeaders) {
    cdSize += h.length;
  }

  // End of central directory record (22 bytes)
  const eocd = Buffer.alloc(22);
  eocd.writeUInt32LE(0x06054b50, 0);  // signature
  eocd.writeUInt16LE(0, 4);           // disk number
  eocd.writeUInt16LE(0, 6);           // disk where cd starts
  eocd.writeUInt16LE(files.length, 8); // entries on this disk
  eocd.writeUInt16LE(files.length, 10); // total entries
  eocd.writeUInt32LE(cdSize, 12);     // cd size
  eocd.writeUInt32LE(cdOffset, 16);   // cd offset
  eocd.writeUInt16LE(0, 20);          // comment length

  const finalZip = Buffer.concat([...localHeaders, ...cdHeaders, eocd]);
  writeFileSync(outputFile, finalZip);
  console.log(`Successfully created ${outputFile} (${finalZip.length} bytes, ${files.length} files)`);
}

const distDir = resolve(import.meta.dir, '../dist');
const outFile = resolve(distDir, 'kpi-schedule-sync.zip');

if (!existsSync(distDir)) {
  console.error(`Error: dist directory not found at ${distDir}. Run build first.`);
  process.exit(1);
}

createZipArchive(distDir, outFile);
