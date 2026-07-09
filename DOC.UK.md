# anthropic - довідник

Повний довідник пакета `anthropic`: клієнт, спільна модель запиту/відповіді
`goloop/ai`, генерація, стрімінг, інструменти, зображення й нативні ендпоінти
Anthropic.

Англійська версія: **[DOC.md](DOC.md)**.

## Зміст

- [Ментальна модель](#ментальна-модель)
- [Створення клієнта](#створення-клієнта)
- [Generate](#generate)
- [Stream](#stream)
- [Інструменти](#інструменти)
- [Зображення](#зображення)
- [Підрахунок токенів](#підрахунок-токенів)
- [Моделі](#моделі)
- [Пакетні запити](#пакетні-запити)
- [Опції](#опції)
- [Помилки](#помилки)

## Ментальна модель

`anthropic.Client` реалізує `ai.Client` - провайдер-незалежний контракт із
`github.com/goloop/ai`. Ті самі типи `ai.Request`, `ai.Response`, `ai.Message`
і `ai.Tool` спільні для всіх goloop AI-провайдерів, тож зміна провайдера - це
зміна конструктора, а не місць виклику.

Ендпоінти, яких немає в інших провайдерів (підрахунок токенів, моделі, пакети),
є нативними методами `anthropic.Client` і не входять у спільний інтерфейс.

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/anthropic"
)
```

## Створення клієнта

```go
c := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))

c = anthropic.New(apiKey,
	anthropic.WithMaxTokens(1024),
	anthropic.WithTimeout(30*time.Second),
)
```

`New` потребує API-ключа; решта має значення за замовчуванням (base URL
`https://api.anthropic.com`, `anthropic-version` `2023-06-01`, `max_tokens`
`1024`, таймаут 60с і два ретраї).

## Generate

`Generate` надсилає один запит і повертає повну відповідь.

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:     anthropic.ModelClaudeSonnet5,
	MaxTokens: 256,
	System:    "You are concise.",
	Messages: []ai.Message{
		ai.UserText("Name three primary colors."),
	},
})

resp.Text()       // усі текстові частини разом
resp.ToolCalls()  // виклики інструментів, якщо були
resp.Usage        // токени входу/виходу
resp.StopReason   // "end_turn", "max_tokens", "tool_use", ...
```

`Message` - це `Role` і список частин `Part` (`ai.Text`, `ai.Image`,
`ai.ToolUse`, `ai.ToolResult`). `ai.UserText` і `ai.AssistantText` будують
одно-текстові повідомлення. Повідомлення `RoleSystem` згортається у верхньорівневий
system-промпт.

## Stream

`Stream` повертає `iter.Seq2[ai.Chunk, error]`. Текст приходить чанками з полем
`Text`; завершений виклик інструмента - чанком із `ToolCall`; фінальний чанк має
`Done == true` і несе `Usage`.

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		return err
	}
	fmt.Print(chunk.Text)
	if chunk.Done {
		_ = chunk.Usage
	}
}
```

Щоб зупинитися раніше - просто вийдіть із range; відповідь закриється за вас.

## Інструменти

Оголошуйте інструменти з JSON Schema для входу. Модель може відповісти викликами
інструментів замість тексту або разом із ним.

```go
req.Tools = []ai.Tool{{
	Name:        "get_weather",
	Description: "Get the current weather for a city.",
	Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
}}
req.ToolChoice = ai.ToolAuto // ToolAuto, ToolNone або ToolRequired
```

Обробіть виклик і надішліть результат назад повідомленням `RoleTool`, де
`ai.ToolResult.ID` збігається з `ai.ToolUse.ID`:

```go
call := resp.ToolCalls()[0]
result := runTool(call) // ваш код
req.Messages = append(req.Messages,
	ai.Message{Role: ai.RoleAssistant, Parts: []ai.Part{call}},
	ai.Message{Role: ai.RoleTool, Parts: []ai.Part{
		ai.ToolResult{ID: call.ID, Content: result},
	}},
)
```

## Зображення

Передавайте байти з MIME-типом або URL:

```go
ai.Image{MIME: "image/png", Data: pngBytes}
ai.Image{URL: "https://example.com/photo.jpg"}
```

Зображення - це частини всередині повідомлення користувача, поряд із текстом.

## Підрахунок токенів

```go
n, err := c.CountTokens(ctx, req)
```

Повертає кількість вхідних токенів для запиту без генерації.

## Моделі

```go
models, err := c.Models(ctx)
m, err := c.GetModel(ctx, "claude-sonnet-5")
```

`Model` містить `ID`, `DisplayName`, `CreatedAt`, `Type`.

## Пакетні запити

Надсилайте багато запитів для асинхронної обробки за нижчою ціною.

```go
batch, err := c.CreateBatch(ctx, []anthropic.BatchItem{
	{CustomID: "a", Request: reqA},
	{CustomID: "b", Request: reqB},
})

batch, err = c.GetBatch(ctx, batch.ID)      // опитуйте ProcessingStatus
results, err := c.BatchResults(ctx, batch)  // коли завершено

batches, err := c.ListBatches(ctx)
batch, err = c.CancelBatch(ctx, batch.ID)
```

`BatchResults` повертає по одному `BatchResult` на запит, зіставлений за
`CustomID`, із сирим JSON результату. Повертає `ErrNoResults`, якщо пакет ще не
завершився.

## Опції

Спільні опції однакові для всіх goloop AI-провайдерів:

- `WithBaseURL(url)` - перевизначити base URL.
- `WithHTTPClient(client)` - власний `*http.Client`.
- `WithTimeout(d)` - таймаут запиту, коли не задано власний клієнт.
- `WithMaxRetries(n)` - ретраї на 429 і 5xx (типово 2).
- `WithHeader(key, value)` - додати заголовок до кожного запиту.

Специфічні для Anthropic:

- `WithVersion(v)` - перевизначити заголовок `anthropic-version`.
- `WithBeta(features...)` - виставити прапорці `anthropic-beta`.
- `WithMaxTokens(n)` - типове `max_tokens`, коли запит його не задає.

## Помилки

Невдала HTTP-відповідь стає `*ai.APIError` зі `Status`, `Type`, `Message` і сирим
тілом. Перевіряйте через `errors.As`:

```go
var apiErr *ai.APIError
if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
	// backoff
}
```

Запити без моделі чи повідомлень падають до мережі з `ai.ErrNoModel` або
`ai.ErrNoMessages`.
