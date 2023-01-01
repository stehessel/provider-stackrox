/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	v1 "github.com/stackrox/rox/generated/api/v1"
	"github.com/stackrox/rox/generated/storage"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/stehessel/provider-stackrox/apis/cluster/v1alpha1"
	apisv1alpha1 "github.com/stehessel/provider-stackrox/apis/v1alpha1"
	"github.com/stehessel/provider-stackrox/pkg/clients/central"
	"github.com/stehessel/provider-stackrox/pkg/features"
)

const (
	errNotCluster    = "managed resource is not a Cluster custom resource"
	errTrackPCUsage  = "cannot track ProviderConfig usage"
	errGetPC         = "cannot get ProviderConfig"
	errGetCreds      = "cannot get credentials"
	errGetFailed     = "cannot get cluster"
	errObserveFailed = "cannot observe cluster"
	errCreateFailed  = "cannot create cluster"
	errUpdateFailed  = "cannot update cluster"
	errDeleteFailed  = "cannot delete cluster"
)

// Setup adds a controller that reconciles Cluster managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ClusterGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ClusterGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Cluster{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube     client.Client
	usage    resource.Tracker
	external *external
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return nil, errors.New(errNotCluster)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	token, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}
	stringToken := string(token)

	client, err := central.NewGRPC(ctx, pc.Spec.Endpoint, stringToken)
	if err != nil {
		return nil, errors.Wrap(err, central.ErrNewClient)
	}
	c.external = &external{client: client}
	return c.external, nil
}

// Disconnect closes the connection of the external client.
func (c *connector) Disconnect(ctx context.Context) error {
	err := c.external.close()
	return errors.Wrap(err, central.ErrCloseClient)
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client *grpc.ClientConn
}

func (c *external) close() error {
	if c != nil && c.client != nil {
		err := c.client.Close()
		return errors.Wrap(err, central.ErrCloseClient)
	}
	return nil
}

func generateObservation(in *storage.Cluster) v1alpha1.ClusterObservation {
	s := in.GetMostRecentSensorId()
	mostRecentSensor := v1alpha1.SensorDeployment{
		AppNamespace:        s.GetAppNamespace(),
		AppNamespaceID:      s.GetAppNamespaceId(),
		AppServiceAccountID: s.GetAppServiceaccountId(),
		DefaultNamespaceID:  s.GetDefaultNamespaceId(),
		K8SNodeName:         s.GetK8SNodeName(),
		SystemNamespaceID:   s.GetSystemNamespaceId(),
	}
	return v1alpha1.ClusterObservation{
		AdmissionController:        in.GetAdmissionController(),
		AdmissionControllerEvents:  in.GetAdmissionControllerEvents(),
		AdmissionControllerUpdates: in.GetAdmissionControllerUpdates(),
		CentralAPIEndpoint:         in.GetCentralApiEndpoint(),
		CollectionMethod:           storage.CollectionMethod_name[int32(in.GetCollectionMethod())],
		CollectorImage:             in.GetCollectorImage(),
		ID:                         in.GetId(),
		Labels:                     in.GetLabels(),
		MainImage:                  in.GetMainImage(),
		ManagedBy:                  storage.ManagerType_name[int32(in.GetManagedBy())],
		MostRecentSensor:           mostRecentSensor,
		Name:                       in.GetName(),
		SlimCollector:              in.GetSlimCollector(),
		Tolerations:                !in.GetTolerationsConfig().GetDisabled(),
		Type:                       storage.ClusterType_name[int32(in.GetType())],
	}
}

func generateCluster(in *v1alpha1.ClusterParameters, base *storage.Cluster) *storage.Cluster {
	if base == nil {
		base = &storage.Cluster{}
	}
	base.AdmissionController = in.AdmissionController
	base.AdmissionControllerEvents = in.AdmissionControllerEvents
	base.AdmissionControllerUpdates = in.AdmissionControllerUpdates
	base.CentralApiEndpoint = in.CentralAPIEndpoint
	base.CollectionMethod = storage.CollectionMethod(storage.CollectionMethod_value[in.CollectionMethod])
	base.CollectorImage = in.CollectorImage
	base.Labels = in.Labels
	base.MainImage = in.MainImage
	base.Name = in.Name
	base.SlimCollector = in.SlimCollector
	base.TolerationsConfig = &storage.TolerationsConfig{Disabled: !in.Tolerations}
	base.Type = storage.ClusterType(storage.ClusterType_value[in.Type])
	return base
}

func isUpToDate(in *v1alpha1.Cluster, observed *storage.Cluster) (bool, string) {
	observedParams := v1alpha1.ClusterParameters{
		AdmissionController:        observed.GetAdmissionController(),
		AdmissionControllerEvents:  observed.GetAdmissionControllerEvents(),
		AdmissionControllerUpdates: observed.GetAdmissionControllerUpdates(),
		CentralAPIEndpoint:         observed.GetCentralApiEndpoint(),
		CollectionMethod:           storage.CollectionMethod_name[int32(observed.GetCollectionMethod())],
		CollectorImage:             observed.GetCollectorImage(),
		Labels:                     observed.GetLabels(),
		MainImage:                  observed.GetMainImage(),
		Name:                       observed.GetName(),
		SlimCollector:              observed.GetSlimCollector(),
		Tolerations:                !observed.GetTolerationsConfig().GetDisabled(),
		Type:                       storage.ClusterType_name[int32(observed.GetType())],
	}
	if diff := cmp.Diff(in.Spec.ForProvider, observedParams, cmpopts.EquateEmpty()); diff != "" {
		diff = "Observed difference in cluster\n" + diff
		return false, diff
	}
	return true, ""
}

func (c *external) getCluster(ctx context.Context, cr *v1alpha1.Cluster) (*storage.Cluster, error) {
	svc := v1.NewClustersServiceClient(c.client)
	resp, err := svc.GetClusters(ctx, &v1.GetClustersRequest{})
	if err != nil {
		return nil, errors.Wrap(err, errGetFailed)
	}
	for _, it := range resp.GetClusters() {
		if it.GetName() == meta.GetExternalName(cr) {
			return it, nil
		}
	}
	return nil, nil
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotCluster)
	}

	cluster, err := c.getCluster(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveFailed)
	}
	if cluster == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider = generateObservation(cluster)
	cr.SetConditions(xpv1.Available())
	upToDate, diff := isUpToDate(cr, cluster)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
		Diff:             diff,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotCluster)
	}
	cr.SetConditions(xpv1.Creating())

	svc := v1.NewClustersServiceClient(c.client)
	req := generateCluster(&cr.Spec.ForProvider, nil)
	resp, err := svc.PostCluster(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateFailed)
	}

	if c := resp.GetCluster(); c != nil {
		cr.Status.AtProvider = generateObservation(c)
		meta.SetExternalName(cr, c.GetName())
	}
	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotCluster)
	}

	cluster, err := c.getCluster(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateFailed)
	}
	if cluster == nil {
		return managed.ExternalUpdate{}, nil
	}

	svc := v1.NewClustersServiceClient(c.client)
	req := generateCluster(&cr.Spec.ForProvider, cluster)
	resp, err := svc.PutCluster(ctx, req)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errCreateFailed)
	}

	if c := resp.GetCluster(); c != nil {
		cr.Status.AtProvider = generateObservation(c)
		meta.SetExternalName(cr, c.GetName())
	}
	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return errors.New(errNotCluster)
	}
	mg.SetConditions(xpv1.Deleting())

	svc := v1.NewClustersServiceClient(c.client)
	_, err := svc.DeleteCluster(ctx, &v1.ResourceByID{Id: cr.Status.AtProvider.ID})
	return errors.Wrap(err, errDeleteFailed)
}
