# AI Providers

The AI Tool supports Gemini 2.5, Gemini 3.5, and Gemma 26B for chat and AI song actions. Whisper provides lyrics transcription.

The collapsible status panel checks all four services when the page opens and every 30 seconds. Gemini models are verified against the Gemini model API; Gemma and Whisper are checked with lightweight requests to their configured endpoints.

Gemini uses the existing `GeminiAPIKey` configuration. Gemma is called from the backend only and reads these environment variables:

```env
ND_GEMMA_API_URL=http://34.172.168.194:8081/chat
ND_GEMMA_API_KEY=your-secret-key
```

Do not put Gemma keys in frontend code. The AI Tool page stores only the selected default provider in browser local storage.
