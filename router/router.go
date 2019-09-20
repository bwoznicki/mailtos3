package router

import (
	"../config"
	"strings"
)

func MatchMailbox(mailboxes []config.Mailbox, emailAddress string) (*config.Mailbox, bool) {
	for _, mailbox := range mailboxes {
		if strings.EqualFold(mailbox.Address, emailAddress) {
			return &mailbox, true
		}
	}
	return &config.Mailbox{}, false
}
