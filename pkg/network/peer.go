package peer

import (
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
)

type Peer interface {
	Send(message *model.PeerMsg) error
	Sub(topic string, handler func(any) error) error

	AvailableNodes() []*crypto.PubKey
	AllNodes() []*crypto.PubKey
}
