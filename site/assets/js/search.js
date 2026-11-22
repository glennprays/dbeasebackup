// Docs search. Loads /docs/search-index.json on first open and ranks
// entries by token-prefix matches: title > heading > body text.
// Progressive enhancement: without JS the trigger stays hidden.
(function (root) {
  "use strict";

  var WEIGHT = { title: 3, heading: 2, text: 1 };
  var MAX_RESULTS = 8;

  function tokenize(s) {
    return (s || "").toLowerCase().split(/[^a-z0-9_]+/).filter(Boolean);
  }

  // prepare turns raw index entries into searchable records.
  function prepare(entries) {
    return entries.map(function (e, i) {
      return {
        e: e,
        order: i,
        title: tokenize(e.t),
        heading: tokenize(e.h),
        text: tokenize(e.x),
      };
    });
  }

  function hasPrefix(words, tok) {
    for (var i = 0; i < words.length; i++) {
      if (words[i].indexOf(tok) === 0) return true;
    }
    return false;
  }

  // rank returns entries matching every query token, best first.
  function rank(records, query) {
    var toks = tokenize(query);
    if (!toks.length) return [];
    var hits = [];
    records.forEach(function (r) {
      var score = 0;
      for (var i = 0; i < toks.length; i++) {
        var t = toks[i], s = 0;
        if (hasPrefix(r.title, t)) s += WEIGHT.title;
        if (hasPrefix(r.heading, t)) s += WEIGHT.heading;
        if (hasPrefix(r.text, t)) s += WEIGHT.text;
        if (s === 0) return; // every token must match somewhere
        score += s;
      }
      // A page's intro entry is the best landing spot for a title match.
      if (!r.e.h) score += 0.5;
      hits.push({ r: r, score: score });
    });
    hits.sort(function (a, b) {
      return b.score - a.score || a.r.order - b.r.order;
    });
    return hits.slice(0, MAX_RESULTS).map(function (h) {
      return h.r.e;
    });
  }

  // snippet returns ~140 chars of text around the first token hit.
  function snippet(text, query) {
    var toks = tokenize(query);
    var lower = (text || "").toLowerCase();
    var at = -1;
    for (var i = 0; i < toks.length && at < 0; i++) at = lower.indexOf(toks[i]);
    if (at < 0) at = 0;
    var start = Math.max(0, at - 40);
    var out = text.slice(start, start + 140);
    return (start > 0 ? "…" : "") + out + (start + 140 < text.length ? "…" : "");
  }

  var api = { tokenize: tokenize, prepare: prepare, rank: rank, snippet: snippet };
  if (typeof module !== "undefined" && module.exports) {
    module.exports = api;
    return;
  }

  // ---------- DOM ----------
  var dialog = document.querySelector("[data-search-dialog]");
  var triggers = document.querySelectorAll("[data-search-open]");
  if (!dialog || !triggers.length || typeof dialog.showModal !== "function") return;

  var input = dialog.querySelector("[data-search-input]");
  var list = dialog.querySelector("[data-search-results]");
  var status = dialog.querySelector("[data-search-status]");
  var records = null;
  var loading = null;
  var active = -1;

  function load() {
    if (records) return Promise.resolve(records);
    if (!loading) {
      loading = fetch(dialog.getAttribute("data-index"))
        .then(function (res) {
          if (!res.ok) throw new Error(res.status);
          return res.json();
        })
        .then(function (entries) {
          records = prepare(entries);
          return records;
        })
        .catch(function () {
          loading = null;
          status.textContent = "Search is unavailable right now.";
          return null;
        });
    }
    return loading;
  }

  function mark(el, text, query) {
    var toks = tokenize(query);
    if (!toks.length) {
      el.textContent = text;
      return;
    }
    var re = new RegExp("(" + toks.map(function (t) {
      return t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    }).join("|") + ")", "gi");
    var last = 0;
    text.replace(re, function (m, _g, idx) {
      if (idx > last) el.appendChild(document.createTextNode(text.slice(last, idx)));
      var mk = document.createElement("mark");
      mk.textContent = m;
      el.appendChild(mk);
      last = idx + m.length;
      return m;
    });
    if (last < text.length) el.appendChild(document.createTextNode(text.slice(last)));
  }

  function setActive(i) {
    var items = list.querySelectorAll("[role=option]");
    if (!items.length) {
      active = -1;
      input.removeAttribute("aria-activedescendant");
      return;
    }
    active = (i + items.length) % items.length;
    items.forEach(function (it, n) {
      it.setAttribute("aria-selected", n === active ? "true" : "false");
    });
    input.setAttribute("aria-activedescendant", items[active].id);
    items[active].scrollIntoView({ block: "nearest" });
  }

  function render(query) {
    list.textContent = "";
    if (!records) return;
    var results = rank(records, query);
    input.setAttribute("aria-expanded", results.length ? "true" : "false");
    if (!tokenize(query).length) {
      status.textContent = "";
    } else if (!results.length) {
      status.textContent = "No results for “" + query.trim() + "”.";
    } else {
      status.textContent = results.length + (results.length === 1 ? " result" : " results");
    }
    results.forEach(function (e, n) {
      var li = document.createElement("li");
      li.id = "search-opt-" + n;
      li.setAttribute("role", "option");
      li.setAttribute("aria-selected", "false");
      var a = document.createElement("a");
      a.href = e.u;
      a.tabIndex = -1;
      var where = document.createElement("span");
      where.className = "search__where";
      where.textContent = e.s + " · " + e.t;
      var title = document.createElement("span");
      title.className = "search__title";
      mark(title, e.h || e.t, query);
      var body = document.createElement("span");
      body.className = "search__text";
      mark(body, snippet(e.x, query), query);
      a.appendChild(where);
      a.appendChild(title);
      a.appendChild(body);
      li.appendChild(a);
      li.addEventListener("mousemove", function () {
        if (active !== n) setActive(n);
      });
      list.appendChild(li);
    });
    setActive(0);
  }

  function open() {
    if (dialog.open) return;
    dialog.showModal();
    input.select();
    load().then(function () {
      render(input.value);
    });
  }

  triggers.forEach(function (t) {
    t.hidden = false;
    t.addEventListener("click", open);
  });

  document.addEventListener("keydown", function (ev) {
    if (ev.key !== "/" || ev.ctrlKey || ev.metaKey || ev.altKey || dialog.open) return;
    var el = document.activeElement;
    var tag = el && el.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || (el && el.isContentEditable)) return;
    ev.preventDefault();
    open();
  });

  input.addEventListener("input", function () {
    render(input.value);
  });

  input.addEventListener("keydown", function (ev) {
    if (ev.key === "ArrowDown") {
      ev.preventDefault();
      setActive(active + 1);
    } else if (ev.key === "ArrowUp") {
      ev.preventDefault();
      setActive(active - 1);
    } else if (ev.key === "Enter") {
      var opt = list.querySelectorAll("[role=option] a")[active];
      if (opt) {
        ev.preventDefault();
        dialog.close();
        window.location.href = opt.href;
      }
    }
  });

  list.addEventListener("click", function (ev) {
    if (ev.target.closest("a")) dialog.close();
  });

  // Close when clicking the backdrop.
  dialog.addEventListener("click", function (ev) {
    if (ev.target === dialog) dialog.close();
  });
})(this);
