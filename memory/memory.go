package memory


import (
	"log"
	"fmt"
    "strings"

    tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)


func convertToLine(msg *tg.Message) string {
	if msg == nil || msg.Text == "" {
		return ""
	}

    user := msg.From.LastName + msg.From.FirstName
    text := msg.Text

    if user == "" {
        user = "anonym"
    }

    return user + ": " + text
}


func reverse(lines []string) string {
    for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
        lines[i], lines[j] = lines[j], lines[i]
    }
	dialog := strings.Join(lines, "\n")
    return dialog
}


func Memorize(msg *tg.Message, chatHistory map[string]string) (string, string) {
    lastLine := convertToLine(msg)
    prevLine := convertToLine(msg.ReplyToMessage)

	if prevLine != "" && lastLine != "" {
		chatHistory[lastLine] = prevLine
	}

	return prevLine, lastLine
}


func Remember(msg *tg.Message, chatHistory map[string]string, lim int) string {
    lines := make([]string, 0)

	prevLine, lastLine := Memorize(msg, chatHistory)
    lines = append(lines, lastLine)
    lines = append(lines, prevLine)

    lastLine = prevLine
	for i := 0; i < lim - 2; i++ {
        v, ok := chatHistory[lastLine]
        if ok && v != "" {
			log.Printf("%d messages remembered", i + 1)
            lines = append(lines, v)
            lastLine = v
        } else {
            break
        }
    }

	dialog := reverse(lines)

	fmt.Println(dialog)

    return dialog
}
