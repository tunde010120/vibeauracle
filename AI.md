# AI Integration Blueprint: GitHub Models

This document provides a technical blueprint for integrating AI capabilities using the **GitHub Models API**. While this specific tool uses AI for generating git commit messages, the integration pattern described here is generic and can be applied to any tool requiring access to diverse LLMs (Large Language Models) through a unified interface.

## 1. Architecture Overview

The integration follows a "Unified API" pattern. Instead of integrating multiple SDKs (OpenAI, Anthropic, Mistral), we use the **GitHub Models API**, which provides an OpenAI-compatible interface for dozens of leading models.

### Key Components
- **Model Manager**: Handles discovery and caching of available models.
- **API Client**: Manages authentication and HTTP communication.
- **Prompt Builder**: Structures application data into messages for the LLM.

---

## 2. Authentication

The API uses a standard GitHub Personal Access Token (PAT) as a Bearer token. No special SDK is required; standard HTTP headers are sufficient.

```rust
pub async fn call_inference_api(api_key: &str, prompt: &str, model: &str) -> Result<String> {
    let client = reqwest::Client::new();
    let url = "https://models.inference.ai.azure.com/chat/completions";

    let response = client
        .post(url)
        .header("Content-Type", "application/json")
        .header("Authorization", format!("Bearer {}", api_key))
        .json(&request)
        .send()
        .await?;
    
    // ... handle response ...
}
```

---

## 3. Model Discovery & Caching

One of the most powerful features of the GitHub Models API is the ability to dynamically discover available models. This allows your tool to automatically support new models as they are added to the platform.

### Fetching Models
The `/models` endpoint returns metadata for all available models, including their tasks (e.g., `chat-completion`).

```rust
pub async fn fetch_available_models(api_key: &str) -> Result<Vec<ModelInfo>> {
    let client = reqwest::Client::new();
    let url = "https://models.inference.ai.azure.com/models";

    let response = client
        .get(url)
        .header("Accept", "application/vnd.github+json")
        .header("Authorization", format!("Bearer {}", api_key))
        .header("X-GitHub-Api-Version", "2022-11-28")
        .send()
        .await?;

    let models_response: Vec<serde_json::Value> = response.json().await?;
    let models: Vec<ModelInfo> = models_response
        .iter()
        .filter_map(|m| {
            // Filter for chat models specifically
            if m.get("task").and_then(|t| t.as_str()) == Some("chat-completion") {
                Some(ModelInfo {
                    id: m.get("name")?.as_str()?.to_string(),
                    name: m.get("name")?.as_str()?.to_string(),
                    // ... other metadata ...
                })
            } else {
                None
            }
        })
        .collect();

    Ok(models)
}
```

### Caching Strategy
To avoid network latency on every run, cache the model list locally.

```rust
pub fn update_cached_models(models: &[ModelInfo]) -> Result<()> {
    let cache_file = get_models_cache_file()?;
    let cached = CachedModels { models: models.to_vec() };
    let content = serde_json::to_string_pretty(&cached)?;
    std::fs::write(&cache_file, content)?;
    Ok(())
}
```

---

## 4. Inference Integration

The core interaction uses a Chat Completion schema. This involves a `system` prompt to define behavior and a `user` prompt for the actual data.

### Request Structure

```rust
#[derive(Debug, Serialize)]
struct Message {
    role: String,
    content: String,
}

#[derive(Debug, Serialize)]
struct ChatCompletionRequest {
    messages: Vec<Message>,
    model: String,
    // Optional: temperature, max_tokens, etc.
}
```

### Making the Request

```rust
pub async fn generate_content(api_key: &str, system_msg: &str, user_input: &str, model: &str) -> Result<String> {
    let request = ChatCompletionRequest {
        messages: vec![
            Message {
                role: "system".to_string(),
                content: system_msg.to_string(),
            },
            Message {
                role: "user".to_string(),
                content: user_input.to_string(),
            },
        ],
        model: model.to_string(),
    };

    let response = client.post(url).json(&request).send().await?;
    let response_data: ChatCompletionResponse = response.json().await?;
    
    // Extraction logic
    Ok(response_data.choices[0].message.content.unwrap_or_default())
}
```

---

## 5. Advanced Capabilities

### Multi-Model Support
By using a unified API, users can switch between models like `gpt-4o`, `Phi-3`, or `Mistral-large` without code changes.

```rust
// Example of a fallback or selection mechanism
let selected_model = match user_preference {
    Some(m) => m,
    None => "gpt-4o-mini", // Cost-effective default
};
```

### Context Compression
When dealing with large inputs (like code diffs), it's essential to compress data to fit within token limits and reduce costs.

```rust
pub fn compress_to_json(file_changes: &[FileChange], max_len: usize) -> String {
    // Logic to progressively truncate data until it fits 'max_len'
    // This ensures the AI receives the most important context first.
    // ... implementation details ...
}
```

### Streaming (Optional)
The API supports Server-Sent Events (SSE) for streaming responses, allowing for real-time UI updates.

```rust
// Set "stream": true in the request
// Read the response body as a stream of data chunks
```

## 6. Best Practices

1.  **System Prompting**: Always use a `system` role to constrain the output format (e.g., "Reply only with JSON", "Be concise").
2.  **Error Handling**: Implement robust error handling for API rate limits (429) and auth errors (401).
3.  **Model Flexibility**: Don't hardcode a single model. Let the platform's variety be an advantage for the user.
4.  **Token Efficiency**: Only send the necessary parts of your data. Large, irrelevant blobs of text increase latency and cost.

