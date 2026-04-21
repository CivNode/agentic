| Model | Tool call? | Final answer? | Iterations | Duration |
|-------|-----------|---------------|------------|----------|
| qwen3.5:8b | No | Error: iteration 0: chat: ollama: status 404 | 0 | 0s |
| qwen3.5:27b | Yes | Error: iteration 1: chat: ollama: Post "http://localhost:11434/api/chat": context deadline exceeded | 1 | 2m0s |
| qwen3.5:35b-a3b | Yes | Yes | 2 | 46s |
| qwen3:8b | Yes | Yes | 2 | 39s |
| llama3.3:latest | Yes | Yes | 2 | 1m51s |
| mistral-small:latest | Yes | Error: iteration 1: chat: ollama: Post "http://localhost:11434/api/chat": context deadline exceeded | 1 | 2m0s |
| mistral-nemo:latest | Yes | Yes | 2 | 19s |
