// Transpiler spec Fig -> JSON milik anjuran.
//
// Sumbernya adalah paket npm @withfig/autocomplete (MIT), yang sudah berisi
// spec terkompilasi sebagai modul ESM. Karena itu tidak dibutuhkan kompiler
// TypeScript sama sekali: modulnya diimpor, lalu objeknya dijalankan melalui
// penyaring di bawah ini.
//
// Yang DIBUANG, dan alasannya:
//   - generator berbentuk fungsi dan `custom`  : butuh mesin JS saat runtime
//   - postProcess                              : idem
//   - generateSpec, parserDirectives           : idem
//   - icon                                     : berupa URL fig:// yang tidak berlaku di sini
//
// Yang DIPERTAHANKAN adalah seluruh struktur statis ditambah generator yang
// sudah berbentuk argv, sehingga tidak ada satu pun perintah yang perlu
// dilewatkan ke shell.
//
// Pemakaian: node transpile.mjs <dir-build-fig> <dir-keluaran>

import { readdirSync, mkdirSync, writeFileSync, statSync } from 'node:fs';
import { resolve, join, dirname, relative } from 'node:path';
import { pathToFileURL } from 'node:url';

const [, , srcArg, outArg] = process.argv;
if (!srcArg || !outArg) {
  console.error('pemakaian: node transpile.mjs <dir-build-fig> <dir-keluaran>');
  process.exit(2);
}
const SRC = resolve(srcArg);
const OUT = resolve(outArg);

const stats = {
  specs: 0, gagal: 0, subcommands: 0, options: 0, args: 0, suggestions: 0,
  generatorDipakai: 0, generatorDibuang: 0, loadSpec: 0, bytes: 0,
};
const gagal = [];

/** asArray menyeragamkan field yang boleh tunggal maupun jamak. */
const asArray = (v) => (v === undefined || v === null ? [] : Array.isArray(v) ? v : [v]);

/** names menyeragamkan name yang boleh string atau array string. */
function names(v) {
  const list = asArray(v).filter((s) => typeof s === 'string' && s.length > 0);
  return list.length === 1 ? list[0] : list.length ? list : undefined;
}

// Deskripsi Fig bisa sepanjang beberapa paragraf, terutama pada aws dan
// gcloud. Dropdown hanya pernah menampilkan satu baris sekitar 60 karakter,
// jadi menyimpan sisanya adalah berkas besar yang tidak pernah terbaca.
const DESC_MAX = 110;

/** desc merapikan deskripsi menjadi satu baris pendek. */
function desc(v) {
  if (typeof v !== 'string') return undefined;
  let s = v.replace(/\s+/g, ' ').trim();
  if (!s) return undefined;
  // Potong di akhir kalimat pertama bila kalimat itu memang pendek.
  const dot = s.search(/\.\s/);
  if (dot > 0 && dot <= DESC_MAX) return s.slice(0, dot);
  if (s.length <= DESC_MAX) return s;
  // Jika tidak, potong di batas kata terakhir sebelum ambang.
  const cut = s.lastIndexOf(' ', DESC_MAX);
  return s.slice(0, cut > DESC_MAX / 2 ? cut : DESC_MAX) + '…';
}

/** keep membuang field yang bernilai kosong agar berkas tidak membengkak. */
function keep(obj) {
  const out = {};
  for (const [k, v] of Object.entries(obj)) {
    if (v === undefined || v === null) continue;
    if (v === false || v === '' ) continue;
    if (Array.isArray(v) && v.length === 0) continue;
    out[k] = v;
  }
  return out;
}

function convGenerator(g) {
  // Hanya bentuk deklaratif yang bisa dibawa. Sisanya butuh mesin JS.
  if (typeof g === 'function' || typeof g?.custom === 'function') return null;
  if (typeof g?.script === 'function') return null;

  const out = {};
  if (Array.isArray(g?.script) && g.script.every((s) => typeof s === 'string')) {
    out.script = g.script;
  }
  if (g?.template) out.template = asArray(g.template);
  if (typeof g?.splitOn === 'string') out.splitOn = g.splitOn;
  if (typeof g?.cache?.ttl === 'number') out.cacheTtl = Math.round(g.cache.ttl / 1000);

  if (!out.script && !out.template) return null;

  // postProcess ikut hilang, jadi keluaran mentah dirapikan seadanya.
  if (out.script) out.trim = true;
  return out;
}

function convSuggestion(s) {
  if (typeof s === 'string') return { name: s };
  const n = names(s?.name);
  if (!n) return null;
  stats.suggestions++;
  return keep({
    name: n,
    displayName: s.displayName,
    description: desc(s.description),
    insertValue: s.insertValue,
    priority: typeof s.priority === 'number' ? s.priority : undefined,
    hidden: s.hidden === true,
    deprecated: s.deprecated ? true : undefined,
  });
}

function convArg(a) {
  if (!a || typeof a !== 'object') return null;
  stats.args++;

  const generators = [];
  for (const g of asArray(a.generators)) {
    const c = convGenerator(g);
    if (c) { generators.push(c); stats.generatorDipakai++; }
    else stats.generatorDibuang++;
  }

  return keep({
    name: typeof a.name === 'string' ? a.name : undefined,
    description: desc(a.description),
    isOptional: a.isOptional === true,
    isVariadic: a.isVariadic === true,
    isCommand: a.isCommand === true,
    default: typeof a.default === 'string' ? a.default : undefined,
    template: asArray(a.template),
    suggestions: asArray(a.suggestions).map(convSuggestion).filter(Boolean),
    generators,
  });
}

function convOption(o) {
  const n = names(o?.name);
  if (!n) return null;
  stats.options++;
  return keep({
    name: n,
    displayName: o.displayName,
    description: desc(o.description),
    insertValue: o.insertValue,
    priority: typeof o.priority === 'number' ? o.priority : undefined,
    isPersistent: o.isPersistent === true,
    isRepeatable: o.isRepeatable === true || typeof o.isRepeatable === 'number',
    requiresSeparator: o.requiresSeparator === true || o.requiresEquals === true
      ? true
      : typeof o.requiresSeparator === 'string' ? o.requiresSeparator : undefined,
    exclusiveOn: asArray(o.exclusiveOn).filter((s) => typeof s === 'string'),
    dependsOn: asArray(o.dependsOn).filter((s) => typeof s === 'string'),
    isDangerous: o.isDangerous === true,
    hidden: o.hidden === true,
    deprecated: o.deprecated ? true : undefined,
    args: asArray(o.args).map(convArg).filter(Boolean),
  });
}

function convSubcommand(sc, depth = 0) {
  const n = names(sc?.name);
  if (!n || depth > 16) return null;
  stats.subcommands++;
  if (typeof sc.loadSpec === 'string') stats.loadSpec++;

  return keep({
    name: n,
    displayName: sc.displayName,
    description: desc(sc.description),
    insertValue: sc.insertValue,
    priority: typeof sc.priority === 'number' ? sc.priority : undefined,
    hidden: sc.hidden === true,
    deprecated: sc.deprecated ? true : undefined,
    isDangerous: sc.isDangerous === true,
    requiresSubcommand: sc.requiresSubcommand === true,
    // loadSpec berbentuk fungsi tidak bisa dibawa; hanya rujukan berkas.
    loadSpec: typeof sc.loadSpec === 'string' ? sc.loadSpec : undefined,
    subcommands: asArray(sc.subcommands).map((s) => convSubcommand(s, depth + 1)).filter(Boolean),
    options: asArray(sc.options).map(convOption).filter(Boolean),
    args: asArray(sc.args).map(convArg).filter(Boolean),
  });
}

/** listFiles menelusuri direktori build secara rekursif. */
function listFiles(dir, base = dir) {
  const out = [];
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) out.push(...listFiles(p, base));
    else if (e.name.endsWith('.js')) out.push(relative(base, p));
  }
  return out;
}

const files = listFiles(SRC);
for (const rel of files) {
  try {
    let mod = (await import(pathToFileURL(resolve(SRC, rel)).href)).default;
    if (Array.isArray(mod)) mod = mod[0];
    if (!mod || typeof mod !== 'object' || !mod.name) continue;

    const conv = convSubcommand(mod);
    if (!conv) continue;

    // Nama berkas keluaran mengikuti struktur sumber, sehingga rujukan
    // loadSpec seperti "aws/accessanalyzer" langsung berlaku sebagai path.
    const dest = resolve(OUT, rel.replace(/\.js$/, '.json'));
    mkdirSync(dirname(dest), { recursive: true });
    const json = JSON.stringify(conv);
    writeFileSync(dest, json);
    stats.bytes += json.length;
    stats.specs++;
  } catch (e) {
    stats.gagal++;
    if (gagal.length < 10) gagal.push(`${rel}: ${String(e.message).slice(0, 70)}`);
  }
}

const mb = (n) => (n / 1048576).toFixed(1) + ' MB';
console.log(`spec berhasil      : ${stats.specs}`);
console.log(`spec gagal         : ${stats.gagal}`);
console.log(`subcommand         : ${stats.subcommands}`);
console.log(`option             : ${stats.options}`);
console.log(`argumen            : ${stats.args}`);
console.log(`suggestion statis  : ${stats.suggestions}`);
console.log(`loadSpec           : ${stats.loadSpec}`);
console.log(`generator dipakai  : ${stats.generatorDipakai}`);
console.log(`generator dibuang  : ${stats.generatorDibuang}`);
console.log(`ukuran keluaran    : ${mb(stats.bytes)}`);
if (gagal.length) console.log('\ncontoh kegagalan:\n  ' + gagal.join('\n  '));
