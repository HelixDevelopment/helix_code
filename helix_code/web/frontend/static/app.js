// app.js — plain-JS client for the HelixCode LLM generation endpoints.
//
// No build toolchain, no framework. Talks to the real server endpoints:
//   POST /api/v1/llm/generate  → JSON { content, provider, model, usage }
//   POST /api/v1/llm/stream    → text/event-stream of `data: <chunk>` frames
//
// Every byte rendered comes from the server's real provider response — there
// is no client-side simulation or canned output.

(function () {
  "use strict";

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
      headers: { "Content-Type": "application/json" },
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
      headers: { "Content-Type": "application/json" },
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
      headers: { "Content-Type": "application/json" },
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
})();
