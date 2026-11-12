#!/usr/bin/env node
// Merge the i18n JSON fragments returned by the page agents into the three locales.
// Snippet Directory /tmp/i18n-frag/*.json,♪ Every shape like ♪ { "<ns>": { "zh-CN": {...}, "zh-TW": {...}, "en-US": {...} }, ... }
// Each namespace is owned by the sole agent → Top value is sufficient, no conflict. Reservations locale It's already there. nav/common/status/login Wait.
import { readdirSync, readFileSync, writeFileSync } from "node:fs";

const LOCALE_DIR = new URL("../web/src/i18n/locales/", import.meta.url);
const FRAG_DIR = "/tmp/i18n-frag";
const LANGS = ["zh-CN", "zh-TW", "en-US"];

// Safety net: Decoded sub-agents may be over-reconstructed HTML Entities&lt;→< &gt;→> &amp;→& Wait.
// UI: (a) rendering text always requires a word character; in settings, a <b> label also has to be written inside a <Trans> element.
function decodeEntities(v) {
  if (typeof v === "string") {
    return v
      .replaceAll("&lt;", "<")
      .replaceAll("&gt;", ">")
      .replaceAll("&quot;", '"')
      .replaceAll("&#39;", "'")
      .replaceAll("&amp;", "&");
  }
  if (Array.isArray(v)) return v.map(decodeEntities);
  if (v && typeof v === "object") {
    const out = {};
    for (const k of Object.keys(v)) out[k] = decodeEntities(v[k]);
    return out;
  }
  return v;
}

const locales = {};
for (const l of LANGS) {
  locales[l] = JSON.parse(readFileSync(new URL(`${l}.json`, LOCALE_DIR), "utf8"));
}

let nsCount = 0;
const seen = new Set();
for (const f of readdirSync(FRAG_DIR).filter((f) => f.endsWith(".json")).sort()) {
  const frag = JSON.parse(readFileSync(`${FRAG_DIR}/${f}`, "utf8"));
  for (const ns of Object.keys(frag)) {
    if (seen.has(ns)) {
      console.warn(`⚠️  Namespace Repeat: ${ns}(From ${f})— The latter covers the former`);
    }
    seen.add(ns);
    for (const l of LANGS) {
      const sub = frag[ns]?.[l];
      if (!sub) {
        console.warn(`⚠️  ${f}: ${ns} is missing ${l}`);
        continue;
      }
      locales[l][ns] = decodeEntities(sub);
    }
    nsCount++;
  }
}

for (const l of LANGS) {
  writeFileSync(new URL(`${l}.json`, LOCALE_DIR), JSON.stringify(locales[l], null, 2) + "\n");
}
console.log(`✓ Merge ${nsCount} Namespace In ${LANGS.length} Grandpa. locale:${[...seen].join(", ")}`);
