// Ranking tests for assets/js/search.js. Run by TestSearchRankingJS when
// node is on PATH: node --test internal/build/testdata/search_rank.test.js
const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");

const search = require(path.join(__dirname, "../../../assets/js/search.js"));
const indexPath = process.env.SEARCH_INDEX;
const records = search.prepare(JSON.parse(fs.readFileSync(indexPath, "utf8")));

test("title match ranks the page intro first", () => {
  const top = search.rank(records, "retention")[0];
  assert.strictEqual(top.u, "/docs/retention/");
});

test("every token must match", () => {
  const res = search.rank(records, "retention zzzznotaword");
  assert.strictEqual(res.length, 0);
});

test("prefix matching works on env var names", () => {
  const res = search.rank(records, "max_parallel");
  assert.ok(res.length > 0);
  assert.ok(res.some((e) => e.u.startsWith("/docs/environment-variables/")));
});

test("heading matches beat body matches", () => {
  const res = search.rank(records, "signature");
  assert.match(res[0].u, /\/docs\/webhooks\/#verify-the-signature/);
});

test("empty query returns nothing", () => {
  assert.deepStrictEqual(search.rank(records, "   "), []);
});

test("snippet centres on the match", () => {
  const s = search.snippet("a ".repeat(100) + "needle here", "needle");
  assert.ok(s.startsWith("…"));
  assert.ok(s.includes("needle"));
});
