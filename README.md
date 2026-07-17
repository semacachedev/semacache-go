# SemaCache Go SDK

Official Go client for [semacache.io](https://semacache.io) — semantic caching for LLM APIs.

Zero dependencies beyond the Go standard library.

## Installation

```bash
go get github.com/semacachedev/semacache-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    semacache "github.com/semacachedev/semacache-go"
)

func main() {
    client := semacache.NewClient("sc-your-key", nil)

    resp, err := client.CreateChatCompletion(context.Background(), semacache.ChatCompletionRequest{
        Model:    "gpt-4o",
        Messages: []semacache.Message{{Role: "user", Content: "What is semantic caching?"}},
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)

    // Cache metadata is always available
    fmt.Println(resp.Cache.MatchType)  // "EXACT", "SEMANTIC", or ""
    fmt.Println(resp.Cache.Confidence) // 0.991 (for semantic hits)
}
```

## Configuration

```go
client := semacache.NewClient("sc-your-key", &semacache.Options{
    UpstreamAPIKey:      "sk-your-openai-key", // optional: pass inline instead of dashboard
    SimilarityThreshold: 0.90,                 // optional: override default (0.95)
    CacheTTL:            3600,                 // optional: cache for 1 hour
})
```

## Per-Request Overrides

```go
resp, err := client.CreateChatCompletion(ctx, semacache.ChatCompletionRequest{
    Model:               "grok-3-mini",
    Messages:            messages,
    SimilarityThreshold: 0.85,
    CacheTTL:            7200,
    NoCache:             true, // skip cache read, still store
})
```

## Image Generation

```go
img, err := client.GenerateImage(ctx, semacache.ImageGenerateRequest{
    Prompt: "A sunset over mountains",
    Model:  "gpt-image-1",  // or "imagen-4.0-generate-001", "grok-imagine-image"
    Size:   "1024x1024",
})
fmt.Println(img.Data[0].URL)
fmt.Println(img.Cache.MatchType)
```

## Video Generation

```go
vid, err := client.GenerateVideo(ctx, semacache.VideoGenerateRequest{
    Prompt:          "A drone flyover of a city at sunset",
    Model:           "veo-3.0-generate-001",  // or "veo-3.1-generate-preview", "grok-imagine-video"
    DurationSeconds: 8,
    AspectRatio:     "16:9",
})
fmt.Println(vid.Data[0].URL)
fmt.Println(vid.Cache.MatchType)
```

## Passthrough params

Each request struct has an `Extras map[string]any` field. Anything you put
there is merged into the JSON body verbatim and forwarded to the upstream
provider. New OpenAI / Gemini / xAI params work the moment the provider
ships them. Use `"extra_body"` for provider-specific extensions.

```go
// Chat — forward temperature, tools, response_format, reasoning_effort, …
resp, err := client.CreateChatCompletion(ctx, semacache.ChatCompletionRequest{
    Model:    "gpt-5.4",
    Messages: []semacache.Message{{Role: "user", Content: "Summarize SemCache in one line"}},
    Extras: map[string]any{
        "temperature":       0.2,
        "reasoning_effort":  "high",
        "response_format":   map[string]any{"type": "json_object"},
    },
})

// Image — forward seed, negative_prompt, aspect_ratio, …
img, err := client.GenerateImage(ctx, semacache.ImageGenerateRequest{
    Prompt: "A red square on a white background",
    Model:  "imagen-4.0-generate-001",
    Extras: map[string]any{
        "seed":            42,
        "negative_prompt": "blurry, low quality",
        "aspect_ratio":    "16:9",
    },
})

// Video — forward resolution, enhance_prompt, negative_prompt, …
vid, err := client.GenerateVideo(ctx, semacache.VideoGenerateRequest{
    Prompt: "A drone flyover of a city",
    Model:  "veo-3.0-generate-001",
    Extras: map[string]any{
        "resolution":     "1080p",
        "enhance_prompt": true,
    },
})

// Gemini-specific escape hatch
resp, err = client.CreateChatCompletion(ctx, semacache.ChatCompletionRequest{
    Model:    "gemini-2.5-flash",
    Messages: []semacache.Message{{Role: "user", Content: "Hello"}},
    Extras: map[string]any{
        "extra_body": map[string]any{
            "safety_settings": []map[string]any{
                {"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_NONE"},
            },
        },
    },
})
```

## Supported Models

**Chat**
- **OpenAI**: gpt-5.4, gpt-5.4-mini, gpt-5.4-nano, gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, gpt-4o, gpt-4o-mini, o3, o3-mini, o4-mini
- **Gemini**: gemini-3.1-pro-preview, gemini-3-flash-preview, gemini-3.1-flash-lite-preview, gemini-2.5-pro, gemini-2.5-flash, gemini-2.5-flash-lite
- **xAI**: grok-4.20, grok-4, grok-4-fast, grok-3, grok-3-mini, grok-3-fast

**Images**
- **OpenAI**: gpt-image-1.5, gpt-image-1, gpt-image-1-mini, dall-e-3, dall-e-2
- **Google Imagen**: imagen-4.0-generate-001, imagen-4.0-ultra-generate-001, imagen-4.0-fast-generate-001
- **xAI**: grok-imagine-image, grok-imagine-image-pro

**Videos**
- **Google Veo**: veo-3.1-generate-preview, veo-3.1-fast-generate-preview, veo-3.1-lite-generate-preview, veo-3.0-generate-001, veo-3.0-fast-generate-001, veo-2.0-generate-001
- **xAI**: grok-imagine-video

**Custom**: any OpenAI-compatible endpoint registered in the dashboard

## Links

- [semacache.io](https://semacache.io)
- [Documentation](https://semacache.io/docs)
- [Dashboard](https://semacache.io/dashboard)
