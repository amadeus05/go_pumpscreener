package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pumpscreener/src/core"
	"pumpscreener/src/domain"
)

type CommandHandler interface {
	AddRule(ctx context.Context, direction string, percent string, interval string, cooldown string, modeValues ...string) (string, error)
	ListRules(ctx context.Context) (string, error)
	RuleItems(ctx context.Context) ([]domain.Rule, error)
	DeleteRule(ctx context.Context, id string) (string, error)
	SetRuleEnabled(ctx context.Context, id string, enabled bool) (string, error)
	ListBlacklist(ctx context.Context) (string, error)
	AddBlacklist(ctx context.Context, symbol string) (string, error)
	RemoveBlacklist(ctx context.Context, symbol string) (string, error)
	Status(ctx context.Context) string
}

type Bot struct {
	token      string
	chatID     string
	apiBase    string
	httpClient *http.Client
	handler    CommandHandler
	offset     int64
}

func NewBot(token, chatID string, handler CommandHandler) *Bot {
	return &Bot{
		token:   token,
		chatID:  chatID,
		apiBase: "https://api.telegram.org/bot" + token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		handler: handler,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	if b.token == "" || b.chatID == "" {
		return nil
	}

	for {
		updates, err := b.getUpdates(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
				continue
			}
		}

		for _, update := range updates {
			if update.UpdateID >= b.offset {
				b.offset = update.UpdateID + 1
			}
			if update.CallbackQuery.ID != "" {
				if strconv.FormatInt(update.CallbackQuery.Message.Chat.ID, 10) != b.chatID {
					continue
				}
				b.handleCallback(ctx, update.CallbackQuery)
				continue
			}
			if update.Message.Text == "" {
				continue
			}
			if strconv.FormatInt(update.Message.Chat.ID, 10) != b.chatID {
				continue
			}
			reply, keyboard := b.handle(ctx, update.Message.Text)
			_ = b.SendMessageWithKeyboard(ctx, reply, keyboard)
		}
	}
}

func (b *Bot) NotifySignal(ctx context.Context, signal domain.Signal) error {
	if !b.Enabled() {
		return nil
	}
	return b.SendMessage(ctx, FormatSignal(signal))
}

func (b *Bot) SendMessage(ctx context.Context, text string) error {
	return b.SendMessageWithKeyboard(ctx, text, nil)
}

func (b *Bot) SendMessageWithKeyboard(ctx context.Context, text string, keyboard *inlineKeyboardMarkup) error {
	payload := map[string]any{
		"chat_id":                  b.chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	return b.post(ctx, "sendMessage", payload)
}

func (b *Bot) Enabled() bool {
	return b.token != "" && b.chatID != ""
}

func (b *Bot) getUpdates(ctx context.Context) ([]update, error) {
	values := url.Values{}
	values.Set("timeout", "25")
	values.Set("offset", strconv.FormatInt(b.offset, 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.apiBase+"/getUpdates?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload updatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.OK {
		return nil, fmt.Errorf("telegram getUpdates failed")
	}
	return payload.Result, nil
}

func (b *Bot) handle(ctx context.Context, text string) (string, *inlineKeyboardMarkup) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "Empty command", mainMenuKeyboard()
	}

	switch parts[0] {
	case "/start", "/menu":
		return "Pumpscreener control panel", mainMenuKeyboard()
	case "/help":
		return help(), mainMenuKeyboard()
	case "/status":
		return b.handler.Status(ctx), mainMenuKeyboard()
	case "/add":
		if len(parts) != 5 && len(parts) != 6 && len(parts) != 7 {
			return "Format: /add up 3 30m 15m or /add up 3 5m 1m hold", addRuleKeyboard()
		}
		return result(b.handler.AddRule(ctx, parts[1], parts[2], parts[3], parts[4], parts[5:]...)), mainMenuKeyboard()
	case "/rules":
		return b.rulesView(ctx)
	case "/delete":
		if len(parts) != 2 {
			return "Format: /delete 3", nil
		}
		return result(b.handler.DeleteRule(ctx, parts[1])), mainMenuKeyboard()
	case "/pause":
		if len(parts) != 2 {
			return "Format: /pause 3", nil
		}
		return result(b.handler.SetRuleEnabled(ctx, parts[1], false)), mainMenuKeyboard()
	case "/resume":
		if len(parts) != 2 {
			return "Format: /resume 3", nil
		}
		return result(b.handler.SetRuleEnabled(ctx, parts[1], true)), mainMenuKeyboard()
	case "/blacklist":
		return result(b.handler.ListBlacklist(ctx)), blacklistKeyboard()
	case "/blacklist_add":
		if len(parts) != 2 {
			return "Format: /blacklist_add BTCUSDT", nil
		}
		return result(b.handler.AddBlacklist(ctx, parts[1])), blacklistKeyboard()
	case "/blacklist_remove":
		if len(parts) != 2 {
			return "Format: /blacklist_remove BTCUSDT", nil
		}
		return result(b.handler.RemoveBlacklist(ctx, parts[1])), blacklistKeyboard()
	default:
		return "Unknown command. Use /menu", mainMenuKeyboard()
	}
}

func (b *Bot) handleCallback(ctx context.Context, callback callbackQuery) {
	_ = b.answerCallback(ctx, callback.ID, "")

	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	data := callback.Data

	switch {
	case data == "menu":
		_ = b.editMessage(ctx, chatID, messageID, "Pumpscreener control panel", mainMenuKeyboard())
	case data == "status":
		_ = b.editMessage(ctx, chatID, messageID, b.handler.Status(ctx), mainMenuKeyboard())
	case data == "help":
		_ = b.editMessage(ctx, chatID, messageID, help(), mainMenuKeyboard())
	case data == "rules":
		text, keyboard := b.rulesView(ctx)
		_ = b.editMessage(ctx, chatID, messageID, text, keyboard)
	case data == "add":
		_ = b.editMessage(ctx, chatID, messageID, addHelp(), addRuleKeyboard())
	case data == "blacklist":
		_ = b.editMessage(ctx, chatID, messageID, result(b.handler.ListBlacklist(ctx)), blacklistKeyboard())
	case strings.HasPrefix(data, "rule:pause:"):
		id := strings.TrimPrefix(data, "rule:pause:")
		message := result(b.handler.SetRuleEnabled(ctx, id, false))
		text, keyboard := b.rulesView(ctx)
		_ = b.editMessage(ctx, chatID, messageID, message+"\n\n"+text, keyboard)
	case strings.HasPrefix(data, "rule:resume:"):
		id := strings.TrimPrefix(data, "rule:resume:")
		message := result(b.handler.SetRuleEnabled(ctx, id, true))
		text, keyboard := b.rulesView(ctx)
		_ = b.editMessage(ctx, chatID, messageID, message+"\n\n"+text, keyboard)
	case strings.HasPrefix(data, "rule:delete:"):
		id := strings.TrimPrefix(data, "rule:delete:")
		message := result(b.handler.DeleteRule(ctx, id))
		text, keyboard := b.rulesView(ctx)
		_ = b.editMessage(ctx, chatID, messageID, message+"\n\n"+text, keyboard)
	default:
		_ = b.editMessage(ctx, chatID, messageID, "Unknown action", mainMenuKeyboard())
	}
}

func (b *Bot) rulesView(ctx context.Context) (string, *inlineKeyboardMarkup) {
	rules, err := b.handler.RuleItems(ctx)
	if err != nil {
		return "Error: " + err.Error(), mainMenuKeyboard()
	}
	if len(rules) == 0 {
		return "No rules yet. Add one with /add up 3 30m 15m", addRuleKeyboard()
	}

	lines := make([]string, 0, len(rules)+1)
	lines = append(lines, "Rules:")
	keyboard := &inlineKeyboardMarkup{}
	for _, rule := range rules {
		status := "on"
		actionText := "Pause"
		actionData := fmt.Sprintf("rule:pause:%d", rule.ID)
		if !rule.Enabled {
			status = "off"
			actionText = "Resume"
			actionData = fmt.Sprintf("rule:resume:%d", rule.ID)
		}

		lines = append(lines, fmt.Sprintf(
			"#%d [%s] %s %.2f%% / %s, cooldown %s, mode %s%s",
			rule.ID,
			status,
			rule.Direction,
			rule.Percent,
			core.HumanDuration(rule.Interval),
			core.HumanDuration(rule.Cooldown),
			rule.EffectiveMode(),
			formatHold(rule),
		))
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []inlineKeyboardButton{
			{Text: fmt.Sprintf("#%d %s", rule.ID, actionText), CallbackData: actionData},
			{Text: fmt.Sprintf("#%d Delete", rule.ID), CallbackData: fmt.Sprintf("rule:delete:%d", rule.ID)},
		})
	}
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []inlineKeyboardButton{
		{Text: "Add rule", CallbackData: "add"},
		{Text: "Menu", CallbackData: "menu"},
	})
	return strings.Join(lines, "\n"), keyboard
}

func (b *Bot) editMessage(ctx context.Context, chatID int64, messageID int64, text string, keyboard *inlineKeyboardMarkup) error {
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	return b.post(ctx, "editMessageText", payload)
}

func (b *Bot) answerCallback(ctx context.Context, callbackID string, text string) error {
	payload := map[string]any{
		"callback_query_id": callbackID,
	}
	if text != "" {
		payload["text"] = text
	}
	return b.post(ctx, "answerCallbackQuery", payload)
}

func (b *Bot) post(ctx context.Context, method string, payload map[string]any) error {
	if !b.Enabled() {
		return nil
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.apiBase+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram %s returned status %d", method, resp.StatusCode)
	}
	return nil
}

func result(message string, err error) string {
	if err != nil {
		return "Error: " + err.Error()
	}
	return message
}

func help() string {
	return strings.Join([]string{
		"Pumpscreener commands:",
		"/menu",
		"/add up 3 30m 15m",
		"/add up 3 5m 1m hold",
		"/add down 5 1h 30m",
		"/rules",
		"/delete 3",
		"/pause 3",
		"/resume 3",
		"/blacklist",
		"/blacklist_add BTCUSDT",
		"/blacklist_remove BTCUSDT",
		"/status",
	}, "\n")
}

func addHelp() string {
	return strings.Join([]string{
		"Add a rule by sending one of these commands:",
		"/add up 3 30m 15m",
		"/add down 5 30m 15m",
		"/add up 3 5m 1m hold",
		"",
		"Format: /add direction percent interval cooldown [hold]",
		"For hold rules, interval is also the hold time.",
	}, "\n")
}

func mainMenuKeyboard() *inlineKeyboardMarkup {
	return &inlineKeyboardMarkup{InlineKeyboard: [][]inlineKeyboardButton{
		{
			{Text: "Status", CallbackData: "status"},
			{Text: "Rules", CallbackData: "rules"},
		},
		{
			{Text: "Add rule", CallbackData: "add"},
			{Text: "Blacklist", CallbackData: "blacklist"},
		},
		{
			{Text: "Help", CallbackData: "help"},
		},
	}}
}

func addRuleKeyboard() *inlineKeyboardMarkup {
	return &inlineKeyboardMarkup{InlineKeyboard: [][]inlineKeyboardButton{
		{
			{Text: "Rules", CallbackData: "rules"},
			{Text: "Menu", CallbackData: "menu"},
		},
	}}
}

func blacklistKeyboard() *inlineKeyboardMarkup {
	return &inlineKeyboardMarkup{InlineKeyboard: [][]inlineKeyboardButton{
		{
			{Text: "Refresh blacklist", CallbackData: "blacklist"},
			{Text: "Menu", CallbackData: "menu"},
		},
	}}
}

func FormatSignal(signal domain.Signal) string {
	symbol := html.EscapeString(signal.Symbol)
	link := fmt.Sprintf("https://www.bybit.com/trade/usdt/%s", url.PathEscape(signal.Symbol))
	startPrice := priceBeforeMove(signal.Price, signal.Move)
	directionIcon := "📈"
	move := fmt.Sprintf("+%.2f%%", math.Abs(signal.Move))
	if signal.Move < 0 {
		directionIcon = "📉"
		move = fmt.Sprintf("-%.2f%%", math.Abs(signal.Move))
	}

	return fmt.Sprintf(
		"🚨 №%d - <a href=\"%s\">%s</a> %s\n%s %s\n💵 %s → %s • ⏰ %s",
		signal.DailyNo,
		link,
		symbol,
		formatSignalDuration(signal.Window),
		directionIcon,
		move,
		formatSignalPrice(startPrice),
		formatSignalPrice(signal.Price),
		signal.Triggered.Format("15:04:05"),
	)
}

func priceBeforeMove(price float64, move float64) float64 {
	base := 1 + move/100
	if base == 0 {
		return price
	}
	return price / base
}

func formatSignalPrice(price float64) string {
	switch {
	case price >= 100:
		return fmt.Sprintf("%.2f", price)
	case price >= 1:
		return fmt.Sprintf("%.4f", price)
	default:
		return fmt.Sprintf("%.4f", price)
	}
}

func formatSignalDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	if value%(24*time.Hour) == 0 && value >= 24*time.Hour {
		return fmt.Sprintf("%dd", int(value/(24*time.Hour)))
	}
	if value%time.Hour == 0 && value >= time.Hour {
		return fmt.Sprintf("%dh", int(value/time.Hour))
	}
	if value%time.Minute == 0 && value >= time.Minute {
		return fmt.Sprintf("%dm", int(value/time.Minute))
	}
	return fmt.Sprintf("%ds", int(value/time.Second))
}

func formatHold(rule domain.Rule) string {
	if rule.EffectiveMode() != domain.RuleModeHold {
		return ""
	}
	return " " + core.HumanDuration(rule.Hold)
}

type updatesResponse struct {
	OK     bool     `json:"ok"`
	Result []update `json:"result"`
}

type update struct {
	UpdateID      int64         `json:"update_id"`
	Message       message       `json:"message"`
	CallbackQuery callbackQuery `json:"callback_query"`
}

type message struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}

type callbackQuery struct {
	ID      string  `json:"id"`
	Data    string  `json:"data"`
	Message message `json:"message"`
}

type inlineKeyboardMarkup struct {
	InlineKeyboard [][]inlineKeyboardButton `json:"inline_keyboard"`
}

type inlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}
