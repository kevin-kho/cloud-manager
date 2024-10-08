package network

import (
	cloudcontrolv1beta1 "github.com/kyma-project/cloud-manager/api/cloud-control/v1beta1"
	"github.com/kyma-project/cloud-manager/pkg/common/actions/focal"
	"github.com/kyma-project/cloud-manager/pkg/kcp/network/types"
)

var _ types.State = &state{}

type state struct {
	focal.State
}

func (s *state) ObjAsNetwork() *cloudcontrolv1beta1.Network {
	return s.Obj().(*cloudcontrolv1beta1.Network)
}

func newState(focalState focal.State) types.State {
	return &state{State: focalState}
}
