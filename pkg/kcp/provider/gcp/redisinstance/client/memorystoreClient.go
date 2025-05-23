package client

import (
	"context"
	"fmt"

	"github.com/kyma-project/cloud-manager/pkg/composed"

	redis "cloud.google.com/go/redis/apiv1"
	"cloud.google.com/go/redis/apiv1/redispb"
	cloudcontrolv1beta1 "github.com/kyma-project/cloud-manager/api/cloud-control/v1beta1"
	gcpclient "github.com/kyma-project/cloud-manager/pkg/kcp/provider/gcp/client"
	gcpmeta "github.com/kyma-project/cloud-manager/pkg/kcp/provider/gcp/meta"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type CreateRedisInstanceOptions struct {
	VPCNetworkFullName string
	IPRangeName        string
	MemorySizeGb       int32
	Tier               string
	RedisVersion       string
	AuthEnabled        bool
	RedisConfigs       map[string]string
	MaintenancePolicy  *cloudcontrolv1beta1.MaintenancePolicyGcp
	Labels             map[string]string
	ReplicaCount       int32
}

type MemorystoreClient interface {
	CreateRedisInstance(ctx context.Context, projectId, locationId, instanceId string, options CreateRedisInstanceOptions) error
	GetRedisInstance(ctx context.Context, projectId, locationId, instanceId string) (*redispb.Instance, *redispb.InstanceAuthString, error)
	UpdateRedisInstance(ctx context.Context, redisInstance *redispb.Instance, updateMask []string) error
	UpgradeRedisInstance(ctx context.Context, projectId, locationId, instanceId, redisVersion string) error
	DeleteRedisInstance(ctx context.Context, projectId, locationId, instanceId string) error
}

func NewMemorystoreClientProvider(gcpClients *gcpclient.GcpClients) gcpclient.GcpClientProvider[MemorystoreClient] {
	return func() MemorystoreClient {
		return NewMemorystoreClient(gcpClients)
	}
}

func NewMemorystoreClient(gcpClients *gcpclient.GcpClients) MemorystoreClient {
	return &memorystoreClient{
		redisInstanceClient: gcpClients.RedisInstance,
	}
}

type memorystoreClient struct {
	redisInstanceClient *redis.CloudRedisClient
}

// UpdateRedisInstanceConfigs implements MemorystoreClient.
func (memorystoreClient *memorystoreClient) UpdateRedisInstance(ctx context.Context, redisInstance *redispb.Instance, updateMask []string) error {
	req := &redispb.UpdateInstanceRequest{
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: updateMask,
		},
		Instance: redisInstance,
	}

	_, err := memorystoreClient.redisInstanceClient.UpdateInstance(ctx, req)
	if err != nil {
		logger := composed.LoggerFromCtx(ctx)
		logger.Error(err, "Failed to update redis instance", "redisInstance", redisInstance.Name)
		return err
	}

	return nil
}

func (memorystoreClient *memorystoreClient) CreateRedisInstance(ctx context.Context, projectId, locationId, instanceId string, options CreateRedisInstanceOptions) error {
	readReplicasMode := redispb.Instance_READ_REPLICAS_DISABLED
	if options.Tier != "BASIC" {
		readReplicasMode = redispb.Instance_READ_REPLICAS_ENABLED
	}

	parent := fmt.Sprintf("projects/%s/locations/%s", projectId, locationId)
	req := &redispb.CreateInstanceRequest{
		Parent:     parent,
		InstanceId: GetGcpMemoryStoreRedisInstanceId(instanceId),
		Instance: &redispb.Instance{
			Name:                  GetGcpMemoryStoreRedisName(projectId, locationId, instanceId),
			MemorySizeGb:          options.MemorySizeGb,
			Tier:                  redispb.Instance_Tier(redispb.Instance_Tier_value[options.Tier]),
			RedisVersion:          options.RedisVersion,
			ConnectMode:           redispb.Instance_PRIVATE_SERVICE_ACCESS, // always
			AuthorizedNetwork:     options.VPCNetworkFullName,
			ReservedIpRange:       options.IPRangeName,
			RedisConfigs:          options.RedisConfigs,
			AuthEnabled:           options.AuthEnabled,
			TransitEncryptionMode: redispb.Instance_SERVER_AUTHENTICATION,
			MaintenancePolicy:     ToMaintenancePolicy(options.MaintenancePolicy),
			Labels:                options.Labels,
			ReplicaCount:          options.ReplicaCount,
			ReadReplicasMode:      readReplicasMode,
		},
	}

	_, err := memorystoreClient.redisInstanceClient.CreateInstance(ctx, req)

	if err != nil {
		logger := composed.LoggerFromCtx(ctx)
		logger.Error(err, "CreateRedisInstance", "projectId", projectId, "locationId", locationId, "instanceId", instanceId)
		return err
	}

	return nil
}

func (memorystoreClient *memorystoreClient) GetRedisInstance(ctx context.Context, projectId, locationId, instanceId string) (*redispb.Instance, *redispb.InstanceAuthString, error) {
	logger := composed.LoggerFromCtx(ctx).WithValues("projectId", projectId, "locationId", locationId, "instanceId", instanceId)

	name := GetGcpMemoryStoreRedisName(projectId, locationId, instanceId)
	req := &redispb.GetInstanceRequest{
		Name: name,
	}

	instanceResponse, err := memorystoreClient.redisInstanceClient.GetInstance(ctx, req)
	if err != nil {
		if gcpmeta.IsNotFound(err) {
			logger.Info("target Redis instance not found")
			return nil, nil, err
		}
		logger.Error(err, "Failed to get Redis instance")
		return nil, nil, err
	}

	if !instanceResponse.AuthEnabled {
		return instanceResponse, nil, err
	}

	authResponse, err := memorystoreClient.redisInstanceClient.GetInstanceAuthString(ctx, &redispb.GetInstanceAuthStringRequest{Name: name})
	if err != nil {
		logger.Error(err, "Failed to get Redis instance Auth")
		return nil, nil, err
	}

	return instanceResponse, authResponse, nil
}

func (memorystoreClient *memorystoreClient) UpgradeRedisInstance(ctx context.Context, projectId, locationId, instanceId, redisVersion string) error {
	name := GetGcpMemoryStoreRedisName(projectId, locationId, instanceId)
	req := &redispb.UpgradeInstanceRequest{
		Name:         name,
		RedisVersion: redisVersion,
	}

	_, err := memorystoreClient.redisInstanceClient.UpgradeInstance(ctx, req)

	if err != nil {
		logger := composed.LoggerFromCtx(ctx)
		logger.Error(err, "UpgradeRedisInstance", "projectId", projectId, "locationId", locationId, "instanceId", instanceId)
		return err
	}

	return nil
}

func (memorystoreClient *memorystoreClient) DeleteRedisInstance(ctx context.Context, projectId string, locationId string, instanceId string) error {
	req := &redispb.DeleteInstanceRequest{
		Name: GetGcpMemoryStoreRedisName(projectId, locationId, instanceId),
	}

	_, err := memorystoreClient.redisInstanceClient.DeleteInstance(ctx, req)

	if err != nil {
		logger := composed.LoggerFromCtx(ctx)
		logger.Error(err, "DeleteRedisInstance", "projectId", projectId, "locationId", locationId, "instanceId", instanceId)
		return err
	}

	return nil
}
