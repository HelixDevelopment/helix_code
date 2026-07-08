#!/usr/bin/env python3
"""Live provider integration test — streaming, embeddings, tool-calling, nonce echo.

Tests every provider with an API_KEY env var set. Skips providers whose env var
is empty or unset. For each live provider proves:
  - Nonce echo (send a unique nonce, verify it is echoed verbatim)
  - Streaming (SSE-compatible chat completions, timed)
  - Embeddings (if the provider exposes an embeddings endpoint)
  - Tool/function calling (if the provider supports tool-use)

Output: docs/qa/phase1_providers_full_<ts>/RESULTS.md
"""

import os, sys, json, time, uuid, textwrap, traceback, datetime
from typing import Optional

httpx_avail = False
try:
    import httpx
    httpx_avail = True
except ImportError:
    pass

# ── Provider Registry ──────────────────────────────────────────────────────

Provider = dict  # type alias

OPENAI_COMPAT = "openai"   # /v1/chat/completions (OpenAI-compatible)
COHERE = "cohere"          # Cohere native /v1/chat
GEMINI = "gemini"          # Google Gemini generateContent
HUGGINGFACE_TGI = "hftgi" # HF Text Generation Inference
NVIDIA_NIM = "nvidia"      # NVIDIA NIM
ZHIPU = "zhipu"            # ZhipuAI (BigModel)
GITHUB_MODELS = "ghmodels" # GitHub Models Azure endpoint
REPLICATE = "replicate"    # Replicate API
CHUTES = "chutes"          # Chutes API

PROVIDERS: list[Provider] = [
    # ── Explicitly listed ──
    {"id": "mistral",    "key_env": "MISTRAL_API_KEY",       "api_type": OPENAI_COMPAT,
     "base_url": "https://api.mistral.ai/v1",
     "chat_model": "mistral-small-latest", "embed_model": "mistral-embed",
     "supports_stream": True, "supports_embed": True, "supports_tools": True},
    {"id": "codestral",  "key_env": "CODESTRAL_API_KEY",     "api_type": OPENAI_COMPAT,
     "base_url": "https://codestral.mistral.ai/v1",
     "chat_model": "codestral-latest", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": False},
    {"id": "groq",       "key_env": "GROQ_API_KEY",          "api_type": OPENAI_COMPAT,
     "base_url": "https://api.groq.com/openai/v1",
     "chat_model": "llama-3.3-70b-versatile", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "cohere",     "key_env": "COHERE_API_KEY",        "api_type": COHERE,
     "base_url": "https://api.cohere.ai/v1",
     "chat_model": "command-r-plus-08-2024", "embed_model": "embed-english-v3.0",
     "supports_stream": True, "supports_embed": True, "supports_tools": False},
    {"id": "cerebras",   "key_env": "CEREBRAS_API_KEY",      "api_type": OPENAI_COMPAT,
     "base_url": "https://api.cerebras.ai/v1",
     "chat_model": "llama3.1-8b", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "sambanova",  "key_env": "SAMBANOVA_API_KEY",     "api_type": OPENAI_COMPAT,
     "base_url": "https://api.sambanova.ai/v1",
     "chat_model": "Meta-Llama-3.3-70B-Instruct", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "siliconflow","key_env": "SILICONFLOW_API_KEY",   "api_type": OPENAI_COMPAT,
     "base_url": "https://api.siliconflow.cn/v1",
     "chat_model": "deepseek-ai/DeepSeek-V3", "embed_model": "BAAI/bge-m3",
     "supports_stream": True, "supports_embed": True, "supports_tools": True},
    {"id": "zai",        "key_env": "ZAI_API_KEY",           "api_type": OPENAI_COMPAT,
     "base_url": "https://open.bigmodel.cn/api/paas/v4",
     "chat_model": "glm-4-plus", "embed_model": "embedding-2",
     "supports_stream": True, "supports_embed": True, "supports_tools": True},
    # ── Others with API_KEY in name ──
    {"id": "deepseek",   "key_env": "DEEPSEEK_API_KEY",      "api_type": OPENAI_COMPAT,
     "base_url": "https://api.deepseek.com/v1",
     "chat_model": "deepseek-chat", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "fireworks",  "key_env": "FIREWORKS_API_KEY",     "api_type": OPENAI_COMPAT,
     "base_url": "https://api.fireworks.ai/inference/v1",
     "chat_model": "accounts/fireworks/models/llama-v3-70b-instruct", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "gemini",     "key_env": "GEMINI_API_KEY",        "api_type": GEMINI,
     "base_url": "https://generativelanguage.googleapis.com/v1beta",
     "chat_model": "gemini-2.0-flash", "embed_model": "models/embedding-001",
     "supports_stream": True, "supports_embed": True, "supports_tools": True},
    {"id": "github_models","key_env": "GITHUB_MODELS_API_KEY","api_type": GITHUB_MODELS,
     "base_url": "https://models.inference.ai.azure.com",
     "chat_model": "gpt-4o-mini", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "huggingface","key_env": "HUGGINGFACE_API_KEY",   "api_type": OPENAI_COMPAT,
     "base_url": "https://api-inference.huggingface.co/models/Qwen/Qwen2.5-72B-Instruct/v1",
     "chat_model": "Qwen/Qwen2.5-72B-Instruct", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": False},
    {"id": "hyperbolic", "key_env": "HYPERBOLIC_API_KEY",    "api_type": OPENAI_COMPAT,
     "base_url": "https://api.hyperbolic.xyz/v1",
     "chat_model": "meta-llama/Meta-Llama-3.3-70B-Instruct", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "openrouter", "key_env": "OPENROUTER_API_KEY",    "api_type": OPENAI_COMPAT,
     "base_url": "https://openrouter.ai/api/v1",
     "chat_model": "openai/gpt-4o-mini", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "novita",     "key_env": "NOVITA_API_KEY",        "api_type": OPENAI_COMPAT,
     "base_url": "https://api.novita.ai/v3/openai",
     "chat_model": "meta-llama/llama-3.1-8b-instruct", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "nvidia",     "key_env": "NVIDIA_API_KEY",        "api_type": NVIDIA_NIM,
     "base_url": "https://integrate.api.nvidia.com/v1",
     "chat_model": "meta/llama-3.3-70b-instruct", "embed_model": "nvidia/nv-embed-qa-4",
     "supports_stream": True, "supports_embed": True, "supports_tools": False},
    {"id": "nvidia_nv",  "key_env": "NVIDIA_API_KEY",        "api_type": OPENAI_COMPAT,
     "base_url": "https://integrate.api.nvidia.com/v1",
     "chat_model": "mistralai/mistral-7b-instruct-v0.3", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": False},
    {"id": "replicate",  "key_env": "REPLICATE_API_KEY",     "api_type": REPLICATE,
     "base_url": "https://api.replicate.com/v1",
     "chat_model": "meta/meta-llama-3-8b-instruct", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": False},
    {"id": "upstage",    "key_env": "UPSTAGE_API_KEY",       "api_type": OPENAI_COMPAT,
     "base_url": "https://api.upstage.ai/v1/solar",
     "chat_model": "solar-pro", "embed_model": "solar-embedding-1-large",
     "supports_stream": True, "supports_embed": True, "supports_tools": True},
    {"id": "chutes",     "key_env": "CHUTES_API_KEY",        "api_type": CHUTES,
     "base_url": "https://inference.chutes.ai/v1",
     "chat_model": "meta-llama/Meta-Llama-3.1-8B-Instruct", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": False},
    {"id": "venice",     "key_env": "VENICE_API_KEY",        "api_type": OPENAI_COMPAT,
     "base_url": "https://api.venice.ai/api/v1",
     "chat_model": "llama-3.2-3b", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "poe",        "key_env": "POE_API_KEY",           "api_type": OPENAI_COMPAT,
     "base_url": "https://api.poe.com/bot/gpt-4o-mini",
     "chat_model": "", "embed_model": None,
     "supports_stream": False, "supports_embed": False, "supports_tools": False},
    {"id": "tencentcloud","key_env": "TENCENT_CLOUD_API_KEY","api_type": OPENAI_COMPAT,
     "base_url": "https://api.lkeap.cloud.tencent.com/v1",
     "chat_model": "deepseek-v3", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": True},
    {"id": "publicai",   "key_env": "PUBLICAI_API_KEY",      "api_type": OPENAI_COMPAT,
     "base_url": "https://api.publicai.com/v1",
     "chat_model": "gpt-4o-mini", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": False},
    {"id": "zhipu",      "key_env": "ZHIPU_API_KEY",         "api_type": OPENAI_COMPAT,
     "base_url": "https://open.bigmodel.cn/api/paas/v4",
     "chat_model": "glm-4-plus", "embed_model": "embedding-2",
     "supports_stream": True, "supports_embed": True, "supports_tools": True},
    {"id": "zen",        "key_env": "ZEN_API_KEY",           "api_type": OPENAI_COMPAT,
     "base_url": "https://api.zen.com/v1",
     "chat_model": "gpt-4o-mini", "embed_model": None,
     "supports_stream": True, "supports_embed": False, "supports_tools": False},
]


# ── HTTP helpers ────────────────────────────────────────────────────────────

client = httpx.Client(timeout=httpx.Timeout(60.0, connect=15.0))
headers_json = {"Content-Type": "application/json"}

def oai_headers(api_key: str) -> dict:
    return {**headers_json, "Authorization": f"Bearer {api_key}"}

def do_post(url: str, headers: dict, payload: dict) -> httpx.Response:
    return client.post(url, headers=headers, json=payload)

def do_get(url: str, headers: dict) -> httpx.Response:
    return client.get(url, headers=headers)

# ── Embeddings ──────────────────────────────────────────────────────────────

def test_openai_embed(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/embeddings"
    payload = {"model": prov["embed_model"], "input": "Nonce echo verification test string.", "encoding_format": "float"}
    try:
        resp = do_post(url, oai_headers(key), payload)
        if resp.status_code == 200:
            data = resp.json()
            emb = data["data"][0]["embedding"]
            dim = len(emb)
            usage = data.get("usage", {}).get("total_tokens", 0)
            return {"pass": True, "dim": dim, "tokens": usage}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}"}
    except Exception as e:
        return {"pass": False, "error": str(e)}

def test_cohere_embed(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/embed"
    payload = {"model": prov["embed_model"], "texts": ["Nonce echo verification test string."], "input_type": "search_query"}
    try:
        resp = do_post(url, {"Authorization": f"Bearer {key}", **headers_json}, payload)
        if resp.status_code == 200:
            data = resp.json()
            emb = data["embeddings"][0]
            return {"pass": True, "dim": len(emb), "tokens": data.get("meta", {}).get("billed_units", {}).get("input_tokens", 0)}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}"}
    except Exception as e:
        return {"pass": False, "error": str(e)}

def test_gemini_embed(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/models/{prov['embed_model']}:embedContent?key={key}"
    payload = {"model": prov["embed_model"], "content": {"parts": [{"text": "Nonce echo verification test string."}]}}
    try:
        resp = do_post(url, headers_json, payload)
        if resp.status_code == 200:
            data = resp.json()
            emb = data["embedding"]["values"]
            return {"pass": True, "dim": len(emb)}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}"}
    except Exception as e:
        return {"pass": False, "error": str(e)}

def test_nvidia_embed(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/embeddings"
    payload = {"model": prov["embed_model"], "input": ["Nonce echo verification test string."]}
    try:
        resp = do_post(url, oai_headers(key), payload)
        if resp.status_code == 200:
            data = resp.json()
            emb = data["data"][0]["embedding"]
            return {"pass": True, "dim": len(emb)}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}"}
    except Exception as e:
        return {"pass": False, "error": str(e)}

def test_huggingface_embed(prov: Provider, key: str) -> dict:
    url = f"https://api-inference.huggingface.co/pipeline/feature-extraction/{prov['embed_model']}"
    headers = {"Authorization": f"Bearer {key}"}
    try:
        resp = do_post(url, headers, {"inputs": "Nonce echo verification test string.", "options": {"wait_for_model": True}})
        if resp.status_code == 200:
            data = resp.json()
            if data and isinstance(data, list) and data[0]:
                return {"pass": True, "dim": len(data[0])}
        return {"pass": False, "error": f"{resp.status_code}: {str(resp.text[:200])}"}
    except Exception as e:
        return {"pass": False, "error": str(e)}

def test_zhipu_embed(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/embeddings"
    payload = {"model": prov["embed_model"], "input": "Nonce echo verification test string."}
    try:
        resp = do_post(url, oai_headers(key), payload)
        if resp.status_code == 200:
            data = resp.json()
            emb = data["data"][0]["embedding"]
            return {"pass": True, "dim": len(emb)}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}"}
    except Exception as e:
        return {"pass": False, "error": str(e)}

# ── Nonce Echo ──────────────────────────────────────────────────────────────

def _do_openai_chat(url: str, headers: dict, model: str, messages: list, stream: bool = False, tools: list = None) -> httpx.Response:
    payload = {"model": model, "messages": messages, "max_tokens": 100, "temperature": 0}
    if stream:
        payload["stream"] = True
    if tools:
        payload["tools"] = tools
        payload["tool_choice"] = "auto"
    return do_post(url, headers, payload)

def test_nonce_echo_openai(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/chat/completions"
    nonce = f"NONCE-{uuid.uuid4().hex[:16].upper()}"
    messages = [{"role": "user", "content": f"Reply with EXACTLY this nonce and nothing else: {nonce}"}]
    try:
        resp = _do_openai_chat(url, oai_headers(key), prov["chat_model"], messages)
        if resp.status_code == 200:
            text = resp.json()["choices"][0]["message"]["content"]
            match = nonce in text.strip()
            return {"pass": match, "nonce": nonce, "response": text.strip(), "exact": text.strip() == nonce}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}", "nonce": nonce}
    except Exception as e:
        return {"pass": False, "error": str(e), "nonce": nonce}

def test_nonce_echo_gemini(prov: Provider, key: str) -> dict:
    nonce = f"NONCE-{uuid.uuid4().hex[:16].upper()}"
    url = f"{prov['base_url']}/models/{prov['chat_model']}:generateContent?key={key}"
    payload = {"contents": [{"parts": [{"text": f"Reply with EXACTLY this nonce and nothing else: {nonce}"}]}]}
    try:
        resp = do_post(url, headers_json, payload)
        if resp.status_code == 200:
            text = resp.json().get("candidates", [{}])[0].get("content", {}).get("parts", [{}])[0].get("text", "")
            match = nonce in text.strip()
            return {"pass": match, "nonce": nonce, "response": text.strip(), "exact": text.strip() == nonce}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}", "nonce": nonce}
    except Exception as e:
        return {"pass": False, "error": str(e), "nonce": nonce}

def test_nonce_echo_cohere(prov: Provider, key: str) -> dict:
    nonce = f"NONCE-{uuid.uuid4().hex[:16].upper()}"
    url = f"{prov['base_url']}/chat"
    payload = {"model": prov["chat_model"], "message": f"Reply with EXACTLY this nonce and nothing else: {nonce}", "max_tokens": 100, "temperature": 0}
    headers = {"Authorization": f"Bearer {key}", **headers_json}
    try:
        resp = do_post(url, headers, payload)
        if resp.status_code == 200:
            text = resp.json()["text"]
            match = nonce in text.strip()
            return {"pass": match, "nonce": nonce, "response": text.strip(), "exact": text.strip() == nonce}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}", "nonce": nonce}
    except Exception as e:
        return {"pass": False, "error": str(e), "nonce": nonce}

def test_nonce_echo_github(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/chat/completions"
    nonce = f"NONCE-{uuid.uuid4().hex[:16].upper()}"
    messages = [{"role": "user", "content": f"Reply with EXACTLY this nonce and nothing else: {nonce}"}]
    try:
        resp = _do_openai_chat(url, oai_headers(key), prov["chat_model"], messages)
        if resp.status_code == 200:
            text = resp.json()["choices"][0]["message"]["content"]
            match = nonce in text.strip()
            return {"pass": match, "nonce": nonce, "response": text.strip(), "exact": text.strip() == nonce}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}", "nonce": nonce}
    except Exception as e:
        return {"pass": False, "error": str(e), "nonce": nonce}

# Streaming test
def test_stream_openai(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/chat/completions"
    payload = {"model": prov["chat_model"], "messages": [{"role": "user", "content": "Count from 1 to 5."}], "max_tokens": 80, "temperature": 0, "stream": True}
    try:
        start = time.monotonic()
        chunks = 0
        full_text = ""
        with client.stream("POST", url, headers=oai_headers(key), json=payload) as resp:
            if resp.status_code != 200:
                return {"pass": False, "error": f"{resp.status_code}"}
            for line in resp.iter_lines():
                if line.startswith("data: "):
                    data_str = line[6:]
                    if data_str.strip() == "[DONE]":
                        break
                    try:
                        chunk = json.loads(data_str)
                        delta = chunk.get("choices", [{}])[0].get("delta", {}).get("content", "")
                        full_text += delta
                        chunks += 1
                    except json.JSONDecodeError:
                        pass
        elapsed = time.monotonic() - start
        return {"pass": chunks > 0, "chunks": chunks, "text_len": len(full_text), "elapsed_s": round(elapsed, 2)}
    except Exception as e:
        return {"pass": False, "error": str(e)}

def test_stream_gemini(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/models/{prov['chat_model']}:streamGenerateContent?key={key}&alt=sse"
    payload = {"contents": [{"parts": [{"text": "Count from 1 to 5."}]}]}
    try:
        start = time.monotonic()
        chunks = 0
        full_text = ""
        with client.stream("POST", url, headers=headers_json, json=payload) as resp:
            if resp.status_code != 200:
                return {"pass": False, "error": f"{resp.status_code}"}
            for line in resp.iter_lines():
                if line.startswith("data: "):
                    data_str = line[6:]
                    try:
                        chunk = json.loads(data_str)
                        text = chunk.get("candidates", [{}])[0].get("content", {}).get("parts", [{}])[0].get("text", "")
                        full_text += text
                        chunks += 1
                    except json.JSONDecodeError:
                        pass
        elapsed = time.monotonic() - start
        return {"pass": chunks > 0, "chunks": chunks, "text_len": len(full_text), "elapsed_s": round(elapsed, 2)}
    except Exception as e:
        return {"pass": False, "error": str(e)}

# Tool calling test
def test_tools_openai(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/chat/completions"
    tools = [{"type": "function", "function": {"name": "get_weather", "description": "Get weather for a city", "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}}}]
    messages = [{"role": "user", "content": "What's the weather in Paris? Use the get_weather tool."}]
    try:
        resp = _do_openai_chat(url, oai_headers(key), prov["chat_model"], messages, tools=tools)
        if resp.status_code == 200:
            msg = resp.json()["choices"][0]["message"]
            has_tool_call = "tool_calls" in msg and msg["tool_calls"]
            if has_tool_call:
                fn_name = msg["tool_calls"][0]["function"]["name"]
                return {"pass": True, "tool_call": fn_name}
            else:
                return {"pass": False, "error": "no tool_calls in response"}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}"}
    except Exception as e:
        return {"pass": False, "error": str(e)}

def test_tools_gemini(prov: Provider, key: str) -> dict:
    url = f"{prov['base_url']}/models/{prov['chat_model']}:generateContent?key={key}"
    tools = [{"function_declarations": [{"name": "get_weather", "description": "Get weather for a city", "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}}]}]
    payload = {"contents": [{"parts": [{"text": "What's the weather in Paris? Use the get_weather tool."}]}], "tools": tools}
    try:
        resp = do_post(url, headers_json, payload)
        if resp.status_code == 200:
            data = resp.json()
            candidates = data.get("candidates", [])
            if candidates:
                parts = candidates[0].get("content", {}).get("parts", [])
                for p in parts:
                    if "functionCall" in p:
                        return {"pass": True, "tool_call": p["functionCall"]["name"]}
                return {"pass": False, "error": "no functionCall in response parts"}
            return {"pass": False, "error": "no candidates"}
        else:
            return {"pass": False, "error": f"{resp.status_code}: {resp.text[:200]}"}
    except Exception as e:
        return {"pass": False, "error": str(e)}

# ── Main ────────────────────────────────────────────────────────────────────

def main():
    results = []
    # Filter to live providers (key is non-empty)
    live = [p for p in PROVIDERS if os.environ.get(p["key_env"], "")]

    print(f"Found {len(live)}/{len(PROVIDERS)} live providers")
    for p in PROVIDERS:
        if p not in live:
            print(f"  SKIP {p['id']}: {p['key_env']} is empty/unset")

    for prov in live:
        key = os.environ[prov["key_env"]]
        at = prov["api_type"]
        pid = prov["id"]
        print(f"\n{'='*60}")
        print(f"  {pid.upper()}")
        print(f"{'='*60}")

        row = {"id": pid, "tests": {}}

        # 1. Nonce Echo
        print(f"  ── Nonce echo ... ", end="", flush=True)
        if at == GEMINI:
            r = test_nonce_echo_gemini(prov, key)
        elif at == COHERE:
            r = test_nonce_echo_cohere(prov, key)
        elif at == GITHUB_MODELS:
            r = test_nonce_echo_github(prov, key)
        else:
            r = test_nonce_echo_openai(prov, key)
        row["tests"]["nonce_echo"] = r
        print(f"{'PASS' if r['pass'] else 'FAIL'}: {r.get('response','')[:60]}" if r['pass'] else f"FAIL: {r.get('error','')[:80]}")

        # 2. Streaming
        if prov["supports_stream"]:
            print(f"  ── Streaming ... ", end="", flush=True)
            if at == GEMINI:
                r = test_stream_gemini(prov, key)
            else:
                r = test_stream_openai(prov, key)
            row["tests"]["streaming"] = r
            print(f"{'PASS' if r['pass'] else 'FAIL'}: {r.get('chunks','?')} chunks, {r.get('text_len','?')} chars in {r.get('elapsed_s','?')}s" if r['pass'] else f"FAIL: {r.get('error','')[:80]}")

        # 3. Embeddings
        if prov["supports_embed"] and prov["embed_model"]:
            print(f"  ── Embeddings ... ", end="", flush=True)
            if at == COHERE:
                r = test_cohere_embed(prov, key)
            elif at == GEMINI:
                r = test_gemini_embed(prov, key)
            elif at == NVIDIA_NIM:
                r = test_nvidia_embed(prov, key)
            elif pid in ("huggingface",):
                r = test_huggingface_embed(prov, key)
            elif pid in ("zai", "zhipu"):
                r = test_zhipu_embed(prov, key)
            else:
                r = test_openai_embed(prov, key)
            row["tests"]["embeddings"] = r
            print(f"{'PASS' if r['pass'] else 'FAIL'}: dim={r.get('dim','?')}" if r['pass'] else f"FAIL: {r.get('error','')[:80]}")

        # 4. Tool calling
        if prov["supports_tools"]:
            print(f"  ── Tool calling ... ", end="", flush=True)
            if at == GEMINI:
                r = test_tools_gemini(prov, key)
            else:
                r = test_tools_openai(prov, key)
            row["tests"]["tool_calling"] = r
            print(f"{'PASS' if r['pass'] else 'FAIL'}: {r.get('tool_call','')}" if r['pass'] else f"FAIL: {r.get('error','')[:80]}")

        results.append(row)

    # ── Write RESULTS.md ──
    out_dir = os.path.dirname(os.path.abspath(__file__))
    out_path = os.path.join(out_dir, "RESULTS.md")
    ts = datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")

    with open(out_path, "w") as f:
        f.write(f"# Provider Phase 1 — Full Live Test Results\n\n")
        f.write(f"**Date:** {ts}\n\n")
        f.write(f"**Scope:** All providers with `API_KEY` env vars — nonce echo, streaming, embeddings, tool-calling\n\n")
        f.write(f"## Summary\n\n")
        f.write(f"| Provider | Nonce Echo | Streaming | Embeddings | Tool Calling | Status |\n")
        f.write(f"|----------|-----------|-----------|------------|--------------|--------|\n")

        summary_rows = []
        for r in results:
            ne = r["tests"].get("nonce_echo", {})
            st = r["tests"].get("streaming", {})
            em = r["tests"].get("embeddings", {})
            tc = r["tests"].get("tool_calling", {})
            status = "ALL PASS" if all([
                ne.get("pass"), st.get("pass", True), em.get("pass", True), tc.get("pass", True)
            ]) else "PARTIAL"
            ne_str = f"PASS{' *' if not ne.get('exact') else ''}" if ne.get("pass") else "FAIL"
            st_str = "PASS" if st.get("pass") else ("N/A" if "pass" not in st else "FAIL")
            em_str = "PASS" if em.get("pass") else ("N/A" if "pass" not in em else "FAIL")
            tc_str = "PASS" if tc.get("pass") else ("N/A" if "pass" not in tc else "FAIL")
            summary_rows.append(f"| {r['id']} | {ne_str} | {st_str} | {em_str} | {tc_str} | {status} |")

        f.write("\n".join(summary_rows))
        f.write("\n\n*\\* nonce present but not exact — model added extra text*\n\n")

        # Detailed per-provider
        f.write("## Detailed Results\n\n")
        for r in results:
            f.write(f"### {r['id']}\n\n")
            for test_name, result in r["tests"].items():
                status_emoji = "PASS" if result.get("pass") else "FAIL"
                f.write(f"- **{test_name}:** {status_emoji}\n")
                for k, v in result.items():
                    if k == "pass":
                        continue
                    f.write(f"  - {k}: `{v}`\n")
            f.write("\n")

        f.write("## Evidence\n\n")
        f.write(f"- Test script: `test_providers.py`\n")
        f.write(f"- Captured at: {ts}\n")
        f.write(f"- Git range: `{prov['chat_model'] if live else 'N/A'}` — full provider matrix\n")

    print(f"\n{'='*60}")
    print(f"  RESULTS written to {out_path}")
    print(f"  Summary: {len([r for r in results if all(t.get('pass',True) for t in r['tests'].values())])}/{len(results)} all-green")
    print(f"{'='*60}")

if __name__ == "__main__":
    main()
