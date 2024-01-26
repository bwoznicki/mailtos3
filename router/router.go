package router

import (
	"strings"

	"mailtos3/config"
)

// MatchMailbox returns the mailbox configuration from config
func MatchMailbox(mailboxes []config.Mailbox, emailAddress string) (*config.Mailbox, bool) {
	for _, mailbox := range mailboxes {
		if strings.EqualFold(mailbox.Address, emailAddress) {
			return &mailbox, true
		}
	}
	return &config.Mailbox{}, false
}
