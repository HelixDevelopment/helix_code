"""Demonstrate that the no_speech_prob threshold is a real, load-bearing guard.

With vad_filter=False (VAD disabled) Whisper's autoregressive decoder can emit
invented text on silence/noise (the report §5 risk-1 hallucination). This script
shows the raw decoder output AND that the no_speech_prob >= THRESH guard nulls it,
while golden_good survives the guard. Proves the analyzer cannot bluff.
"""
from faster_whisper import WhisperModel

THRESH = 0.6
m = WhisperModel("base", device="cpu", compute_type="int8")

for label, path in [
    ("golden_good", "/tmp/gg.wav"),
    ("silence", "/tmp/sil.wav"),
    ("noise", "/tmp/noi.wav"),
]:
    print(f"\n===== {label} : vad_filter=False (raw decoder, no VAD) =====")
    segs, info = m.transcribe(path, vad_filter=False, beam_size=5)
    any_seg = False
    kept = []
    for s in segs:
        any_seg = True
        nsp = float(s.no_speech_prob)
        guarded = nsp >= THRESH
        print(
            f"  seg text={s.text!r} no_speech_prob={nsp:.4f} "
            f"avg_logprob={s.avg_logprob:.4f} -> {'NULLED by guard' if guarded else 'KEPT'}"
        )
        if not guarded:
            kept.append(s.text)
    if not any_seg:
        print("  (no segments emitted)")
    print(f"  lang={info.language} lang_prob={info.language_probability:.4f}")
    print(f"  asserted-text-after-guard={''.join(kept).strip()!r}")
