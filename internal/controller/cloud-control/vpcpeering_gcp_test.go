package cloudcontrol

import (
	pb "cloud.google.com/go/compute/apiv1/computepb"
	cloudcontrolv1beta1 "github.com/kyma-project/cloud-manager/api/cloud-control/v1beta1"
	networkPkg "github.com/kyma-project/cloud-manager/pkg/kcp/network"
	scopePkg "github.com/kyma-project/cloud-manager/pkg/kcp/scope"
	. "github.com/kyma-project/cloud-manager/pkg/testinfra/dsl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

var _ = Describe("Feature: KCP VpcPeering", func() {
	It("Scenario: KCP GCP VpcPeering is created", func() {
		const (
			kymaName           = "7e829442-f92e-4205-9d36-0d622a422d74"
			kymaNetworkName    = kymaName + "--kyma"
			kymaProject        = "kyma-project"
			kymaVpc            = "shoot-12345-abc"
			remoteNetworkName  = "f5331c29-bb1a-439c-8376-94be50232eb4"
			remotePeeringName  = "peering-sap-gcp-skr-dev-cust-00002-to-sap-sc-learn"
			remoteVpc          = "default"
			remoteProject      = "sap-sc-learn"
			remoteRefNamespace = "kcp-system"
			remoteRefName      = "skr-gcp-vpcpeering"
			importCustomRoutes = false
		)

		scope := &cloudcontrolv1beta1.Scope{}

		By("Given Scope exists", func() {
			scopePkg.Ignore.AddName(kymaName)

			Eventually(CreateScopeGcp).
				WithArguments(infra.Ctx(), infra, scope, WithName(kymaName)).
				Should(Succeed())
		})

		// and Given the Kyma network object exists in KCP
		kymaNetwork := &cloudcontrolv1beta1.Network{
			Spec: cloudcontrolv1beta1.NetworkSpec{
				Network: cloudcontrolv1beta1.NetworkInfo{
					Reference: &cloudcontrolv1beta1.NetworkReference{
						Gcp: &cloudcontrolv1beta1.GcpNetworkReference{
							GcpProject:  kymaProject,
							NetworkName: kymaVpc,
						},
					},
				},
				Type: cloudcontrolv1beta1.NetworkTypeKyma,
			},
		}

		By("And Given Kyma Network exists in KCP", func() {
			// Tell Scope reconciler to ignore this kymaName
			networkPkg.Ignore.AddName(kymaNetworkName)

			Eventually(CreateObj).
				WithArguments(infra.Ctx(), infra.KCP().Client(), kymaNetwork, WithName(kymaNetworkName), WithScope(scope.Name)).
				Should(Succeed())
		})

		// and Given the remote network object exists in KCP
		remoteNetwork := &cloudcontrolv1beta1.Network{
			Spec: cloudcontrolv1beta1.NetworkSpec{
				Network: cloudcontrolv1beta1.NetworkInfo{
					Reference: &cloudcontrolv1beta1.NetworkReference{
						Gcp: &cloudcontrolv1beta1.GcpNetworkReference{
							GcpProject:  remoteProject,
							NetworkName: remoteVpc,
						},
					},
				},
				Type: cloudcontrolv1beta1.NetworkTypeExternal,
			},
		}

		By("And Given Remote Network exists in KCP", func() {
			// Tell Scope reconciler to ignore this kymaName
			networkPkg.Ignore.AddName(remoteNetworkName)

			Eventually(CreateObj).
				WithArguments(infra.Ctx(), infra.KCP().Client(), remoteNetwork, WithName(remoteNetworkName), WithScope(scope.Name), WithState("Ready")).
				Should(Succeed())
		})

		By("When KCP KymaNetwork is Ready", func() {
			Eventually(UpdateStatus).
				WithArguments(infra.Ctx(),
					infra.KCP().Client(),
					kymaNetwork,
					WithState("Ready"),
					WithConditions(KcpReadyCondition())).
				Should(Succeed())
		})

		By("And When KCP RemoteNetwork is Ready", func() {
			Eventually(UpdateStatus).
				WithArguments(infra.Ctx(),
					infra.KCP().Client(),
					remoteNetwork,
					WithState("Ready"),
					WithConditions(KcpReadyCondition())).
				Should(Succeed())
		})

		vpcpeering := &cloudcontrolv1beta1.VpcPeering{
			Spec: cloudcontrolv1beta1.VpcPeeringSpec{
				Details: &cloudcontrolv1beta1.VpcPeeringDetails{
					LocalNetwork: klog.ObjectRef{
						Name:      kymaNetwork.Name,
						Namespace: kymaNetwork.Namespace,
					},
					RemoteNetwork: klog.ObjectRef{
						Name:      remoteNetwork.Name,
						Namespace: remoteNetwork.Namespace,
					},
					PeeringName:        remotePeeringName,
					LocalPeeringName:   "cm-" + remoteNetworkName,
					ImportCustomRoutes: importCustomRoutes,
				},
			},
		}

		By("And When KCP VpcPeering is created and the remote network is tagged", func() {
			Eventually(CreateObj).
				WithArguments(infra.Ctx(), infra.KCP().Client(), vpcpeering,
					WithName(remoteNetworkName),
					WithRemoteRef(remoteRefName),
					WithScope(kymaName),
				).
				Should(Succeed())
		})

		var remotePeeringObject *pb.NetworkPeering
		By("Then GCP VpcPeering is created on remote side", func() {
			Eventually(LoadAndCheck).
				WithArguments(infra.Ctx(), infra.KCP().Client(), vpcpeering,
					NewObjActions(),
					HavingVpcPeeringStatusRemoteId(),
				).
				Should(Succeed())
			remotePeeringObject = infra.GcpMock().GetMockVpcPeering(remoteProject, remoteVpc)
		})

		var kymaPeeringObject *pb.NetworkPeering
		By("And Then GCP VpcPeering is created on kyma side", func() {
			Eventually(LoadAndCheck).
				WithArguments(infra.Ctx(), infra.KCP().Client(), vpcpeering,
					NewObjActions(),
					HavingVpcPeeringStatusId(),
				).
				Should(Succeed())
			kymaPeeringObject = infra.GcpMock().GetMockVpcPeering(kymaProject, kymaVpc)
		})

		By("When GCP VpcPeering is active on the remote side", func() {
			// GCP will set both to ACTIVE when the peering is active
			infra.GcpMock().SetMockVpcPeeringLifeCycleState(remoteProject, remoteVpc, pb.NetworkPeering_ACTIVE)
			Expect(ptr.Deref(remotePeeringObject.State, "") == pb.NetworkPeering_ACTIVE.String()).Should(BeTrue())
		})

		By("And When GCP VpcPeering is active on the kyma side", func() {
			// GCP will set both to ACTIVE when the peering is active
			infra.GcpMock().SetMockVpcPeeringLifeCycleState(kymaProject, kymaVpc, pb.NetworkPeering_ACTIVE)
			Expect(ptr.Deref(kymaPeeringObject.State, "") == pb.NetworkPeering_ACTIVE.String()).Should(BeTrue())
		})

		By("Then KCP VpcPeering has Ready condition", func() {
			Eventually(LoadAndCheck).
				WithArguments(infra.Ctx(), infra.KCP().Client(), vpcpeering,
					NewObjActions(),
					HavingConditionTrue(cloudcontrolv1beta1.ConditionTypeReady),
				).
				Should(Succeed())
		})

		// DELETE
		By("When KCP VpcPeering is deleted", func() {
			Eventually(Delete).
				WithArguments(infra.Ctx(), infra.KCP().Client(), vpcpeering).
				Should(Succeed(), "Error deleting VPC Peering")
		})

		By("Then VpcPeering does not exist", func() {
			Eventually(IsDeleted).
				WithArguments(infra.Ctx(), infra.KCP().Client(), vpcpeering).
				Should(Succeed(), "VPC Peering was not deleted")
		})

	})
})
