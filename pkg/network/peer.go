package peer

import (
	"github.com/computeflux/tee-provider/pkg/model"
)

type Peer interface {
	Send(message *model.PeerMsg) error
	Sub(topic string, handler func(any) error) error

	AvailableNodes() []*model.PubKey
	AllNodes() []*model.PubKey
}
