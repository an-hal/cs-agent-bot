package classifier

import (
	"strconv"
	"strings"
)

type Intent string

const (
	IntentAngry        Intent = "angry"
	IntentPaidClaim    Intent = "paid_claim"
	IntentNPS          Intent = "nps"
	IntentCSInterested Intent = "cs_interested"
	IntentReject       Intent = "reject"
	IntentDelay        Intent = "delay"
	IntentPositive     Intent = "positive"
	IntentOOO          Intent = "ooo"
	IntentWantsHuman   Intent = "wants_human"
)

type ReplyClassifier interface {
	ClassifyReply(messageType string, text string) Intent
}

type replyClassifier struct{}

func NewReplyClassifier() ReplyClassifier {
	return &replyClassifier{}
}

func (c *replyClassifier) ClassifyReply(messageType string, text string) Intent {
	// Voice notes, images, and videos are always wants_human
	switch messageType {
	case "voice", "audio", "image", "video", "document":
		return IntentWantsHuman
	}

	lower := strings.ToLower(text)

	// Priority order matching (first match wins)

	// 1. Angry
	angryKeywords := []string{"ancam", "lawyer", "hukum", "lapor", "bohong", "tipu"}
	if containsAny(lower, angryKeywords) {
		return IntentAngry
	}

	// 2. Paid claim
	paidKeywords := []string{"sudah bayar", "lunas", "transfer", "trf", "sdh bayar", "sudah transfer"}
	if containsAny(lower, paidKeywords) {
		return IntentPaidClaim
	}

	// 3. NPS (numeric reply)
	trimmed := strings.TrimSpace(text)
	if score, err := strconv.Atoi(trimmed); err == nil && score >= 1 && score <= 10 {
		return IntentNPS
	}

	// 4. CS interested
	csKeywords := []string{"demo", "info", "mau", "jadwal", "tertarik", "share"}
	if containsAny(lower, csKeywords) {
		return IntentCSInterested
	}

	// 5. Reject
	rejectKeywords := []string{"tidak", "cancel", "stop", "ga perlu", "nggak"}
	if containsAny(lower, rejectKeywords) {
		return IntentReject
	}

	// 6. Delay
	delayKeywords := []string{"nanti", "bulan depan", "next", "minggu depan"}
	if containsAny(lower, delayKeywords) {
		return IntentDelay
	}

	// 7. Positive
	positiveKeywords := []string{"oke", "bisa", "setuju", "boleh"}
	if containsAny(lower, positiveKeywords) {
		return IntentPositive
	}

	// 8. OOO
	oooKeywords := []string{"out of office", "auto reply", "sedang tidak"}
	if containsAny(lower, oooKeywords) {
		return IntentOOO
	}

	// 9. Wants human (ends with ?)
	if strings.HasSuffix(strings.TrimSpace(text), "?") {
		return IntentWantsHuman
	}

	// Default to positive if no match
	return IntentPositive
}

func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
