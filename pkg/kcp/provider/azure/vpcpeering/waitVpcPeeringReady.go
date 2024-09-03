package vpcpeering

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5"
	"github.com/kyma-project/cloud-manager/pkg/composed"
	"github.com/kyma-project/cloud-manager/pkg/util"
	"k8s.io/utils/ptr"
)

func waitVpcPeeringReady(ctx context.Context, st composed.State) (error, context.Context) {
	state := st.(*State)
	logger := composed.LoggerFromCtx(ctx)

	if ptr.Deref(state.peering.Properties.PeeringState, "") != armnetwork.VirtualNetworkPeeringStateConnected {
		logger.Info("Waiting for peering Connected state", "Id", *state.peering.ID, "PeeringState", *state.peering.Properties.PeeringState)
		return composed.StopWithRequeueDelay(util.Timing.T60000ms()), nil
	}

	return nil, nil
}