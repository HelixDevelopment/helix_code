// app.js — plain-JS client for the HelixCode LLM generation endpoints.
//
// No build toolchain, no framework. Talks to the real server endpoints:
//   POST /api/v1/auth/login    → JSON { token, user, session } (mints a JWT)
//   POST /api/v1/llm/generate  → JSON { content, provider, model, usage }
//   POST /api/v1/llm/stream    → text/event-stream of `data: <chunk>` frames
//   POST /api/v1/specify        → JSON { output, provider, model, ... }
//
// Auth: the generation surfaces (/llm/generate, /llm/stream) and /specify are
// auth-gated server-side (server.go: the llmCost + specify route groups carry
// authMiddleware / VerifyJWTWithDB) because they drive real, paid providers.
// The console logs in via the real /api/v1/auth/login endpoint, stores the
// returned JWT in sessionStorage, and sends it as `Authorization: Bearer
// <token>` on every paid call. Without a token those calls return 401 and the
// output never populates — so login is required before generating.
//
// Every byte rendered comes from the server's real provider response — there
// is no client-side simulation or canned output. No credential is hardcoded —
// the operator types username + password into the real login form.

(function () {
  "use strict";

  // --- Auth token store -----------------------------------------------------
  // The JWT lives in sessionStorage (cleared when the tab closes) — a session
  // credential, never persisted to disk or logged. authHeaders() merges the
  // Bearer header into a request's headers ONLY when a token is present, so
  // unauthenticated GETs (provider/model discovery) still work token-free.
  var TOKEN_KEY = "helixcode.jwt";

  function getToken() {
    try {
      return window.sessionStorage.getItem(TOKEN_KEY) || "";
    } catch (e) {
      return "";
    }
  }

  function setToken(tok) {
    try {
      if (tok) window.sessionStorage.setItem(TOKEN_KEY, tok);
      else window.sessionStorage.removeItem(TOKEN_KEY);
    } catch (e) {
      /* sessionStorage unavailable (private mode); token stays in memory only */
    }
  }

  function authHeaders(base) {
    var h = base || {};
    var tok = getToken();
    if (tok) h["Authorization"] = "Bearer " + tok;
    return h;
  }

  var form = document.getElementById("gen-form");
  var output = document.getElementById("output");
  var meta = document.getElementById("meta");
  var sendBtn = document.getElementById("send");

  function setMeta(text, isError) {
    meta.textContent = text || "";
    meta.className = "meta" + (isError ? " error" : "");
  }

  function buildBody() {
    var body = { prompt: document.getElementById("prompt").value };
    var provider = document.getElementById("provider").value.trim();
    var model = document.getElementById("model").value.trim();
    if (provider) body.provider = provider;
    if (model) body.model = model;
    return body;
  }

  async function generateOnce(body) {
    var res = await fetch("/api/v1/llm/generate", {
      method: "POST",
      headers: authHeaders({ "Content-Type": "application/json" }),
      body: JSON.stringify(body),
    });
    var data = await res.json();
    if (!res.ok || data.status === "error") {
      throw new Error(data.error || ("HTTP " + res.status));
    }
    output.textContent = data.content || "";
    var u = data.usage || {};
    setMeta(
      "provider=" + (data.provider || "?") +
      "  model=" + (data.model || "?") +
      "  tokens=" + (u.total_tokens || 0) +
      "  finish=" + (data.finish_reason || "?")
    );
  }

  async function generateStream(body) {
    output.textContent = "";
    var res = await fetch("/api/v1/llm/stream", {
      method: "POST",
      headers: authHeaders({ "Content-Type": "application/json" }),
      body: JSON.stringify(body),
    });
    if (!res.ok || !res.body) {
      var errData = await res.json().catch(function () { return {}; });
      throw new Error(errData.error || ("HTTP " + res.status));
    }
    var reader = res.body.getReader();
    var decoder = new TextDecoder();
    var buf = "";
    for (;;) {
      var chunk = await reader.read();
      if (chunk.done) break;
      buf += decoder.decode(chunk.value, { stream: true });
      var frames = buf.split("\n\n");
      buf = frames.pop(); // keep incomplete trailing frame
      for (var i = 0; i < frames.length; i++) {
        var line = frames[i].trim();
        if (line.indexOf("data:") !== 0) continue;
        var payload = line.slice(5).trim();
        if (payload === "[DONE]") { setMeta("stream complete"); return; }
        output.textContent += payload.replace(/\\n/g, "\n");
      }
    }
    setMeta("stream complete");
  }

  form.addEventListener("submit", async function (e) {
    e.preventDefault();
    var body = buildBody();
    var stream = document.getElementById("stream").checked;
    sendBtn.disabled = true;
    setMeta("requesting…");
    output.textContent = "";
    try {
      if (stream) {
        await generateStream(body);
      } else {
        await generateOnce(body);
      }
    } catch (err) {
      setMeta((err && err.message) || String(err), true);
    } finally {
      sendBtn.disabled = false;
    }
  });

  // --- Specify phase (POST /api/v1/specify) ---------------------------------
  //
  // Same pattern as generateOnce: POST a JSON body, then render the server's
  // REAL fields on success or surface the server's REAL error string on
  // failure. No client-side simulation — every byte rendered comes from the
  // speckit engine's actual provider-backed response, and the 502 deadline /
  // provider error is shown verbatim, never faked into a success.

  var specForm = document.getElementById("spec-form");
  var specOutput = document.getElementById("spec-output");
  var specMeta = document.getElementById("spec-meta");
  var specSendBtn = document.getElementById("spec-send");

  function setSpecMeta(text, isError) {
    specMeta.textContent = text || "";
    specMeta.className = "meta" + (isError ? " error" : "");
  }

  function buildSpecBody() {
    var body = { request: document.getElementById("spec-request").value };
    var provider = document.getElementById("spec-provider").value.trim();
    var model = document.getElementById("spec-model").value.trim();
    if (provider) body.provider = provider;
    if (model) body.model = model;
    return body;
  }

  async function specifyOnce(body) {
    var res = await fetch("/api/v1/specify", {
      method: "POST",
      headers: authHeaders({ "Content-Type": "application/json" }),
      body: JSON.stringify(body),
    });
    var data = await res.json();
    if (!res.ok || data.status === "error") {
      throw new Error(data.error || ("HTTP " + res.status));
    }
    specOutput.textContent = data.output || "";
    setSpecMeta(
      "provider=" + (data.provider || "?") +
      "  model=" + (data.model || "?") +
      "  qualityScore=" + (data.qualityScore != null ? data.qualityScore : "?") +
      "  debateID=" + (data.debateID || "?") +
      "  success=" + (data.success === true)
    );
  }

  specForm.addEventListener("submit", async function (e) {
    e.preventDefault();
    var body = buildSpecBody();
    specSendBtn.disabled = true;
    setSpecMeta("specifying…");
    specOutput.textContent = "";
    try {
      await specifyOnce(body);
    } catch (err) {
      setSpecMeta((err && err.message) || String(err), true);
    } finally {
      specSendBtn.disabled = false;
    }
  });

  // --- Login (POST /api/v1/auth/login) --------------------------------------
  //
  // The paid endpoints above are auth-gated; the operator logs in here to mint
  // a real JWT. We POST the real login endpoint, read the server's real
  // `token`, and store it in sessionStorage. No credential is hardcoded — the
  // username/password come from the real form inputs. The auth status line
  // reflects the live token state read back from sessionStorage.

  var loginForm = document.getElementById("login-form");
  var loginStatus = document.getElementById("auth-status");
  var loginBtn = document.getElementById("login-send");
  var logoutBtn = document.getElementById("logout");

  function setAuthStatus(text, isError) {
    if (!loginStatus) return;
    loginStatus.textContent = text || "";
    loginStatus.className = "meta" + (isError ? " error" : "");
  }

  function refreshAuthStatus() {
    if (getToken()) {
      setAuthStatus("authenticated (token stored)");
    } else {
      setAuthStatus("not authenticated — log in to generate");
    }
  }

  async function loginOnce(username, password) {
    var res = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: username, password: password }),
    });
    var data = await res.json();
    if (!res.ok || data.status === "error" || !data.token) {
      throw new Error(data.error || data.message || ("HTTP " + res.status));
    }
    setToken(data.token);
  }

  if (loginForm) {
    loginForm.addEventListener("submit", async function (e) {
      e.preventDefault();
      var username = document.getElementById("login-username").value.trim();
      var password = document.getElementById("login-password").value;
      loginBtn.disabled = true;
      setAuthStatus("logging in…");
      try {
        await loginOnce(username, password);
        refreshAuthStatus();
      } catch (err) {
        setToken("");
        setAuthStatus((err && err.message) || String(err), true);
      } finally {
        loginBtn.disabled = false;
      }
    });
  }

  if (logoutBtn) {
    logoutBtn.addEventListener("click", function () {
      setToken("");
      refreshAuthStatus();
    });
  }

  refreshAuthStatus();
})();
