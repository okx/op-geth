package apollo

import (
	"fmt"

	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/ethereum/go-ethereum/log"
)

type CustomChangeListener struct{}

func (c *CustomChangeListener) OnChange(changeEvent *storage.ChangeEvent) {
	for _, value := range changeEvent.Changes {
		if value.ChangeType == storage.MODIFIED {
			log.Info(fmt.Sprintf("apollo old config : %+v", value.OldValue.(string)))
			log.Info(fmt.Sprintf("apollo config changed: %+v", value.NewValue.(string)))
		}
	}
}

func (c *CustomChangeListener) OnNewestChange(event *storage.FullChangeEvent) {
	//write your code here
}
