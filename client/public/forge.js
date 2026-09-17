// The editor's client half, and deliberately all of it.
//
// Every control here posts to an endpoint and then asks the server to re-render
// the page. There is no client-side model of a project, no optimistic update
// and no second renderer: the server owns the state and draws the tiles, so
// what is on screen after a change is what is stored, not what this file
// guessed.
//
// Everything binds by delegation on `document`. A local navigation replaces
// `#outlet` wholesale, so a listener bound to an element inside it dies on the
// next render — which is the bug that makes half a page stop responding after
// a save.

const json = { "Content-Type": "application/json" };

/** Re-render the current page in place, keeping the scroll position.
 *
 * `fresh` skips howl's prefetch cache, and it is load-bearing. That cache
 * serves an entry younger than 15 s without asking the server, which is right
 * for a link and exactly wrong here: this runs immediately after a write, so a
 * cached fragment is by definition the state before it. Without it, applying a
 * rhythm left the strip showing the old layout until the entry aged out —
 * which read as "it only updates if I go to another step and come back".
 *
 * `scroll` is the current position, not `false`: howl reads it as a number and
 * anything else means the top of the page, so a change made halfway down the
 * screenshots list jumped back to the header. */
async function refresh() {
  if (window.howl) {
    await window.howl.navigate(location.href, {
      replace: true,
      fresh: true,
      scroll: window.scrollY,
    });
  } else {
    location.reload();
  }
}

// Links are document loads, not fragment swaps.
//
// The swap replaces #outlet, which is where the top bar and the sidebar live:
// they are torn down and rebuilt on every step change, and howl may serve the
// fragment from a cache filled before the click. Neither is buying anything on
// a server that is in this process. A document load hands the whole page over
// at once, and the browser holds the previous one on screen until it is ready
// — the one thing a swap cannot do.
//
// The capture phase is what makes this possible: howl's link handler is on
// `document` in the bubble phase and skips any event whose default has already
// been prevented. `data-no-spa` would say the same thing declaratively, but the
// framework reads it off the anchor rather than an ancestor, so it cannot be
// set once for the application.
//
// The eligibility test mirrors the framework's `spaTarget`. It has to: a link
// this takes over but howl would have declined — another origin, a download, a
// target — would simply stop working. There is no isRawRoute check because this
// application has no *.raw.templ routes; add one here if it ever grows one.
document.addEventListener(
  "click",
  (event) => {
    if (event.defaultPrevented || event.button !== 0) return;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const a = event.target.closest?.("a[href]");
    if (!a || a.target || a.hasAttribute("download") || a.hasAttribute("data-no-spa")) return;

    const url = new URL(a.href, location.origin);
    if (url.origin !== location.origin || url.pathname.startsWith("/static/")) return;
    event.preventDefault();
    if (url.href === location.href) return;
    location.href = url.pathname + url.search;
  },
  true,
);

/** Say that a slow action is running, in the button itself.
 *
 * `data-busy` alone dims a control and stops a second click, which is right for
 * something that takes 200ms. Shelling out to a coding agent takes thirty to
 * ninety seconds, and a button that is merely greyed out for ninety seconds
 * reads as broken — the report from `post` only arrives at the end. So a
 * control carrying `data-slow` swaps its label for that message while it works
 * and puts it back afterwards. Returns the undo. */
function saySlow(el) {
  const note = el?.dataset?.slow;
  if (!note) return () => {};
  const original = el.innerHTML;
  el.innerHTML =
    '<span class="spin" aria-hidden="true"></span>' +
    "<span>" +
    note.replace(/[<&]/g, (c) => (c === "<" ? "&lt;" : "&amp;")) +
    "</span>";
  return () => {
    el.innerHTML = original;
  };
}

/** Post JSON and re-render. Returns the parsed body, or null if it failed. */
async function post(url, body, el) {
  if (el) el.dataset.busy = "true";
  const undoSlow = saySlow(el);
  try {
    const res = await fetch(url, {
      method: "POST",
      headers: json,
      body: JSON.stringify(body ?? {}),
    });
    const text = await res.text();
    const data = text ? safeParse(text) : {};
    if (!res.ok) {
      report(data?.message || data?.error || text || res.statusText);
      return null;
    }
    if (data?.redirect) {
      // A document load, like every other link in this application: what the
      // fragment swap is kept for is re-rendering the page you are already on.
      location.href = data.redirect;
      return data;
    }
    await refresh();
    if (data?.message) report(data.message, "ok");
    return data;
  } catch (err) {
    report(String(err));
    return null;
  } finally {
    undoSlow();
    if (el) delete el.dataset.busy;
  }
}

function safeParse(text) {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

/** A transient line at the bottom of the window. Deliberately not a component:
 *  it has to outlive the outlet swap that a successful action triggers. */
function report(message, tone = "error") {
  let host = document.getElementById("forge-toast");
  if (!host) {
    host = document.createElement("div");
    host.id = "forge-toast";
    host.className =
      "fixed bottom-4 left-1/2 z-50 -translate-x-1/2 rounded-lg px-4 py-2 text-sm shadow-lg";
    document.body.appendChild(host);
  }
  host.className =
    "fixed bottom-4 left-1/2 z-50 -translate-x-1/2 rounded-lg px-4 py-2 text-sm shadow-lg " +
    (tone === "ok"
      ? "bg-success text-success-foreground"
      : "bg-destructive text-destructive-foreground");
  host.textContent = message;
  clearTimeout(host.timer);
  host.timer = setTimeout(() => host.remove(), tone === "ok" ? 3500 : 8000);
}

// A button that posts a fixed body: apply a template, move a screen, add a
// language. The endpoint and the body are attributes, so a new one is markup
// and nothing else.
document.addEventListener("click", (event) => {
  const el = event.target.closest("[data-action]");
  if (!el) return;
  event.preventDefault();
  if (el.dataset.confirm && !confirm(el.dataset.confirm)) return;
  post(el.dataset.action, safeParse(el.dataset.body || "{}"), el);
});

// A form that posts itself as JSON. Repeated names (the language checkboxes)
// become an array, which is what the endpoints declare.
document.addEventListener("submit", (event) => {
  const form = event.target.closest("[data-form]");
  if (!form) return;
  event.preventDefault();

  // event.submitter is the button that actually triggered this. It matters
  // because a modal's actions sit outside the form it submits (form="<id>"),
  // where a querySelector over the form's own subtree would not find them —
  // and the button is what gets the busy state.
  const submitter =
    event.submitter ||
    form.querySelector('[type="submit"]') ||
    [...form.elements].find((el) => el.type === "submit");
  post(form.dataset.form, serialize(form), submitter);
});

/** Post a form as multipart and re-render.
 *
 * The JSON path above cannot carry a file. Splitting the edit form into "upload
 * the icon" and then "save the name" would re-render the page between the two
 * and throw away whatever had been typed and not yet saved, so a form that
 * holds both goes as one request. */
document.addEventListener("submit", async (event) => {
  const form = event.target.closest("[data-form-multipart]");
  if (!form) return;
  event.preventDefault();

  const submitter =
    event.submitter || [...form.elements].find((el) => el.type === "submit");
  if (submitter) submitter.dataset.busy = "true";
  try {
    const res = await fetch(form.dataset.formMultipart, {
      method: "POST",
      body: new FormData(form),
    });
    const text = await res.text();
    if (!res.ok) {
      report(text || res.statusText);
      return;
    }
    await refresh();
  } catch (err) {
    report(String(err));
  } finally {
    if (submitter) delete submitter.dataset.busy;
  }
});

/** Turn a form into the JSON its endpoint declares.
 *
 * The only subtlety is which fields are arrays. A checkbox group — the
 * languages, the export's locale picker — has several inputs sharing one name
 * and must arrive as a list even when one or zero are ticked; everything else,
 * including a radio group, is a single value. Deciding that from "did I see
 * this key twice" is what produced `[["en-US"], "ja"]`: the first value got
 * wrapped once on the way in and again on the second sighting. */
function serialize(form) {
  const counts = {};
  for (const el of form.elements) {
    if (el.name) counts[el.name] = (counts[el.name] || 0) + 1;
  }
  const lists = new Set(
    [...form.elements]
      .filter(
        (el) =>
          el.name &&
          // A repeated checkbox group is a list by shape. A repeated <select>
          // is not — one per store slot, all named "frames" — and nothing in
          // the markup distinguishes it from a single control, so it says so
          // with data-list. So does a group that can render a single box — the
          // export's languages, on a project that ships in one — because one
          // box is a lone checkbox by shape, and `"locales":"pt-BR"` is a 400
          // from an endpoint declaring []string.
          ((el.type === "checkbox" && counts[el.name] > 1) ||
            el.dataset.list !== undefined),
      )
      .map((el) => el.name),
  );

  const body = {};
  // An empty group sends no entries at all, and a missing key reads as "not
  // specified" rather than "none of them" — so the lists are seeded first.
  for (const name of lists) body[name] = [];
  // A lone checkbox sends nothing when it is unticked, which an endpoint
  // declaring a bool decodes as false — right by accident, and wrong the
  // moment the field's default is true. `data-bool` sends the answer either
  // way, as a real boolean rather than the string "on".
  const bools = new Set();
  for (const el of form.elements) {
    if (el.name && el.type === "checkbox" && el.dataset.bool !== undefined) {
      body[el.name] = el.checked;
      bools.add(el.name);
    }
  }
  for (const [key, value] of new FormData(form).entries()) {
    // A ticked one is in FormData too, as its value — and letting that through
    // turned `listing: true` back into "1", a 400 on every export.
    if (bools.has(key)) continue;
    if (lists.has(key)) body[key].push(value);
    else body[key] = value;
  }
  return body;
}

// A panel toggle. The panel's open state is a checkbox and a CSS sibling
// selector — no JavaScript decides what is visible — but the *button* that
// flips it has to be a real button: a <button> inside a <label> never activates
// the label, because a label does not forward activation to an interactive
// element. That shipped once as "New project does nothing".
document.addEventListener("click", (event) => {
  const el = event.target.closest("[data-panel-toggle]");
  if (!el) return;
  event.preventDefault();
  const toggle = document.getElementById(el.dataset.panelToggle);
  if (!toggle) return;
  toggle.checked = !toggle.checked;
  el.setAttribute("aria-expanded", String(toggle.checked));
});

// The workspace sidebar's collapse toggle.
//
// shadcn-templ ships this behaviour in its own script, which this application
// does not serve: that script belongs to the interactive bundle, and its other
// half moves the sidebar into a sheet on small screens through a dialog
// component that is not here either. The desktop behaviour it actually needs
// is two data attributes — every collapsed-state rule in the component keys
// off them — so it is these few lines instead.
//
// The cookie is what makes it survive: every page here is a document load, so
// the sidebar is rendered fresh each time and a state held only in the DOM
// would come back open. The server reads the cookie and renders it the way it
// was left. Same name and same values shadcn's own script writes, so neither
// half has to know about the other.
const SIDEBAR_COOKIE = "sidebar_state";

/** Put a wrapper into a state. */
function paintSidebar(wrapper, collapsed) {
  const mode = wrapper.dataset.tuiSidebarCollapsibleMode;
  if (mode === "none") return;
  wrapper.dataset.state = collapsed ? "collapsed" : "expanded";
  // Like shadcn, data-collapsible carries the mode only while collapsed, so
  // the icon and offcanvas selectors need no second state check.
  wrapper.setAttribute("data-collapsible", collapsed ? mode : "");
}

document.addEventListener("click", (event) => {
  const trigger = event.target.closest("[data-tui-sidebar-trigger]");
  if (!trigger) return;
  const target = trigger.dataset.tuiSidebarTarget;
  const wrapper =
    (target &&
      document.querySelector(
        `[data-tui-sidebar-wrapper][data-tui-sidebar-id="${CSS.escape(target)}"]`,
      )) ||
    document.querySelector("[data-tui-sidebar-wrapper]");
  if (!wrapper) return;

  const collapsed = wrapper.dataset.state !== "collapsed";
  paintSidebar(wrapper, collapsed);
  document.cookie = `${SIDEBAR_COOKIE}=${collapsed ? "false" : "true"}; path=/; max-age=${60 * 60 * 24 * 7}`;
});

// The character counters on the Release step. The server renders the count and
// this keeps it honest while typing — the only other place the client draws
// before the server has answered, and for the same reason as the slider: a
// number that lags a keystroke reads as broken.
document.addEventListener("input", (event) => {
  const el = event.target.closest("[data-count]");
  if (!el) return;
  const out = document.querySelector(
    `[data-count-for="${CSS.escape(el.dataset.count)}"]`,
  );
  if (!out) return;
  const max = el.getAttribute("maxlength");
  // Characters, not bytes and not UTF-16 units: a description in Japanese is
  // 4000 characters at the store too, and el.value.length counts an emoji
  // twice.
  out.textContent = `${[...el.value].length}/${max}`;
});

// The export summary: how many folders the ticked boxes add up to.
//
// The server renders it and this keeps it honest while the boxes are clicked,
// for the same reason the character counters exist — the answer to "what will
// this write" has to be true before the button is pressed, not after.
function countSets(form) {
  const out = form.querySelector("[data-sets]");
  if (!out) return;
  const on = (name) =>
    [...form.querySelectorAll(`input[name="${name}"]`)].filter((el) => el.checked)
      .length;
  // No device group at all means the project ships to one, and it is implied.
  const devices = form.querySelector('input[name="sizes"]') ? on("sizes") : 1;
  const languages = on("locales");
  const sets = devices * languages;
  out.textContent = sets === 1 ? "1 folder" : `${sets} folders`;
  out.dataset.empty = String(sets === 0);
}

document.addEventListener("change", (event) => {
  const el = event.target.closest('input[name="sizes"], input[name="locales"]');
  if (el?.form) countSets(el.form);
});

// Also on arrival: a re-render brings back a form whose boxes may differ from
// the ones the last render counted.
document.addEventListener("howl:navigate", () => {
  document.querySelectorAll("form:has([data-sets])").forEach(countSets);
});

// A select that navigates: the device switcher in the rail. The value is
// appended to the attribute, so the markup carries the whole URL and this
// knows nothing about what is being switched.
document.addEventListener("change", (event) => {
  const el = event.target.closest("[data-goto]");
  if (!el) return;
  const url = el.dataset.goto + encodeURIComponent(el.value);
  location.href = url;
});

// The tune panel. Every control carries `data-patch` and the panel carries the
// endpoint, so one listener covers every control that exists or will exist.
document.addEventListener("change", (event) => {
  const el = event.target.closest("[data-patch]");
  if (!el) return;
  const scope = el.closest("[data-patch-scope]");
  const url = scope?.dataset.patchScope;
  if (!url) return;
  post(url, { key: el.name, value: el.value }, el);
});

// A button that sets one key to a fixed value — "no backdrop", "add a marker
// colour". Same endpoint as the controls; it just carries the value itself.
document.addEventListener("click", (event) => {
  const el = event.target.closest("[data-patch-set]");
  if (!el) return;
  const url = el.closest("[data-patch-scope]")?.dataset.patchScope;
  if (!url) return;
  post(url, { key: el.dataset.patchSet, value: el.dataset.value ?? "" }, el);
});

// A slider's number, updated as it moves. This is the one place the client
// draws something before the server has answered, and it is a label rather
// than a tile: the drawing itself always comes from the renderer.
document.addEventListener("input", (event) => {
  const el = event.target.closest("[data-live]");
  if (!el) return;
  const out = document.querySelector(`[data-range-value="${el.dataset.live}"]`);
  if (out) out.firstChild.nodeValue = el.value;
});

// Copy fields save when they lose focus, or on Enter. Not on every keystroke:
// each save re-renders the page, and re-rendering under the cursor while
// somebody is typing is how a text field eats characters.
document.addEventListener(
  "blur",
  (event) => {
    const el = event.target.closest?.("[data-copy]");
    if (!el || el.value === el.defaultValue) return;
    el.defaultValue = el.value;
    post(el.dataset.copy, {
      locale: el.dataset.locale,
      field: el.dataset.field,
      value: el.value,
    });
  },
  true,
);

document.addEventListener("keydown", (event) => {
  if (event.key !== "Enter" || event.shiftKey) return;
  const el = event.target.closest?.("[data-copy]");
  if (!el || el.tagName === "TEXTAREA") return;
  event.preventDefault();
  el.blur();
});

// Escape closes whatever modal is open. Everything else about a modal is a
// checkbox and a sibling selector; this is the only behaviour CSS has no way to
// express, and it is the first thing anyone tries.
document.addEventListener("keydown", (event) => {
  if (event.key !== "Escape") return;
  const open = document.querySelector("[data-modal]:checked");
  if (!open) return;
  open.checked = false;
  document
    .querySelector(`[data-panel-toggle="${open.id}"]`)
    ?.setAttribute("aria-expanded", "false");
});

// A button that opens a hidden file input. Same reason as the panel toggle: a
// <button> inside a <label for="…"> never reaches the input.
document.addEventListener("click", (event) => {
  const el = event.target.closest("[data-picks]");
  if (!el) return;
  event.preventDefault();
  document.getElementById(el.dataset.picks)?.click();
});

// Uploads: a file input, or a drop anywhere on the zone.
document.addEventListener("change", (event) => {
  const el = event.target.closest("[data-upload]");
  if (!el || !el.files?.length) return;
  send(el.dataset.upload, el.files, el.dataset.screen, el);
});

document.addEventListener("dragover", (event) => {
  const zone = event.target.closest("[data-drop]");
  if (!zone) return;
  event.preventDefault();
  zone.classList.add("dropping");
});

document.addEventListener("dragleave", (event) => {
  const zone = event.target.closest("[data-drop]");
  // dragleave fires for every nested element the pointer crosses; only the
  // one that actually leaves the zone should clear the highlight.
  if (zone && !zone.contains(event.relatedTarget)) zone.classList.remove("dropping");
});

document.addEventListener("drop", (event) => {
  const zone = event.target.closest("[data-drop]");
  if (!zone) return;
  event.preventDefault();
  zone.classList.remove("dropping");
  if (event.dataTransfer?.files?.length) send(zone.dataset.drop, event.dataTransfer.files, "", zone);
});

/** Does this look like an image?
 *
 * `file.type` is the obvious test and it is not reliable here. WKWebView builds
 * its File objects from the NSURLs the open panel returned and does not always
 * resolve a MIME type for them, so in the native window `type` is often "" —
 * and a filter written as `type.startsWith("image/")` rejects every file the
 * author picked, silently, while working perfectly in a browser.
 *
 * So: reject only what is positively not an image, fall back to the extension,
 * and when neither says anything let it through. The server decodes every
 * upload anyway and answers with a real message if it is not an image, which
 * makes it the honest authority rather than this. */
function looksLikeImage(file) {
  if (file.type) return file.type.startsWith("image/");
  return /\.(png|jpe?g|gif|webp|heic|heif|tiff?|bmp)$/i.test(file.name || "");
}

async function send(url, files, screen, el) {
  const body = new FormData();
  for (const file of files) {
    if (looksLikeImage(file)) body.append("files", file);
  }
  if (!body.has("files")) {
    report("Those do not look like images");
    return;
  }
  if (screen) body.append("screen", screen);

  if (el) el.dataset.busy = "true";
  try {
    const res = await fetch(url, { method: "POST", body });
    if (!res.ok) {
      report((await res.text()) || res.statusText);
      return;
    }
    await refresh();
  } catch (err) {
    report(String(err));
  } finally {
    if (el) delete el.dataset.busy;
  }
}
