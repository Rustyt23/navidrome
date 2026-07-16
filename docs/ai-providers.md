# AI Providers

The AI Tool supports Gemini 2.5, Gemini 3.5, DeepSeek V3.2 on Amazon Bedrock, and Gemma for chat and AI song actions. Whisper provides lyrics transcription.

The collapsible status panel checks every configured service when the page opens and every 30 seconds. Gemini models are verified against the Gemini model API, DeepSeek through the Amazon Bedrock Converse API, and Gemma and Whisper through their configured endpoints.

Gemini uses the existing `GeminiAPIKey` configuration. DeepSeek V3.2 uses `AWSBearerTokenBedrock`; keep this bearer token in `navidrome.toml` and never expose it to frontend code. Gemma is called from the backend only and reads these environment variables:

```env
ND_GEMMA_API_URL=https://gemma.example.com/chat
ND_GEMMA_API_KEY=your-secret-key
```

Do not put provider keys in frontend code. The AI Tool page stores only the selected default provider in browser local storage.
