package azurevpcpeering

import (
	"context"
	"github.com/kyma-project/cloud-manager/api"
	"github.com/kyma-project/cloud-manager/pkg/composed"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func addFinalizer(ctx context.Context, st composed.State) (error, context.Context) {
	state := st.(*State)
	logger := composed.LoggerFromCtx(ctx)

	if composed.MarkedForDeletionPredicate(ctx, state) {
		return nil, nil
	}

	added := controllerutil.AddFinalizer(state.Obj(), api.CommonFinalizerDeletionHook)
	if !added {
		// finalizer already added
		return nil, nil
	}

	err := state.UpdateObj(ctx)
	if err != nil {
		return composed.LogErrorAndReturn(err, "Error saving AzureVpcPeering after finalizer added", composed.StopWithRequeue, ctx)
	}

	logger.Info("Added finalizer to SKR AzureVpcPeering, requeue")

	return composed.StopWithRequeue, nil
}
