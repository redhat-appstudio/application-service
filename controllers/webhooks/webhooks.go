package webhooks

import (
	"github.com/redhat-appstudio/operator-toolkit/webhook"
)

// EnabledWebhooks is a slice containing references to all the webhooks that have to be registered
var EnabledWebhooks = []webhook.Webhook{
	&ApplicationWebhook{},
	&ComponentWebhook{},
}
