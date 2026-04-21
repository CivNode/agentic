# Reliability matrix

Run date: 2026-04-21. Each model given up to 300s per run. The task is two-round: call `get_weather`, then produce a one-sentence summary after the tool returns. A "Yes / Yes" row means the model both called the tool and produced a coherent final answer. Failures on larger models on modest hardware are usually wall-clock limits, not transport bugs — the library's multi-round protocol is exercised separately by `tests/integration/TestOllamaIntegration_MultiRound`.

| Model | Tool call? | Final answer? | Iterations | Duration |
|-------|-----------|---------------|------------|----------|
| qwen3:8b | Yes | Yes | 2 | 1m3s |
| qwen3.5:27b | Yes | Yes | 2 | 2m24s |
| qwen3.5:35b-a3b | Yes | Yes | 2 | 54s |
| llama3.3:latest | Yes | Yes | 2 | 2m52s |
| mistral-small:latest | Yes | Yes | 2 | 2m53s |
| mistral-nemo:latest | Yes | Yes | 2 | 20s |
