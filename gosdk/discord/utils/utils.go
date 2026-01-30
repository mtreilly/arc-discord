package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const discordEpoch = 1420070400000

// ParseMention extracts the ID from a Discord mention.
func ParseMention(mention string) (string, bool) {
	if len(mention) < 3 || mention[0] != '<' || mention[len(mention)-1] != '>' {
		return "", false
	}
	inner := mention[1 : len(mention)-1]
	switch {
	case strings.HasPrefix(inner, "@!"):
		return inner[2:], true
	case strings.HasPrefix(inner, "@&"):
		return inner[2:], true
	case strings.HasPrefix(inner, "@"):
		return inner[1:], true
	case strings.HasPrefix(inner, "#"):
		return inner[1:], true
	default:
		return "", false
	}
}

// FormatUserMention builds a user mention ID.
func FormatUserMention(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}

// FormatChannelMention builds a channel mention ID.
func FormatChannelMention(channelID string) string {
	return fmt.Sprintf("<#%s>", channelID)
}

// FormatRoleMention builds a role mention ID.
func FormatRoleMention(roleID string) string {
	return fmt.Sprintf("<@&%s>", roleID)
}

// ParseEmoji extracts name, ID, and animation flag from a custom emoji string.
func ParseEmoji(emoji string) (name, id string, animated bool, ok bool) {
	if !strings.HasPrefix(emoji, "<") || !strings.HasSuffix(emoji, ">") {
		return "", "", false, false
	}
	inner := emoji[1 : len(emoji)-1]
	parts := strings.Split(inner, ":")
	if len(parts) != 3 {
		return "", "", false, false
	}
	animated = parts[0] == "a"
	name = parts[1]
	id = parts[2]
	return name, id, animated, true
}

// FormatEmoji creates a Discord emoji payload string.
func FormatEmoji(name, id string, animated bool) string {
	prefix := ""
	if animated {
		prefix = "a:"
	}
	return fmt.Sprintf("<%s%s:%s>", prefix, name, id)
}

// SnowflakeToTime converts a snowflake string to a time.
func SnowflakeToTime(id string) (time.Time, error) {
	val, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	timestamp := (val >> 22) + discordEpoch
	return time.UnixMilli(timestamp), nil
}

// TimeToSnowflake converts a time to a snowflake string.
func TimeToSnowflake(t time.Time) string {
	ms := t.UnixMilli()
	if ms < discordEpoch {
		ms = discordEpoch
	}
	val := (ms - discordEpoch) << 22
	return strconv.FormatInt(val, 10)
}

// ChunkSlice splits a slice into chunks of the requested size.
func ChunkSlice[T any](slice []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	result := make([][]T, 0, (len(slice)+size-1)/size)
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		result = append(result, slice[i:end])
	}
	return result
}

// RateLimitDelay returns how long to wait before sending the next request.
func RateLimitDelay(remaining, limit int, reset time.Time) time.Duration {
	if remaining <= 0 || limit <= 0 || reset.IsZero() {
		if reset.After(time.Now()) {
			return time.Until(reset)
		}
		return time.Millisecond * 100
	}
	delay := time.Until(reset) / time.Duration(remaining)
	if delay < 0 {
		return time.Millisecond * 100
	}
	if delay == 0 {
		delay = time.Millisecond * 100
	}
	return delay
}
