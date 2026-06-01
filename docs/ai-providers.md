# AI Providers

The AI Tool supports Gemini 2.5, Gemini 3.5, and Gemma 4 for chat and AI song actions.

Gemini uses the existing `GeminiAPIKey` configuration. Gemma is called from the backend only and reads these environment variables:

```env
ND_GEMMA_API_URL=http://34.172.168.194:3001/chat
ND_GEMMA_API_KEY=your-secret-key
```

Do not put Gemma keys in frontend code. The AI Tool page stores only the selected default provider in browser local storage.
